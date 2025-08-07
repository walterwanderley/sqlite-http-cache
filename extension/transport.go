package extension

import (
	"crypto/tls"
	"net/http"
)

type transport struct {
	http.RoundTripper
	Headers map[string]string
}

func newTransport(tlsConfig *tls.Config, headers map[string]string) http.RoundTripper {
	baseTransport := http.DefaultTransport.(*http.Transport).Clone()
	baseTransport.TLSClientConfig = tlsConfig
	if len(headers) == 0 {
		return baseTransport
	}
	return &transport{
		baseTransport, headers,
	}
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	for k, v := range t.Headers {
		req2.Header.Set(k, v)
	}
	return t.RoundTripper.RoundTrip(req2)
}
