package db

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
)

type concurrentRepository struct {
	db         *sql.DB
	queriers   []*querier
	globalQuit chan struct{}

	writers []*sql.Stmt
	// roundRobin strategy to choose one writer
	currentWriter int
	muWriter      sync.Mutex
}

func newConcurrentRepository(db *sql.DB, tableNames ...string) (*concurrentRepository, error) {
	globalQuit := make(chan struct{})

	size := len(tableNames)
	queriers := make([]*querier, size)
	writers := make([]*sql.Stmt, size)
	for i, tableName := range tableNames {
		readStmt, err := db.Prepare(getReaderQuery(tableName))
		if err != nil {
			return nil, fmt.Errorf("prepare read query for %q: %w", tableName, err)
		}
		q := newQuerier(readStmt, tableName)
		queriers[i] = q
		q.start()

		writeStmt, err := db.Prepare(getWriterQuery(tableName))
		if err != nil {
			return nil, fmt.Errorf("prepare writer query for %q: %w", tableName, err)
		}
		writers[i] = writeStmt
	}

	return &concurrentRepository{
		db:         db,
		queriers:   queriers,
		globalQuit: globalQuit,
		writers:    writers,
	}, nil
}

func (r *concurrentRepository) FindByURL(ctx context.Context, url string) (*Response, error) {
	size := len(r.queriers)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	respCh := make(chan *Response, size)
	unitWork := work{
		ctx:  ctx,
		url:  url,
		resp: respCh,
	}

	go func() {
		for _, q := range r.queriers {
			q.source <- unitWork
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

func (r *concurrentRepository) Write(ctx context.Context, url string, resp *Response) error {
	r.muWriter.Lock()
	defer r.muWriter.Unlock()
	r.currentWriter++
	if r.currentWriter >= len(r.writers) {
		r.currentWriter = 0
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: false})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()
	if resp.TableName != "" {
		_, err = tx.ExecContext(ctx, "DELETE FROM "+resp.TableName+" WHERE url = ?", url)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	err = execWriter(ctx, tx.StmtContext(ctx, r.writers[r.currentWriter]), url, resp)
	if err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
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

type querier struct {
	source    chan work
	quit      chan struct{}
	stmt      *sql.Stmt
	tableName string
}

func newQuerier(stmt *sql.Stmt, tableName string) *querier {
	return &querier{
		stmt:      stmt,
		tableName: tableName,
	}
}

func (q *querier) start() {
	q.source = make(chan work, 10)
	go func() {
		for {
			select {
			case unitWork := <-q.source:
				response, err := rowToResponse(q.stmt.QueryRowContext(unitWork.ctx, unitWork.url))
				if err != nil {
					unitWork.resp <- nil
					continue
				}
				response.TableName = q.tableName
				unitWork.resp <- response

			case <-q.quit:
				q.stmt.Close()
				return
			}
		}
	}()
}
