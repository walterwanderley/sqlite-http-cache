package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/walterwanderley/sqlite-http-cache/config"
	_ "github.com/walterwanderley/sqlite-http-cache/extension"
)

var (
	interval time.Duration
	ttl      uint
	matchURL string

	timeout           uint
	insecure          bool
	ignoreStatusError bool
	responseTableName string

	oauth2ClientID     string
	oauth2ClientSecret string
	oauth2TokenURL     string

	certFile    string
	certKeyFile string
	caFile      string
)

func main() {
	// Scheduler strategy
	flag.DurationVar(&interval, "check-interval", 30*time.Second, "Interval to wait for check expired data")
	flag.UintVar(&ttl, "ttl", 30*60, "Time to Live in seconds")
	flag.StringVar(&matchURL, "match-url", "", "Filter URLs")
	// Request/Store config
	flag.UintVar(&timeout, "timeout", 30*1000, "Timeout in milliseconds")
	flag.BoolVar(&insecure, "insecure", false, "Disable TLS verification")
	flag.BoolVar(&ignoreStatusError, "ignore-status-error", false, "ignore responses with status code != 2xx")
	flag.StringVar(&responseTableName, "response-table", config.DefaultResponseTableName, "Database table used to store response data")
	// Oauth2 Client Credentials
	flag.StringVar(&oauth2ClientID, "oauth2-client-id", "", "Oauth2 Client ID")
	flag.StringVar(&oauth2ClientSecret, "oauth2-client-secret", "", "Oauth2 Client Secret")
	flag.StringVar(&oauth2TokenURL, "oauth2-token-url", "", "Oauth2 Token URL (Client Credentials Flow)")
	// mTLS
	flag.StringVar(&certFile, "cert-file", "", "mTLS: Path to the Client Certificate file")
	flag.StringVar(&certKeyFile, "cert-key-file", "", "mTLS: Path to the Client Certificate Key file")
	flag.StringVar(&caFile, "ca-file", "", "Path to the CA file")
	flag.Parse()

	if len(strings.TrimSpace(responseTableName)) == 0 {
		log.Fatalf("response-table cannot be empty")
	}

	args := flag.Args()
	if len(args) != 1 {
		log.Fatalf("Usage: %s <flags> [DSN]\n\nExample:\n\t%s file:example.db?_journal=WAL&_sync=NORMAL&_timeout=5000&_txlock=immediate\n", os.Args[0], os.Args[0])
	}
	dsn := args[0]

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		log.Fatalf("cannot connect to the database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(fmt.Sprintf("CREATE VIRTUAL TABLE temp.http_refresh USING http_request(%s)", opts()))
	if err != nil {
		log.Fatalf("error creating virtual table: %v", err)
	}

	stmt, err := db.Prepare(fmt.Sprintf(`INSERT INTO temp.http_refresh(url) 
	SELECT url FROM %s
	WHERE url LIKE ? AND unixepoch() - unixepoch(timestamp) > ?`, responseTableName))
	if err != nil {
		log.Fatalf("error preparing statement: %v", err)
	}
	defer stmt.Close()

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	refreshData(stmt)
	if interval == 0 {
		return
	}

	fmt.Println("Press CTRL+C to exit")

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			refreshData(stmt)
		case <-done:
			return
		}
	}
}

func refreshData(stmt *sql.Stmt) {
	slog.Info("starting data verification")

	res, err := stmt.Exec(matchURL, ttl)
	if err != nil {
		slog.Error("error refreshing data", "error", err)
		return
	}

	rowsAffected, _ := res.RowsAffected()
	slog.Info("verification finished", "rows_affected", rowsAffected)
}

func opts() string {
	opts := make([]string, 0)
	opts = append(opts, fmt.Sprintf("%s='%d'", config.Timeout, timeout))
	opts = append(opts, fmt.Sprintf("%s='%v'", config.Insecure, insecure))
	opts = append(opts, fmt.Sprintf("%s='%v'", config.IgnoreStatusError, ignoreStatusError))
	opts = append(opts, fmt.Sprintf("%s='%s'", config.ResponseTableName, responseTableName))
	opts = append(opts, fmt.Sprintf("%s='%s'", config.Oauth2ClientID, oauth2ClientID))
	opts = append(opts, fmt.Sprintf("%s='%s'", config.Oauth2ClientSecret, oauth2ClientSecret))
	opts = append(opts, fmt.Sprintf("%s='%s'", config.Oauth2TokenURL, oauth2TokenURL))
	opts = append(opts, fmt.Sprintf("%s='%s'", config.CertFile, certFile))
	opts = append(opts, fmt.Sprintf("%s='%s'", config.CertKeyFile, certKeyFile))
	opts = append(opts, fmt.Sprintf("%s='%s'", config.CertCAFile, caFile))
	return strings.Join(opts, ", ")
}
