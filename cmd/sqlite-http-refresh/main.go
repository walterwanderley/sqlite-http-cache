package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	sqlite3 "github.com/mattn/go-sqlite3"

	"github.com/walterwanderley/sqlite-http-cache/config"
)

var (
	interval time.Duration
	ttl      uint

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
	// Scheduler
	flag.DurationVar(&interval, "check-interval", 30*time.Second, "Interval to wait for check expired data")
	flag.UintVar(&ttl, "ttl", 30*60, "Time to Live in seconds")
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
	if len(args) != 2 {
		log.Fatalf("Usage: %s [DSN] [Path to sqlite-http-cache Extension]\n\nExample:\n\t%s file:test.db /path/to/http_cache.so\n", os.Args[0], os.Args[0])
	}
	dsn, extensionPath := args[0], args[1]

	driverName := "sqlite-http-cache"
	sql.Register(driverName, &sqlite3.SQLiteDriver{
		ConnectHook: func(conn *sqlite3.SQLiteConn) error {
			return conn.LoadExtension(extensionPath, "sqlite3_extension_init")
		},
	})

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		log.Fatalf("cannot connect to the database: %v", err)
	}
	defer db.Close()

	res, err := db.Exec("INSERT INTO temp.http_request(url) SELECT url FROM http_response")
	if err != nil {
		panic(err)
	}

	rowsAffected, _ := res.RowsAffected()
	fmt.Println("Rows affected:", rowsAffected)

	// TODO polling
}
