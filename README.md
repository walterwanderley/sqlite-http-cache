# sqlite-http-cache
SQLite Extension to cache HTTP requests

and an [SQLite http proxy cache](#sqlite-proxy-cache).

## Installation

Download **httpcache** extension from the [releases page](https://github.com/walterwanderley/sqlite-http-cache/releases).

### Compiling from source

```sh
go build -ldflags="-s -w" -buildmode=c-shared -o httpcache.so
```

## Basic usage

```sh
sqlite3

# Load the extension
.load ./httpcache

# Check extension info
SELECT cache_info();

# Insert URL into the temp.http_request virtual table to trigger the HTTP Request 
INSERT INTO temp.http_request VALUES('https://swapi.tech/api/films/1');

# Set output mode (optional)
.mode qbox

# Fetch data from http_response table (created by the extension)
SELECT JSON_EXTRACT(body, '$.result.properties.title') AS title,
  JSON_EXTRACT(body, '$.result.properties.release_date') AS release_date 
  FROM http_response;

# Use cache_age, cache_lifetime or cache_expired function to check cache validity based on RFC9111
SELECT url, cache_age(header, request_time, response_time) AS age, 
cache_lifetime(header, response_time) AS lifetime, 
cache_expired(header, request_time, response_time, false) AS expired, 
cache_expired_ttl(header, request_time, response_time, false, 3600) AS expiredTTLFallback 
FROM http_response; 
```

All HTTP responses are stored in tables using the following schema:

```sql
CREATE TABLE IF NOT EXISTS http_response(
		url TEXT PRIMARY KEY,
		status INTEGER,
		body BLOB,
		header JSONB,
		request_time DATETIME,
		response_time DATETIME
)
```

## Configuring

You can configure the behaviour by passing parameters to a VIRTUAL TABLE.

| Param | Description | Default |
|-------|-------------|---------|
| timeout | Timeout in milliseconds | 0 |
| insecure | Insecure skip TLS validation | false |
| status_code | Comma-separated list of HTTP status code to persist. Use empty to persist all status | 200, 203, 204, 206, 300, 301, 308, 404, 405, 410, 414, 501 |
| response_table | Database table used to store response data | http_response |
| oauth2_client_id | Oauth2 Client ID | |
| oauth2_client_secret | Oauth2 Client Secret | |
| oauth2_token_url | Oauth2 Token URL (Client Credentials Flow) | |
| cert_file | Mutual TLS: path to certificate file | |
| cert_key_file | Mutual TLS: path to certificate key file | |
| ca_file | Path to CA certificate file | |

**Any other parameter will be included as an HTTP header in the request** 

You ca use environment variable in the param values. Example:

```sql
CREATE VIRTUAL TABLE temp.custom_request USING http_request(authorization='Bearer ${API_TOKEN}');
```

### Examples

#### Customizing the request 

```sh
# Create a Virtual Table to customize options
CREATE VIRTUAL TABLE temp.custom_request USING http_request(insecure=true, timeout=10000, accept=application/json, authorization='Bearer ${API_TOKEN}', response_table=films);

# Insert URL into the Virtual Table to trigger the HTTP Request 
INSERT INTO temp.custom_request VALUES('https://swapi.tech/api/films/2');

# Query the response table
SELECT JSON_EXTRACT(body, '$.result.properties.title') AS title,
  JSON_EXTRACT(body, '$.result.properties.release_date') AS release_date 
  FROM films;
```

#### Configuring Oauth2 Client Credentials

```sh
CREATE VIRTUAL TABLE temp.oauth2_request USING http_request(oauth2_client_id=${CLIENT_ID}, oauth2_client_secret=${CLIENT_SECRET}, oauth2_token_url='https://my-token-url');

INSERT INTO temp.oauth2_request VALUES('https://swapi.tech/api/films/3');

SELECT JSON_EXTRACT(body, '$.result.properties.title') AS title,
  JSON_EXTRACT(body, '$.result.properties.release_date') AS release_date 
  FROM http_response;
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

## SQLite Proxy Cache

The sqlite-http-proxy is an HTTP proxy cache that can store data in multiple sqlite databases and query concurrently to get the faster response. The cache imlements the RFC9111 (except the vary header).

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

```sh
sqlite-http-proxy --help
```