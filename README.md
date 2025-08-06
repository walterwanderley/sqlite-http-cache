# sqlite-http-cache
SQLite Extension to cache HTTP requests

## Compiling

```sh
go build -buildmode=c-shared -o http_cache.so
```

## Basic usage

```sh
sqlite3

.load /path/to/http_cache.so

INSERT INTO temp.http_request VALUES('https://swapi.tech/api/films/1');

SELECT * FROM http_response;

SELECT JSON_EXTRACT(body, '$.result.properties.title') AS title,
  JSON_EXTRACT(body, '$.result.properties.release_date') AS release_date 
  FROM http_response;
```

### Configuring

```sh
CREATE VIRTUAL TABLE temp.custom_request USING http_request(insecure=true, timeout=10000, accept=application/json, authorization=Bearer token, response_table=films);

INSERT INTO temp.custom_request VALUES('https://swapi.tech/api/films/2');

SELECT JSON_EXTRACT(body, '$.result.properties.title') AS title,
  JSON_EXTRACT(body, '$.result.properties.release_date') AS release_date 
  FROM films;
```

### Configuring Oauth2 Client Credentials


```sh
CREATE VIRTUAL TABLE temp.oauth2_request USING http_request(oauth2_client_id=ID, oauth2_client_secret=supersecret, oauth2_token_url='https://my-token-url');

INSERT INTO temp.oauth2_request VALUES('https://swapi.tech/api/films/3');

SELECT JSON_EXTRACT(body, '$.result.properties.title') AS title,
  JSON_EXTRACT(body, '$.result.properties.release_date') AS release_date 
  FROM http_response;
```