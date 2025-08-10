package main

import (
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/walterwanderley/sqlite-http-cache/config"
	"github.com/walterwanderley/sqlite-http-cache/db"
	_ "github.com/walterwanderley/sqlite-http-cache/extension"
)

var (
	interval *time.Duration
	ttl      *uint
	matchURL *string

	timeout           *uint
	insecure          *bool
	ignoreStatusError *bool
	responseTables    *[]string

	oauth2ClientID     *string
	oauth2ClientSecret *string
	oauth2TokenURL     *string

	certFile    *string
	certKeyFile *string
	caFile      *string
)

func main() {
	fs := ff.NewFlagSet("sqlite-http-refresh")
	// Scheduler strategy
	interval = fs.DurationLong("sync-interval", 30*time.Second, "Interval to wait for check expired data")
	ttl = fs.UintLong("ttl", 30*60, "Time to Live in seconds")
	matchURL = fs.StringLong("match-url", "%", "Filter URLs (SQL syntax)")
	// Request/Store config
	timeout = fs.UintLong("timeout", 30*1000, "Timeout in milliseconds")
	insecure = fs.BoolLong("insecure", "Disable TLS verification")
	ignoreStatusError = fs.BoolLong("ignore-status-error", "ignore responses with status code != 2xx")
	responseTables = fs.StringListLong("response-table", "List of database tables used to store response data")
	// Oauth2 Client Credentials
	oauth2ClientID = fs.StringLong("oauth2-client-id", "", "Oauth2 Client ID")
	oauth2ClientSecret = fs.StringLong("oauth2-client-secret", "", "Oauth2 Client Secret")
	oauth2TokenURL = fs.StringLong("oauth2-token-url", "", "Oauth2 Token URL (Client Credentials Flow)")
	// mTLS
	certFile = fs.StringLong("cert-file", "", "mTLS: Path to the Client Certificate file")
	certKeyFile = fs.StringLong("cert-key-file", "", "mTLS: Path to the Client Certificate Key file")
	caFile = fs.StringLong("ca-file", "", "Path to the CA file")
	_ = fs.String('c', "config", "", "config file (optional)")

	if err := ff.Parse(fs, os.Args[1:],
		ff.WithEnvVarPrefix("SQLITE_HTTP_REFRESH"),
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ff.PlainParser),
	); err != nil {
		fmt.Printf("%s\n", ffhelp.Flags(fs))
		fmt.Printf("err=%v\n", err)
		return
	}

	args := fs.GetArgs()
	if len(args) != 1 {
		log.Fatalf("Usage: %s <flags> [DSN]\n\nExample:\n\t%s file:example.db?_journal=WAL&_sync=NORMAL&_timeout=5000&_txlock=immediate\n", os.Args[0], os.Args[0])
	}
	dsn := args[0]

	sqlDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		log.Fatalf("cannot connect to the database: %v", err)
	}
	defer sqlDB.Close()

	var tableList []string
	if len(*responseTables) == 0 {
		tableList, err = db.ResponseTables(sqlDB)
		if err != nil {
			log.Fatalf("discovery response tables: %v", err)
		}
	} else {
		tableList = *responseTables
	}

	stmts := make(map[string]*sql.Stmt)
	for _, responseTableName := range tableList {
		_, err = sqlDB.Exec(fmt.Sprintf("CREATE VIRTUAL TABLE temp.%s_refresh USING http_request(%s)", responseTableName, opts(responseTableName)))
		if err != nil {
			log.Fatalf("error creating virtual table: %v", err)
		}

		stmt, err := sqlDB.Prepare(fmt.Sprintf(`INSERT INTO temp.%s_refresh(url) 
	SELECT url FROM %s
	WHERE url LIKE ? AND unixepoch() - unixepoch(timestamp) > ?`, responseTableName, responseTableName))
		if err != nil {
			log.Fatalf("error preparing statement: %v", err)
		}
		defer stmt.Close()
		stmts[responseTableName] = stmt
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	refreshData(stmts)
	if *interval == 0 {
		return
	}

	fmt.Println("Press CTRL+C to exit")

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			refreshData(stmts)
		case <-done:
			return
		}
	}
}

func refreshData(stmts map[string]*sql.Stmt) {
	slog.Info("starting data verification")

	for tableName, stmt := range stmts {
		res, err := stmt.Exec(*matchURL, *ttl)
		if err != nil {
			slog.Error("error refreshing data", "error", err, "table", tableName)
			continue
		}

		rowsAffected, _ := res.RowsAffected()
		slog.Info("verification finished", "rows_affected", rowsAffected, "table", tableName)
	}
}

func opts(responseTableName string) string {
	opts := make([]string, 0)
	opts = append(opts, fmt.Sprintf("%s='%d'", config.Timeout, *timeout))
	opts = append(opts, fmt.Sprintf("%s='%v'", config.Insecure, *insecure))
	opts = append(opts, fmt.Sprintf("%s='%v'", config.IgnoreStatusError, *ignoreStatusError))
	opts = append(opts, fmt.Sprintf("%s='%s'", config.ResponseTableName, responseTableName))
	opts = append(opts, fmt.Sprintf("%s='%s'", config.Oauth2ClientID, *oauth2ClientID))
	opts = append(opts, fmt.Sprintf("%s='%s'", config.Oauth2ClientSecret, *oauth2ClientSecret))
	opts = append(opts, fmt.Sprintf("%s='%s'", config.Oauth2TokenURL, *oauth2TokenURL))
	opts = append(opts, fmt.Sprintf("%s='%s'", config.CertFile, *certFile))
	opts = append(opts, fmt.Sprintf("%s='%s'", config.CertKeyFile, *certKeyFile))
	opts = append(opts, fmt.Sprintf("%s='%s'", config.CertCAFile, *caFile))
	return strings.Join(opts, ", ")
}
