package http

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/walterwanderley/sqlite-http-cache/db"
)

type Transport struct {
	base       http.RoundTripper
	repository db.Repository
}

func NewTransport(base http.RoundTripper, sqlDB *sql.DB, tableNames ...string) (*Transport, error) {
	if base == nil {
		base = http.DefaultTransport.(*http.Transport).Clone()
	}
	repository, err := db.NewRepository(sqlDB, tableNames...)
	if err != nil {
		return nil, fmt.Errorf("creating repository: %w", err)
	}
	return &Transport{
		base:       base,
		repository: repository,
	}, nil
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet {
		return t.base.RoundTrip(req)
	}

	url := req.URL.String()
	resp, err := t.repository.FindByURL(req.Context(), url)
	if err != nil {
		return t.base.RoundTrip(req)
	}

	return &http.Response{
		StatusCode: resp.Status,
		Body:       resp.Body,
		Header:     http.Header(resp.Headers),
	}, nil

}

func (t *Transport) Close() error {
	return t.repository.Close()
}
