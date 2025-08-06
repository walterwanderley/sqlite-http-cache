package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.riyazali.net/sqlite"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

const (
	timeoutOption            = "timeout"              // timeout in miliseconds
	insecureOption           = "insecure"             // insecure skip TLS validation
	ignoreStatusErrorOption  = "ignore_status_error"  // do not persist responses if status code != 2xx
	responseTableNameOption  = "response_table"       // table name of the http responses
	oauth2ClientIDOption     = "oauth2_client_id"     // oauth2 client credentials flow: cient_id
	oauth2ClientSecretOption = "oauth2_client_secret" // oauth2 client credentials flow: cient_secret
	oauth2TokenURLOption     = "oauth2_token_url"     // oauth2 client credentials flow: token URL

	defaultResponseTableName = "http_response"
	defaultVirtualTableName  = "http_request"
)

var tableNameValid = regexp.MustCompilePOSIX("^[a-zA-Z_][a-zA-Z0-9_.]*$").MatchString

type CacheModule struct {
}

func (m *CacheModule) Connect(conn *sqlite.Conn, args []string, declare func(string) error) (sqlite.VirtualTable, error) {
	tableName := args[2]
	if tableName == "" {
		tableName = defaultVirtualTableName
	}
	var (
		responseTableName string
		timeout           time.Duration
		insecure          bool
		ignoreStatusError bool
		headers           = make(map[string]string)
		credentials       clientcredentials.Config
		err               error
	)
	if len(args) > 3 {
		for _, option := range args[3:] {
			k, v, ok := strings.Cut(option, "=")
			if !ok {
				return nil, fmt.Errorf("invalid option: %q", option)
			}
			k = strings.TrimSpace(k)
			v = strings.TrimSpace(v)
			switch strings.ToLower(k) {
			case timeoutOption:
				i, err := strconv.Atoi(v)
				if err != nil {
					return nil, fmt.Errorf("invalid %q option: %v", k, err)
				}
				timeout = time.Duration(i) * time.Millisecond
			case insecureOption:
				insecure, err = strconv.ParseBool(v)
				if err != nil {
					return nil, fmt.Errorf("invalid %q option: %v", k, err)
				}
			case ignoreStatusErrorOption:
				ignoreStatusError, err = strconv.ParseBool(v)
				if err != nil {
					return nil, fmt.Errorf("invalid %q option: %v", k, err)
				}
			case responseTableNameOption:
				if tableNameValid(v) {
					responseTableName = v
				} else {
					return nil, fmt.Errorf("invalid %q option", k)
				}
			case oauth2ClientIDOption:
				credentials.ClientID = v
			case oauth2ClientSecretOption:
				credentials.ClientSecret = v
			case oauth2TokenURLOption:
				credentials.TokenURL = v
			default:
				headers[k] = v
			}
		}
	}
	if responseTableName == "" {
		responseTableName = defaultResponseTableName
	}

	err = conn.Exec(fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s(
		url TEXT PRIMARY KEY,
		status INTEGER,
		body BLOB,
		headers JSONB,
		timestamp INTEGER
		)`, responseTableName), nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Timeout:   timeout,
		Transport: NewTransport(insecure, headers),
	}

	if credentials.TokenURL != "" && credentials.ClientID != "" {
		ctx := context.WithValue(context.Background(), oauth2.HTTPClient, client)
		client = credentials.Client(ctx)
	}
	vt, err := NewRequestVirtualTable(tableName, client, ignoreStatusError, responseTableName, conn)
	if err != nil {
		return nil, err
	}
	return vt, declare("CREATE TABLE x(url TEXT PRIMARY KEY)")
}

func NewRequestVirtualTable(virtualTableName string, client *http.Client, ignoreStatusError bool, responseTableName string, conn *sqlite.Conn) (*RequestVirtualTable, error) {
	stmt, _, err := conn.Prepare(fmt.Sprintf(`
		INSERT INTO %s(url, status, body, headers, timestamp) 
		VALUES(?, ?, ?, ?, unixepoch())
		ON CONFLICT(url) DO UPDATE SET 
		status = ?,
		body = ?,
		headers = ?,
		timestamp = unixepoch()`, responseTableName))
	if err != nil {
		return nil, err
	}

	return &RequestVirtualTable{
		client:            client,
		ignoreStatusError: ignoreStatusError,
		tableName:         virtualTableName,
		conn:              conn,
		stmt:              stmt}, nil
}

type RequestVirtualTable struct {
	client            *http.Client
	ignoreStatusError bool
	tableName         string
	conn              *sqlite.Conn
	stmt              *sqlite.Stmt
	mu                sync.Mutex
}

func (vt *RequestVirtualTable) BestIndex(_ *sqlite.IndexInfoInput) (*sqlite.IndexInfoOutput, error) {
	return &sqlite.IndexInfoOutput{}, nil
}

func (vt *RequestVirtualTable) Open() (sqlite.VirtualCursor, error) {
	return nil, fmt.Errorf("SELECT operations on %q is not supported", vt.tableName)
}

func (vt *RequestVirtualTable) Disconnect() error {
	return vt.stmt.Finalize()
}

func (vt *RequestVirtualTable) Destroy() error {
	return nil
}

func (vt *RequestVirtualTable) Insert(values ...sqlite.Value) (int64, error) {
	url := values[0].Text()
	resp, err := vt.client.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if vt.ignoreStatusError && resp.StatusCode/100 != 2 {
		return 0, nil
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var headersBuf bytes.Buffer
	json.NewEncoder(&headersBuf).Encode(resp.Header)

	body := string(bodyBytes)
	status := int64(resp.StatusCode)
	headers := headersBuf.String()

	vt.mu.Lock()
	err = vt.stmt.Reset()
	if err != nil {
		return 0, err
	}
	vt.stmt.BindText(1, url)
	vt.stmt.BindInt64(2, status)
	vt.stmt.BindText(3, body)
	vt.stmt.BindText(4, headers)
	//ON CONFLICT
	vt.stmt.BindInt64(5, status)
	vt.stmt.BindText(6, body)
	vt.stmt.BindText(7, headers)
	_, err = vt.stmt.Step()
	vt.mu.Unlock()
	if err != nil {
		return 0, err
	}
	return 0, nil
}

func (vt *RequestVirtualTable) Update(_ sqlite.Value, _ ...sqlite.Value) error {
	return fmt.Errorf("UPDATE operations on %q is not supported", vt.tableName)
}

func (vt *RequestVirtualTable) Replace(old sqlite.Value, new sqlite.Value, _ ...sqlite.Value) error {
	return fmt.Errorf("UPDATE operations on %q is not supported", vt.tableName)
}

func (vt *RequestVirtualTable) Delete(_ sqlite.Value) error {
	return fmt.Errorf("DELETE operations on %q is not supported", vt.tableName)
}

func init() {
	sqlite.Register(func(api *sqlite.ExtensionApi) (sqlite.ErrorCode, error) {
		if err := api.CreateModule(defaultVirtualTableName, &CacheModule{}, sqlite.ReadOnly(false)); err != nil {
			return sqlite.SQLITE_ERROR, err
		}
		return sqlite.SQLITE_OK, nil
	})
}

func main() {}
