package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/elazarl/goproxy"

	"github.com/walterwanderley/sqlite-http-cache/db"
)

type requestQuerier interface {
	FindByURL(ctx context.Context, url string) (*db.Response, error)
}

type requestHandler struct {
	verbose  bool
	ttl      uint
	readOnly bool
	querier  requestQuerier
}

func (h *requestHandler) Handle(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	if r.Method != http.MethodGet {
		return r, nil
	}

	url := ctx.Req.URL.String()
	resp, err := h.querier.FindByURL(r.Context(), url)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Error("database query", "error", err.Error())
		}
		// tell the responseHandler to save the new response data
		ctx.UserData = ":-1"
		return r, nil
	}

	if !h.readOnly && h.ttl > 0 && uint(time.Since(resp.Timestamp).Seconds()) > h.ttl {
		// data is too old, tell the responseHandler to save the new data
		ctx.UserData = fmt.Sprintf("%s:%d", resp.TableName, resp.DatabaseID)
		return r, nil
	}
	if h.verbose {
		slog.Info("serving from database", "url", url, "status", resp.Status, "timestamp", resp.Timestamp.Format(time.RFC3339))
	}

	header := http.Header(resp.Header)
	if date := header.Get("Date"); date != "" {
		d, err := time.Parse(time.RFC1123, date)
		if err == nil {
			header.Set("Age", fmt.Sprint(int(time.Since(d).Seconds())))
		}
	}

	return r, &http.Response{
		StatusCode: resp.Status,
		Body:       resp.Body,
		Header:     header,
	}
}
