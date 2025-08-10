package main

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

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
	if ctx.UserData != nil {
		if h.verbose {
			slog.Info("recording response", "url", ctx.Req.URL.String(), "status", resp.StatusCode)
		}
		responseDB, err := db.HttpToResponse(resp)
		if err != nil {
			slog.Error("adapter response body", "error", err)
		} else {
			go func() {
				userData := ctx.UserData.(string)
				tableName, databaseID, ok := strings.Cut(userData, ":")
				if ok {
					responseDB.DatabaseID, _ = strconv.Atoi(databaseID)
				}
				responseDB.TableName = tableName
				err := h.writer.Write(context.Background(), ctx.Req.URL.String(), responseDB)
				if err != nil {
					slog.Error("recording response", "error", err, "url", ctx.Req.URL.String(), "status", resp.StatusCode)
				}
			}()
		}
	}
	return resp
}
