# sqlite-http-cache
SQLite Extension to cache HTTP requests

## Compiling

```sh
go build -ldflags="-s -w" -buildmode=c-shared -o httpcache.so
```

## Basic usage

```sh
sqlite3

# Load the extension
.load /path/to/httpcache.so

# Insert URL into the temp.http_request virtual table to trigger the HTTP Request 
INSERT INTO temp.http_request VALUES('https://swapi.tech/api/films/1');

# Fetch data from http_response table (created by the extension)
SELECT JSON_EXTRACT(body, '$.result.properties.title') AS title,
  JSON_EXTRACT(body, '$.result.properties.release_date') AS release_date 
  FROM http_response;

# Use cacheage, cachelifetime or cachexpired function to check cache validity based on RFC9111
SELECT url, cacheage(header) AS age, cachelifetime(header, request_time, true) AS lifetime, cachexpired(header, request_time, true) AS expired 
FROM http_response; 
```

## Configuring

You can configure the behaviour by passing parameters to a VIRTUAL TABLE.

| Param | Description | Default |
|-------|-------------|---------|
| timeout | Timeout in milliseconds | 0 |
| insecure | Insecure skip TLS validation | false |
| status_code | Comma-separated list of HTTP status code to persist. Use empty to persist all status | 200,301,404 |
| response_table | Database table used to store response data | http_response |
| oauth2_client_id | Oauth2 Client ID | |
| oauth2_client_secret | Oauth2 Client Secret | |
| oauth2_token_url | Oauth2 Token URL (Client Credentials Flow) | |
| cert_file | Mutual TLS: path to certificate file | |
| crt_key_file | Mutual TLS: path to certificate key file | |
| ca_file | Path to CA certificate file | |

**Any other parameter will be included as an HTTP header in the request** 

You ca use environment variable in the param values. Example:

```sql
CREATE VIRTUAL TABLE temp.custom_request USING http_request(authorization=Bearer ${API_TOKEN});
```

### Examples

#### Customizing the request 

```sh
# Create a Virtual Table to customize options
CREATE VIRTUAL TABLE temp.custom_request USING http_request(insecure=true, timeout=10000, accept=application/json, authorization=Bearer ${API_TOKEN}, response_table=films);

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

1. Install

```sh
go install github.com/walterwanderley/cmd/sqlite-http-refresh@latest
```

2. Run

```sh
sqlite-http-refresh file:example.db?_journal=WAL&_sync=NORMAL&_timeout=5000&_txlock=immediate
```

### Operating System Schedulers

- **Cron (Linux/macOS):** You can set up cron jobs to execute a script at specified intervals (e.g., every minute, hour, or day). This script would then connect to your SQLite database and perform the desired INSERT operations.

- **Task Scheduler (Windows):** Similar to cron, Windows Task Scheduler allows you to schedule tasks, including running scripts or programs that interact with SQLite.

### Programming Language Libraries

- **Python:** Libraries like schedule, APScheduler, or Celery can be used to define and manage scheduled tasks within a Python application. This application would then execute INSERT statements into your SQLite database at the specified times.

- **Node.js:** Libraries such as node-cron or agenda provide similar scheduling capabilities for Node.js applications.

- **Golang:** [Check an example](https://github.com/walterwanderley/sqlite-http-cache/blob/main/cmd/sqlite-http-refresh/main.go) using the stdlib.
