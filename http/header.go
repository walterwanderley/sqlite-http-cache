package http

import (
	"net/http"
	"time"
)

func Age(header http.Header) *int {
	if date := header.Get("Date"); date != "" {
		d, err := time.Parse(time.RFC1123, date)
		if err == nil {
			age := int(time.Since(d).Seconds())
			return &age
		}
	}
	return nil
}
