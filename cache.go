package main

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
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

	if strings.EqualFold(tableName, responseTableName) {
		return nil, fmt.Errorf("use different names on virtual table and response table")
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
	vtab, err := NewRequestVirtualTable(tableName, client, ignoreStatusError, responseTableName, conn)
	if err != nil {
		return nil, err
	}
	return vtab, declare("CREATE TABLE x(url TEXT PRIMARY KEY)")
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
