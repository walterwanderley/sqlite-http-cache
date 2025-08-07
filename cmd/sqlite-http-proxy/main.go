package main

import (
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/elazarl/goproxy"
	_ "github.com/mattn/go-sqlite3"

	"github.com/walterwanderley/sqlite-http-cache/db"
)

func main() {
	var (
		port       uint
		allowHTTP2 bool
		verbose    bool

		caCert    string
		caCertKey string

		responseTables string
	)
	flag.UintVar(&port, "p", 8080, "Server port")
	flag.BoolVar(&verbose, "v", false, "Enable verbose mode")
	flag.BoolVar(&allowHTTP2, "h2", false, "Allow HTTP2")
	flag.StringVar(&responseTables, "response-tables", "", "Comma separated list of database tables used to store response data")
	flag.StringVar(&caCert, "ca-cert", "", "Path to CA Certificate file (required to HTTPS proxy)")
	flag.StringVar(&caCertKey, "ca-cert-key", "", "Path to CA Certificate Key file (required to HTTPS proxy)")
	flag.Parse()

	if len(flag.Args()) != 1 {
		log.Fatalf("Usage: %s <flags> [DSN]\n\nExample:\n\t%s file:example.db\n", os.Args[0], os.Args[0])
	}
	dsn := flag.Args()[0]

	sqlDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		log.Fatalf("open db error: %v", err)
	}
	defer sqlDB.Close()

	var tableList []string
	if responseTables == "" {
		tableList, err = db.ResponseTables(sqlDB)
		if err != nil {
			log.Fatalf("discovery response tables: %v", err)
		}
	} else {
		tableList = strings.Split(responseTables, ",")
	}

	repository, err := db.NewRepository(sqlDB, tableList...)
	if err != nil {
		log.Fatalf("new repository: %v", err)
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

	proxy.OnRequest().DoFunc(
		func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			if r.Method != http.MethodGet || ctx.Req == nil || ctx.Req.URL == nil {
				return r, nil
			}

			url := ctx.Req.URL.String()
			resp, err := repository.FindByURL(r.Context(), url)
			if err != nil {
				if !errors.Is(err, sql.ErrNoRows) {
					proxy.Logger.Printf("ERROR: query error: %s", err.Error())
				}
				return r, nil
			}
			if verbose {
				proxy.Logger.Printf("INFO: serving from database url=%s status=%d timestamp=%s", url, resp.Status, resp.Timestamp.Format(time.RFC3339))
			}

			return r, &http.Response{
				StatusCode: resp.Status,
				Body:       resp.Body,
				Header:     http.Header(resp.Headers),
			}
		})

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
