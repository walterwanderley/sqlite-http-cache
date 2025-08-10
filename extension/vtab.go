package extension

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/walterwanderley/sqlite"
)

type RequestVirtualTable struct {
	client            *http.Client
	ignoreStatusError bool
	tableName         string
	responseTableName string
	conn              *sqlite.Conn
	stmt              *sqlite.Stmt
	mu                sync.Mutex
}

func NewRequestVirtualTable(virtualTableName string, client *http.Client, ignoreStatusError bool, responseTableName string, conn *sqlite.Conn) (*RequestVirtualTable, error) {
	stmt, _, err := conn.Prepare(fmt.Sprintf(`INSERT INTO %s(url, status, body, header, timestamp) 
		VALUES(?, ?, ?, ?, DATETIME('now'))
		ON CONFLICT(url) DO UPDATE SET 
		status = ?,
		body = ?,
		header = ?,
		timestamp = DATETIME('now')`, responseTableName))
	if err != nil {
		return nil, err
	}

	return &RequestVirtualTable{
		client:            client,
		ignoreStatusError: ignoreStatusError,
		tableName:         virtualTableName,
		responseTableName: responseTableName,
		conn:              conn,
		stmt:              stmt,
	}, nil
}

func (vt *RequestVirtualTable) BestIndex(in *sqlite.IndexInfoInput) (*sqlite.IndexInfoOutput, error) {
	return &sqlite.IndexInfoOutput{}, nil
}

func (vt *RequestVirtualTable) Open() (sqlite.VirtualCursor, error) {
	return nil, fmt.Errorf("SELECT operations on %q is not supported, exec SELECT on %q table", vt.tableName, vt.responseTableName)
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

	body := string(bodyBytes)
	status := int64(resp.StatusCode)

	var headerBuf bytes.Buffer
	json.NewEncoder(&headerBuf).Encode(resp.Header)
	header := headerBuf.String()

	vt.mu.Lock()
	err = vt.stmt.Reset()
	if err != nil {
		return 0, err
	}
	vt.stmt.BindText(1, url)
	vt.stmt.BindInt64(2, status)
	vt.stmt.BindText(3, body)
	vt.stmt.BindText(4, header)
	//ON CONFLICT
	vt.stmt.BindInt64(5, status)
	vt.stmt.BindText(6, body)
	vt.stmt.BindText(7, header)
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
