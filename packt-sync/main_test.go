package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestBooksPaginates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Error("missing bearer token")
		}
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		data := []book{}
		if offset < 2 {
			data = []book{{ProductName: "Book " + strconv.Itoa(offset), ProductID: strconv.Itoa(offset)}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"count": 2, "data": data})
	}))
	defer server.Close()
	api := &client{baseURL: server.URL, token: "token", httpClient: server.Client(), log: discardLogger()}
	books, err := api.books(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(books) != 2 {
		t.Fatalf("received %d books, want 2", len(books))
	}
}

func TestSyncBooksDownloadsConcurrently(t *testing.T) {
	var active, maximum atomic.Int32
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/products-v1/") {
			parts := strings.Split(r.URL.Path, "/")
			_ = json.NewEncoder(w).Encode(map[string]string{"data": server.URL + "/download/" + parts[3] + "/" + parts[5]})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/download/") {
			current := active.Add(1)
			defer active.Add(-1)
			for {
				previous := maximum.Load()
				if current <= previous || maximum.CompareAndSwap(previous, current) {
					break
				}
			}
			time.Sleep(40 * time.Millisecond)
			_, _ = w.Write([]byte("book contents"))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	destination := t.TempDir()
	api := &client{baseURL: server.URL, token: "token", httpClient: server.Client(), log: discardLogger()}
	cfg := config{downloadPath: destination, extensions: []string{"epub"}, concurrency: 2}
	books := []book{{"First / Book [eBook]", "1"}, {"Second Book", "2"}, {"Third Book", "3"}, {"Fourth Book", "4"}}
	if err := syncBooks(context.Background(), cfg, api, books); err != nil {
		t.Fatal(err)
	}
	if got := maximum.Load(); got != 2 {
		t.Fatalf("maximum concurrent downloads = %d, want 2", got)
	}
	for _, name := range []string{"First _ Book", "Second Book", "Third Book", "Fourth Book"} {
		contents, err := os.ReadFile(filepath.Join(destination, name, name+".epub"))
		if err != nil {
			t.Fatal(err)
		}
		if string(contents) != "book contents" {
			t.Fatalf("unexpected contents for %s: %q", name, contents)
		}
	}
	matches, err := filepath.Glob(filepath.Join(destination, "*", ".packt-download-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary downloads remain: %v", matches)
	}
}

func TestSyncBooksSkipsExistingFile(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests++ }))
	defer server.Close()
	destination := t.TempDir()
	path := filepath.Join(destination, "Existing", "Existing.pdf")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	api := &client{baseURL: server.URL, httpClient: server.Client(), log: discardLogger()}
	if err := syncBooks(context.Background(), config{downloadPath: destination, extensions: []string{"pdf"}, concurrency: 2}, api, []book{{"Existing", "1"}}); err != nil {
		t.Fatal(err)
	}
	if requests != 0 {
		t.Fatalf("made %d requests for an existing file", requests)
	}
}

func TestAuthenticate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var credentials map[string]string
		if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
			t.Error(err)
		}
		if credentials["username"] != "user" || credentials["password"] != "password" {
			t.Errorf("unexpected credentials: %v", credentials)
		}
		_, _ = w.Write([]byte(`{"data":{"access":"access-token"}}`))
	}))
	defer server.Close()
	api := &client{baseURL: server.URL, httpClient: server.Client(), log: discardLogger()}
	if err := api.authenticate(context.Background(), "user", "password"); err != nil {
		t.Fatal(err)
	}
	if api.token != "access-token" {
		t.Fatalf("token = %q, want access-token", api.token)
	}
}

func TestSafeBookName(t *testing.T) {
	tests := map[string]string{"A/B [EBOOK]": "A_B", "../": "_", "  Ordinary Book  ": "Ordinary Book"}
	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			if got := safeBookName(input); got != want {
				t.Fatalf("safeBookName(%q) = %q, want %q", input, got, want)
			}
		})
	}
}
