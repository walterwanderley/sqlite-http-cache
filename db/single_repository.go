package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
)

type singleRepository struct {
	readStmt  *sql.Stmt
	writeStmt *sql.Stmt
	mu        sync.Mutex
}

func newSingleRepository(db *sql.DB, tableName string) (*singleRepository, error) {
	readStmt, err := db.Prepare(readerQuery(tableName))
	if err != nil {
		return nil, fmt.Errorf("prepare reader query for %q: %w", tableName, err)
	}
	writeStmt, err := db.Prepare(WriterQuery(tableName))
	if err != nil {
		return nil, fmt.Errorf("prepare writer query for %q: %w", tableName, err)
	}
	return &singleRepository{
		readStmt:  readStmt,
		writeStmt: writeStmt,
	}, nil
}

func (r *singleRepository) FindByURL(ctx context.Context, url string) (*Response, error) {
	response, err := rowToResponse(r.readStmt.QueryRowContext(ctx, url))
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (r *singleRepository) Write(ctx context.Context, url string, resp *Response) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return execWriter(ctx, r.writeStmt, url, resp)
}

func (r *singleRepository) Close() error {
	return errors.Join(r.readStmt.Close(), r.writeStmt.Close())
}
