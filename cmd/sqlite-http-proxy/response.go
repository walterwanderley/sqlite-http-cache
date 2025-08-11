package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/elazarl/goproxy"
	"github.com/walterwanderley/sqlite-http-cache/db"
)

type responseWriter interface {
	Write(ctx context.Context, url string, resp *db.Response) error
}

type responseHandler struct {
	writer  responseWriter
	verbose bool
}

func (h *responseHandler) Handle(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	ud, ok := ctx.UserData.(userData)
	if ok {
		if h.verbose {
			slog.Info("recording response", "url", ctx.Req.URL.String(), "status", resp.StatusCode)
		}
		responseDB, err := db.HttpToResponse(resp)
		if err != nil {
			slog.Error("adapter response body", "error", err)
		} else {
			go func() {
				responseDB.RequestTime = ud.requestTime
				responseDB.ResponseTime = time.Now()
				responseDB.DatabaseID = ud.databaseID
				responseDB.TableName = ud.tableName
				err := h.writer.Write(context.Background(), ctx.Req.URL.String(), responseDB)
				if err != nil {
					slog.Error("recording response", "error", err, "url", ctx.Req.URL.String(), "status", resp.StatusCode)
				}
			}()
		}
	}
	return resp
}
