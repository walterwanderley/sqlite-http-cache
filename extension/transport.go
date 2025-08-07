package extension

import (
	"crypto/tls"
	"net/http"
)

type Transport struct {
	http.RoundTripper
	Headers map[string]string
}

func NewTransport(tlsConfig *tls.Config, headers map[string]string) http.RoundTripper {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig
	if len(headers) == 0 {
		return transport
	}
	return &Transport{
		transport, headers,
	}
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	for k, v := range t.Headers {
		req2.Header.Set(k, v)
	}
	return t.RoundTripper.RoundTrip(req2)
}
