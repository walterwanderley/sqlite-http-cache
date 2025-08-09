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
	querier requestQuerier
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

	if !readOnly && uint(time.Since(resp.Timestamp).Seconds()) > ttl {
		// data is too old, tell the responseHandler to save the new data
		ctx.UserData = fmt.Sprintf("%s:%d", resp.TableName, resp.DatabaseID)
		return r, nil
	}
	if verbose {
		slog.Info("serving from database", "url", url, "status", resp.Status, "timestamp", resp.Timestamp.Format(time.RFC3339))
	}

	return r, &http.Response{
		StatusCode: resp.Status,
		Body:       resp.Body,
		Header:     http.Header(resp.Headers),
	}
}
