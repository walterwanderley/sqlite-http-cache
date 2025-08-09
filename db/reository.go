package db

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type Repository interface {
	io.Closer
	FindByURL(ctx context.Context, url string) (*Response, error)
	Write(ctx context.Context, url string, resp *Response) error
}

type Response struct {
	Status     int
	Body       io.ReadCloser
	Headers    map[string][]string
	Timestamp  time.Time
	DatabaseID int
	TableName  string
}

func NewRepository(db *sql.DB, tableNames ...string) (Repository, error) {
	if db == nil {
		return nil, fmt.Errorf("db is nil")
	}
	if len(tableNames) == 0 {
		var err error
		tableNames, err = ResponseTables(db)
		if err != nil {
			return nil, fmt.Errorf("discovery response tables: %w", err)
		}
	}

	if len(tableNames) == 1 {
		return newSingleRepository(db, tableNames[0])
	}

	return newConcurrentRepository(db, 0, tableNames...)
}

func CreateResponseTables(db *sql.DB, tableNames ...string) error {
	var tableNameValid = regexp.MustCompilePOSIX("^[a-zA-Z_][a-zA-Z0-9_.]*$").MatchString

	for _, tableName := range tableNames {
		if !tableNameValid(tableName) {
			return fmt.Errorf("tabe name %q is invalid", tableName)
		}
		query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s(
		url TEXT PRIMARY KEY,
		status INTEGER,
		body BLOB,
		headers JSONB,
		timestamp DATETIME
		)`, tableName)
		_, err := db.Exec(query)
		if err != nil {
			return fmt.Errorf("creating table %q: %w", tableName, err)
		}
	}
	return nil
}

func HttpToResponse(resp *http.Response) (*Response, error) {
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}
	resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	return &Response{
		Status:  resp.StatusCode,
		Headers: resp.Header,
		Body:    io.NopCloser(bytes.NewReader(bodyBytes)),
	}, nil
}

func getReaderQuery(tableName string) string {
	return fmt.Sprintf("SELECT status, body, headers, timestamp FROM %s WHERE url = ?", tableName)
}

func rowToResponse(row *sql.Row) (*Response, error) {
	if err := row.Err(); err != nil {
		return nil, fmt.Errorf("response row: %w", err)
	}
	var (
		status    int
		body      string
		headers   string
		timestamp time.Time
	)
	err := row.Scan(&status, &body, &headers, &timestamp)
	if err != nil {
		return nil, fmt.Errorf("scan response row: %w", err)
	}
	var headersMap map[string][]string
	json.Unmarshal([]byte(headers), &headersMap)

	return &Response{
		Status:    status,
		Body:      io.NopCloser(strings.NewReader(body)),
		Headers:   headersMap,
		Timestamp: timestamp,
	}, nil
}

func getWriterQuery(tableName string) string {
	return fmt.Sprintf(`INSERT INTO %s(url, status, body, headers, timestamp) 
	VALUES(?, ?, ?, ?, DATETIME('now'))
	ON CONFLICT(url) DO UPDATE SET 
	status = ?,
	body = ?,
	headers = ?,
	timestamp = DATETIME('now')`, tableName)
}

func execWriter(ctx context.Context, stmt *sql.Stmt, url string, resp *Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading body: %w", err)
	}
	bodyStr := string(body)

	var headersBuf bytes.Buffer
	json.NewEncoder(&headersBuf).Encode(resp.Headers)
	headers := headersBuf.String()

	_, err = stmt.ExecContext(ctx,
		url, resp.Status, bodyStr, headers,
		// On Conflict
		resp.Status, bodyStr, headers)

	if err != nil {
		return fmt.Errorf("store response: %w", err)
	}
	return nil
}
