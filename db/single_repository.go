package db

import (
	"context"
	"database/sql"
	"fmt"
)

type singleRepository struct {
	stmt *sql.Stmt
}

func newSingleRepository(db *sql.DB, tableName string) (*singleRepository, error) {
	stmt, err := db.Prepare(fmt.Sprintf("SELECT status, body, headers, timestamp FROM %s WHERE url = ?", tableName))
	if err != nil {
		return nil, fmt.Errorf("prepare query for %q: %w", tableName, err)
	}
	return &singleRepository{
		stmt: stmt,
	}, nil
}

func (r *singleRepository) FindByURL(ctx context.Context, url string) (*Response, error) {
	response, err := rowToResponse(r.stmt.QueryRowContext(ctx, url))
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (r *singleRepository) Close() error {
	return nil
}
