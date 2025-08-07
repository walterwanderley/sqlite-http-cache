package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

type Repository interface {
	io.Closer
	FindByURL(ctx context.Context, url string) (*Response, error)
}

type Response struct {
	Status    int
	Body      io.ReadCloser
	Headers   map[string][]string
	Timestamp time.Time
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

	return newConcurrentRepository(db, tableNames...)
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
