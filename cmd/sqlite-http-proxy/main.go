package main

import (
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/elazarl/goproxy"
	_ "github.com/mattn/go-sqlite3"

	"github.com/walterwanderley/sqlite-http-cache/db"
)

var (
	port       uint
	dbParams   string
	allowHTTP2 bool
	verbose    bool

	ttl uint

	caCert    string
	caCertKey string

	responseTables    string
	forceCreateTables bool
	readOnly          bool
)

func main() {
	flag.UintVar(&port, "p", 8080, "Server port")
	flag.StringVar(&dbParams, "db-params", "_journal=WAL&_sync=NORMAL&_timeout=5000&_txlock=immediate", "Database connection params")
	flag.BoolVar(&verbose, "v", false, "Enable verbose mode")
	flag.BoolVar(&allowHTTP2, "h2", false, "Allow HTTP2")
	flag.UintVar(&ttl, "ttl", 0, "Time to Live in seconds (0 is infinite time)")
	flag.StringVar(&responseTables, "response-tables", "", "Comma separated list of database tables used to store response data")
	flag.BoolVar(&forceCreateTables, "force-create-tables", false, "Force create response tables if not exists")
	flag.StringVar(&caCert, "ca-cert", "", "Path to CA Certificate file (required to HTTPS proxy)")
	flag.StringVar(&caCertKey, "ca-cert-key", "", "Path to CA Certificate Key file (required to HTTPS proxy)")
	flag.BoolVar(&readOnly, "ro", false, "Read Only mode. Do not store new HTTP responses")
	flag.Parse()

	if len(flag.Args()) == 0 {
		log.Fatalf("Usage: %s <flags> [DatabasePath1] [DatabasePathN\n\nExample:\n\t%s example.db example2.db example3.db\n", os.Args[0], os.Args[0])
	}

	dbs := make([]*sql.DB, 0)
	var (
		repository db.Repository
		tableList  []string
		err        error
	)
	for _, file := range flag.Args() {
		var dsn string
		if file == ":memory:" {
			dsn = file + "?cache=shared"
		} else {
			dsn = fmt.Sprintf("file:%s?%s", file, dbParams)
		}

		sqlDB, err := sql.Open("sqlite3", dsn)
		if err != nil {
			log.Fatalf("open db error: %v", err)
		}
		defer sqlDB.Close()

		dbs = append(dbs, sqlDB)

		if responseTables == "" {
			tableList, err = db.ResponseTables(sqlDB)
			if err != nil {
				log.Fatalf("discovery response tables: %v", err)
			}
		} else {
			tableList = strings.Split(responseTables, ",")
			if forceCreateTables {
				err := db.CreateResponseTables(sqlDB, tableList...)
				if err != nil {
					log.Fatalf("force create tables: %v", err)
				}
			}
		}
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
	proxy.Verbose = verbose
	proxy.AllowHTTP2 = allowHTTP2

	if caCert != "" && caCertKey != "" {
		proxy.Logger.Printf("INFO: Starting HTTP/HTTPS Proxy...")
		cert, err := parseCA([]byte(caCert), []byte(caCertKey))
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

	proxy.OnRequest().Do(&requestHandler{
		querier: repository,
	})
	if !readOnly {
		proxy.OnResponse().Do(&responseHandler{
			writer: repository,
		})
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("cannot open port %d: %v", port, err)
	}

	proxy.Logger.Printf("SQLite-HTTP-Proxy listening port=%d", port)
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
