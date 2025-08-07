package db

import (
	"context"
	"database/sql"
	"fmt"
)

type concurrentRepository struct {
	workers    []*worker
	globalQuit chan struct{}
}

func newConcurrentRepository(db *sql.DB, tableNames ...string) (Repository, error) {
	globalQuit := make(chan struct{})

	workers := make([]*worker, len(tableNames))
	for i, tableName := range tableNames {
		stmt, err := db.Prepare(fmt.Sprintf("SELECT status, body, headers, timestamp FROM %s WHERE url = ?", tableName))
		if err != nil {
			return nil, fmt.Errorf("prepare query for %q: %w", tableName, err)
		}
		w := newWorker(stmt)
		workers[i] = w
		w.start()
	}

	return &concurrentRepository{
		workers:    workers,
		globalQuit: globalQuit,
	}, nil
}

func (r *concurrentRepository) FindByURL(ctx context.Context, url string) (*Response, error) {
	size := len(r.workers)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	respCh := make(chan *Response, size)
	unitWork := work{
		ctx:  ctx,
		url:  url,
		resp: respCh,
	}

	go func() {
		for _, w := range r.workers {
			w.source <- unitWork
		}
	}()
	var count int
	for {
		select {
		case resp := <-respCh:
			count++
			if resp != nil {
				return resp, nil
			}
			if count == size {
				return nil, sql.ErrNoRows
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (r *concurrentRepository) Close() error {
	close(r.globalQuit)
	return nil
}

type work struct {
	ctx  context.Context
	url  string
	resp chan *Response
}

type worker struct {
	source chan work
	quit   chan struct{}
	stmt   *sql.Stmt
}

func newWorker(stmt *sql.Stmt) *worker {
	return &worker{
		stmt: stmt,
	}
}

func (w *worker) start() {
	w.source = make(chan work, 10)
	go func() {
		for {
			select {
			case unitWork := <-w.source:
				response, err := rowToResponse(w.stmt.QueryRowContext(unitWork.ctx, unitWork.url))
				if err != nil {
					unitWork.resp <- nil
					continue
				}
				unitWork.resp <- response

			case <-w.quit:
				w.stmt.Close()
				return
			}
		}
	}()
}
