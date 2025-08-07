package http

import (
	"context"
	"database/sql"
	"io"
	"net/http"

	"github.com/walterwanderley/sqlite-http-cache/http/internal"
)

var HTTPClient internal.ContextKey

type Config struct {
	DB     *sql.DB
	Tables []string
}

func (c Config) Client(ctx context.Context) (*http.Client, io.Closer, error) {
	cc := internal.ContextClient(ctx)
	t, err := NewTransport(cc.Transport, c.DB, c.Tables...)
	if err != nil {
		return nil, nil, err
	}
	return &http.Client{
		Transport:     t,
		CheckRedirect: cc.CheckRedirect,
		Jar:           cc.Jar,
		Timeout:       cc.Timeout,
	}, t, nil
}
