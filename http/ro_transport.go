package http

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"

	"github.com/walterwanderley/sqlite-http-cache/db"
)

type readOnlyTransportQuerier interface {
	io.Closer
	FindByURL(ctx context.Context, url string) (*db.Response, error)
}

type readOnlyTransport struct {
	base    http.RoundTripper
	querier readOnlyTransportQuerier
}

func newReadOnlyTransport(base http.RoundTripper, sqlDB *sql.DB, tableNames ...string) (*readOnlyTransport, error) {
	if base == nil {
		base = http.DefaultTransport.(*http.Transport).Clone()
	}
	querier, err := db.NewRepository(sqlDB, tableNames...)
	if err != nil {
		return nil, fmt.Errorf("creating repository: %w", err)
	}
	return &readOnlyTransport{
		base:    base,
		querier: querier,
	}, nil
}

func (t *readOnlyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet {
		return t.base.RoundTrip(req)
	}

	url := req.URL.String()
	resp, err := t.querier.FindByURL(req.Context(), url)
	if err != nil {
		return t.base.RoundTrip(req)
	}

	return &http.Response{
		StatusCode: resp.Status,
		Body:       resp.Body,
		Header:     http.Header(resp.Headers),
	}, nil

}

func (t *readOnlyTransport) Close() error {
	return t.querier.Close()
}
