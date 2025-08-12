package main

import (
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/elazarl/goproxy"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"
	"github.com/tursodatabase/go-libsql"

	"github.com/walterwanderley/sqlite-http-cache/config"
	"github.com/walterwanderley/sqlite-http-cache/db"
	proxyhandler "github.com/walterwanderley/sqlite-http-cache/http/proxy"
)

func main() {
	fs := ff.NewFlagSet("libsql-http-proxy")
	port := fs.Uint('p', "port", 8080, "Server port")
	dbPrimaryURL := fs.StringLong("db-primary-url", "", "Database primary URL")
	dbSyncInterval := fs.DurationLong("db-sync-interval", 30*time.Second, "Database sync interval")
	dbAuthToken := fs.StringLong("db-token", "", "Database authorization token")
	dbEncryptionKey := fs.StringLong("db-key", "", "Database encryption key")
	verbose := fs.Bool('v', "verbose", "Enable verbose mode")
	allowHTTP2 := fs.BoolLong("h2", "Allow HTTP2")
	statusCodes := fs.StringListLong("status-code", fmt.Sprintf("List of cacheable status code. Defaults to the heuristically cacheable codes: %v", config.DefaultStatusCodes()))
	ttl := fs.IntLong("ttl", 0, "Time to Live in seconds (0 is infinite time)")
	responseTables := fs.StringListLong("response-table", "List of database tables used to store response data")
	caCert := fs.StringLong("ca-cert", "", "Path to CA Certificate file (required to HTTPS proxy)")
	caCertKey := fs.StringLong("ca-cert-key", "", "Path to CA Certificate Key file (required to HTTPS proxy)")
	readOnly := fs.BoolLong("ro", "Read Only mode. Do not store new HTTP responses")
	rfc9111 := fs.BoolLong("rfc9111", "Use RFC9111 spec")
	shared := fs.BoolLong("shared", "Enable shared cache mode")
	_ = fs.String('c', "config", "", "config file (optional)")

	if err := ff.Parse(fs, os.Args[1:],
		ff.WithEnvVarPrefix("LIBSQL_HTTP_PROXY"),
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ff.PlainParser),
	); err != nil {
		fmt.Printf("%s\n", ffhelp.Flags(fs))
		fmt.Printf("err=%v\n", err)
		return
	}

	if len(fs.GetArgs()) == 0 {
		log.Fatalf("Usage: %s <FLAGS> [DatabasePath1] [DatabasePathN\n\nExample:\n\t%s example.db example2.db example3.db\n", os.Args[0], os.Args[0])
	}

	if *verbose {
		fmt.Printf("Using options: port=%d db-primary-url=%s, h2=%v, ttl=%d, response-tables=%v, ca-cert=%s, ca-cert-key=%s, read-only=%v, rfc9111=%v shared-cache=%v\n",
			*port, *dbPrimaryURL, *allowHTTP2, *ttl, *responseTables, *caCert, *caCertKey, *readOnly, *rfc9111, *shared)
	}

	dbs := make([]*sql.DB, 0)
	var (
		repository db.Repository
		tableList  []string
		err        error
	)

	dbOpts := make([]libsql.Option, 0)
	if *dbAuthToken != "" {
		dbOpts = append(dbOpts, libsql.WithAuthToken(*dbAuthToken))
	}
	if *dbEncryptionKey != "" {
		dbOpts = append(dbOpts, libsql.WithEncryption(*dbEncryptionKey))
	}
	if *dbSyncInterval > 0 {
		dbOpts = append(dbOpts, libsql.WithSyncInterval(*dbSyncInterval))
	}

	fnRegisterResonseTables := func(sqlDB *sql.DB, dbPath string) {
		if responseTables == nil || len(*responseTables) == 0 {
			tableList, err = db.ResponseTables(sqlDB)
			if err != nil {
				log.Fatalf("discovery response tables: %v", err)
			}
		} else {
			tableList = *responseTables
			err := db.CreateResponseTables(sqlDB, tableList...)
			if err != nil {
				log.Fatalf("create response tables on DB %q: %v", dbPath, err)
			}
		}
	}

	for _, dbPath := range fs.GetArgs() {
		if strings.HasPrefix(dbPath, "file:") {
			sqlDB, err := sql.Open("libsql", dbPath)
			if err != nil {
				log.Fatalf("connecting to database %q: %v", dbPath, err)
			}
			fnRegisterResonseTables(sqlDB, dbPath)
			dbs = append(dbs, sqlDB)
			continue
		}
		dir, err := os.MkdirTemp("", "libsql-*")
		if err != nil {
			log.Fatalf("creating database directory: %v", err)
		}
		defer os.RemoveAll(dir)

		connector, err := libsql.NewEmbeddedReplicaConnector(filepath.Join(dir, dbPath), *dbPrimaryURL, dbOpts...)
		if err != nil {
			log.Fatalf("creating database connector: %v", err)
		}
		defer connector.Close()

		sqlDB := sql.OpenDB(connector)
		defer func() {
			if closeError := sqlDB.Close(); closeError != nil {
				fmt.Println("Error closing database", closeError)
				if err == nil {
					err = closeError
				}
			}
		}()

		fnRegisterResonseTables(sqlDB, dbPath)
		dbs = append(dbs, sqlDB)

	}
	if len(dbs) == 1 {
		repository, err = db.NewRepository(dbs[0], tableList...)
		if err != nil {
			log.Fatalf("new repository: %v", err)
		}
	} else {
		repository, err = db.NewMultiDatabaseRepository(dbs)
		if err != nil {
			log.Fatalf("new multi database repository: %v", err)
		}
	}
	defer repository.Close()

	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = *verbose
	proxy.AllowHTTP2 = *allowHTTP2

	if *caCert != "" && *caCertKey != "" {
		proxy.Logger.Printf("INFO: Starting HTTP/HTTPS Proxy...")
		cert, err := parseCA([]byte(*caCert), []byte(*caCertKey))
		if err != nil {
			log.Fatal(err)
		}

		customCaMitm := &goproxy.ConnectAction{Action: goproxy.ConnectMitm, TLSConfig: goproxy.TLSConfigFromCA(cert)}
		var customAlwaysMitm goproxy.FuncHttpsHandler = func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
			return customCaMitm, host
		}
		proxy.OnRequest().HandleConnect(customAlwaysMitm)
	} else {
		proxy.Logger.Printf("INFO: Starting HTTP Proxy...")
	}

	cacheableStatus := make([]int, 0)
	for _, status := range *statusCodes {
		statusStr := strings.TrimSpace(status)
		code, err := strconv.Atoi(statusStr)
		if err != nil {
			log.Fatalf("Invalid status-code %q. Must be integer: %v", status, err)
		}
		cacheableStatus = append(cacheableStatus, code)
	}
	if len(cacheableStatus) == 0 {
		cacheableStatus = config.DefaultStatusCodes()
	}

	proxy.OnRequest().Do(proxyhandler.NewRequestHandler(
		proxyhandler.RequestConfig{
			Querier:         repository,
			CacheableStatus: cacheableStatus,
			TTL:             *ttl,
			RFC9111:         *rfc9111,
			SharedCache:     *shared,
			ReadOnly:        *readOnly,
			Verbose:         *verbose,
		},
	))

	if !*readOnly {
		proxy.OnResponse().Do(proxyhandler.NewResponseHandler(
			proxyhandler.ResponseConfig{
				Writer:      repository,
				RFC9111:     *rfc9111,
				TTL:         *ttl,
				SharedCache: *shared,
				Verbose:     *verbose,
			},
		))
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("cannot open port %d: %v", port, err)
	}

	proxy.Logger.Printf("LibSQL-HTTP-Proxy listening port=%d", *port)
	log.Fatal(http.Serve(lis, proxy))
}

func parseCA(caCert, caKey []byte) (*tls.Certificate, error) {
	parsedCert, err := tls.X509KeyPair(caCert, caKey)
	if err != nil {
		return nil, err
	}
	if parsedCert.Leaf, err = x509.ParseCertificate(parsedCert.Certificate[0]); err != nil {
		return nil, err
	}
	return &parsedCert, nil
}
