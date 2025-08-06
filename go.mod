module github.com/walterwanderley/sqlite-http-cache

go 1.24

replace go.riyazali.net/sqlite => ../sqlite

require (
	go.riyazali.net/sqlite v0.0.0
	golang.org/x/oauth2 v0.30.0
)

require github.com/mattn/go-pointer v0.0.1 // indirect
