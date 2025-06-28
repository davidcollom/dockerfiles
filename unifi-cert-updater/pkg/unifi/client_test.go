package unifi

import (
	"net/http"
	"net/http/httptest"
)

func setupTestServer(response string, status int) (*httptest.Server, *UniFiClient) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))

	client := &UniFiClient{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	return server, client
}
