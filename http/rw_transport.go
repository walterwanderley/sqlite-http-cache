package http

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/walterwanderley/sqlite-http-cache/db"
)

type readWriteTransportQuerier interface {
	readOnlyTransportQuerier
	Write(ctx context.Context, url string, resp *db.Response) error
}

type readWriteTransport struct {
	base    http.RoundTripper
	querier readWriteTransportQuerier
	ttl     time.Duration
}

func newReadWriteTransport(base http.RoundTripper, sqlDB *sql.DB, ttl time.Duration, tableNames ...string) (*readWriteTransport, error) {
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
		ttl:     ttl,
	}, nil
}

func (t *readWriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet {
		return t.base.RoundTrip(req)
	}

	url := req.URL.String()
	respDB, err := t.querier.FindByURL(req.Context(), url)
	if err != nil || (t.ttl > 0 && time.Since(respDB.Timestamp) > t.ttl) {
		resp, err := t.base.RoundTrip(req)
		if err == nil {
			newRespDB, err := db.HttpToResponse(resp)
			if err == nil {
				newRespDB.TableName = respDB.TableName
				t.querier.Write(context.Background(), url, newRespDB)
			}
		}
		return resp, err
	}

	return &http.Response{
		StatusCode: respDB.Status,
		Body:       respDB.Body,
		Header:     http.Header(respDB.Headers),
	}, nil

}

func (t *readWriteTransport) Close() error {
	return t.querier.Close()
}
