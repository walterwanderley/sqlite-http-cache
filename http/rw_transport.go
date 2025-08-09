package http

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"github.com/walterwanderley/sqlite-http-cache/db"
)

type readWriteTransportQuerier interface {
	readOnlyTransportQuerier
	Write(ctx context.Context, url string, resp *db.Response) error
}

type readWriteTransport struct {
	base    http.RoundTripper
	querier readWriteTransportQuerier
}

func newReadWriteTransport(base http.RoundTripper, sqlDB *sql.DB, tableNames ...string) (*readWriteTransport, error) {
	if base == nil {
		base = http.DefaultTransport.(*http.Transport).Clone()
	}
	querier, err := db.NewRepository(sqlDB, tableNames...)
	if err != nil {
		return nil, fmt.Errorf("creating repository: %w", err)
	}
	return &readWriteTransport{
		base:    base,
		querier: querier,
	}, nil
}

func (t *readWriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet {
		return t.base.RoundTrip(req)
	}

	url := req.URL.String()
	resp, err := t.querier.FindByURL(req.Context(), url)
	if err != nil {
		resp, err := t.base.RoundTrip(req)
		if err == nil {

		}
		return resp, err
	}

	return &http.Response{
		StatusCode: resp.Status,
		Body:       resp.Body,
		Header:     http.Header(resp.Headers),
	}, nil

}

func (t *readWriteTransport) Close() error {
	return t.querier.Close()
}
