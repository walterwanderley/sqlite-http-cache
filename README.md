# sqlite-http-cache

This repository contains tools to integrate with the [httpcache SQLite Extension](https://github.com/litesql/httpcache).


## SQLite HTTP Proxy Cache

The sqlite-http-proxy is an HTTP proxy cache that can store data in multiple sqlite databases and query concurrently to get the faster response. The cache implements [RFC9111](https://www.rfc-editor.org/rfc/rfc9111.html) (except for the Vary header).

1. Installation:

Download sqlite-http-proxy from the [releases page](https://github.com/walterwanderley/sqlite-http-cache/releases), or install from source:

```sh
go install github.com/walterwanderley/cmd/sqlite-http-proxy@latest
```

2. Executing:

```sh
sqlite-http-proxy --port 9090 --response-table http_response proxy1.db proxy2.db proxy3.db
```

3. Testing:

```sh
time curl -x http://127.0.0.1:9090 http://swapi.tech/api/films/1
time curl -x http://127.0.0.1:9090 http://swapi.tech/api/films/1
```

### Proxing HTTPS Requests

To proxy https requests you need to pass CA Certificate and CA Certificate key to the sqlite-http-proxy.

```sh
sqlite-http-proxy --ca-cert=/path/to/ca.crt --ca-cert-key=/path/to/ca.key proxyN.db
```

Use the command line flag --help for more info.

```sh
sqlite-http-proxy --help
```

## Refresh data

To schedule inserts in SQLite, a common approach involves using external scheduling mechanisms as SQLite itself does not have a built-in scheduler for timed operations or recurring tasks.

### sqlite-http-refresh

1. Install from source or download from [releases page](https://github.com/walterwanderley/sqlite-http-cache/releases)

```sh
go install github.com/walterwanderley/cmd/sqlite-http-refresh@latest
```

2. Run

```sh
sqlite-http-refresh file:example.db?_journal=WAL&_sync=NORMAL&_timeout=5000&_txlock=immediate
```

### Operating System Schedulers

You can set up Cron Jobs (or Task Scheduler) to execute a script at specified intervals (e.g., every minute, hour, or day). This script would then connect to your SQLite database and perform the desired INSERT operations.

Example:

```sql
INSERT INTO temp.http_request 
SELECT url FROM http_response 
WHERE unixepoch() - unixepoch(response_time) > :ttl ;
```
*ttl is Time to Live in seconds*