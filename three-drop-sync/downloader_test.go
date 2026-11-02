package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func downloadTestFile(server *httptest.Server, name string) DownloadFile {
	return DownloadFile{URL: server.URL + "/" + name, Website: "example.com", ModelID: "123", Name: name}
}
func testDownloadOptions(t *testing.T, server *httptest.Server, files ...DownloadFile) DownloadOptions {
	t.Helper()
	return DownloadOptions{Output: t.TempDir(), Files: files, Concurrency: 3, MaxFileSize: 1024, client: server.Client()}
}
func TestDownloadConcurrencyAndLimit(t *testing.T) {
	var active, peak atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := active.Add(1)
		defer active.Add(-1)
		for p := peak.Load(); n > p && !peak.CompareAndSwap(p, n); p = peak.Load() {
		}
		time.Sleep(30 * time.Millisecond)
		fmt.Fprint(w, "model")
	}))
	defer server.Close()
	o := testDownloadOptions(t, server)
	for i := 0; i < 8; i++ {
		o.Files = append(o.Files, downloadTestFile(server, fmt.Sprintf("%d.stl", i)))
	}
	o.MaxFiles = 4
	summary, err := Download(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Downloaded != 4 || summary.Limited != 4 {
		t.Fatalf("unexpected summary %+v", summary)
	}
	if peak.Load() < 2 || peak.Load() > 3 {
		t.Fatalf("worker concurrency %d", peak.Load())
	}
	summary, err = Download(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Downloaded != 4 || summary.Skipped != 4 || summary.Limited != 0 {
		t.Fatalf("verified files consumed limit: %+v", summary)
	}
}
func TestDownloadCorruptionAndVersions(t *testing.T) {
	var payload atomic.Value
	payload.Store("first")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, payload.Load()) }))
	defer server.Close()
	f := downloadTestFile(server, "model.stl")
	o := testDownloadOptions(t, server, f)
	if _, err := Download(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte("first"))
	firstHash := hex.EncodeToString(hash[:])
	target := filepath.Join(fileDirectory(o.Output, f), firstHash, f.Name)
	if err := os.WriteFile(target, []byte("corruption"), 0600); err != nil {
		t.Fatal(err)
	}
	summary, err := Download(context.Background(), o)
	if err != nil || summary.Downloaded != 1 {
		t.Fatalf("corrupt file wasn't repaired: %+v %v", summary, err)
	}
	backups, _ := filepath.Glob(filepath.Join(filepath.Dir(target), ".corrupt-*"))
	if len(backups) != 1 {
		t.Fatal("corrupt original was not preserved")
	}
	payload.Store("second")
	hash = sha256.Sum256([]byte("second"))
	o.Files[0].SHA256 = hex.EncodeToString(hash[:])
	if _, err := Download(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(target)
	if err != nil || string(b) != "first" {
		t.Fatal("changed content overwrote previous version")
	}
}
func TestDownloadSizeAndChecksum(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.(http.Flusher).Flush()
		fmt.Fprint(w, strings.Repeat("x", 100))
	}))
	defer server.Close()
	o := testDownloadOptions(t, server, downloadTestFile(server, "model.stl"))
	o.MaxFileSize = 10
	summary, err := Download(context.Background(), o)
	if err == nil || summary.Failed != 1 {
		t.Fatalf("size limit not enforced: %+v %v", summary, err)
	}
	paths, _ := filepath.Glob(filepath.Join(fileDirectory(o.Output, o.Files[0]), ".download-*"))
	if len(paths) != 0 {
		t.Fatal("temporary download left behind")
	}
	o.MaxFileSize = 1024
	o.Files[0].SHA256 = strings.Repeat("0", 64)
	if _, err := Download(context.Background(), o); err == nil {
		t.Fatal("incorrect checksum accepted")
	}
}
func TestDownloadCancellation(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done() }))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	o := testDownloadOptions(t, server, downloadTestFile(server, "model.stl"))
	go func() { <-started; cancel() }()
	_, err := Download(ctx, o)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation missing: %v", err)
	}
}
func TestDownloadSafety(t *testing.T) {
	base := DownloadFile{URL: "https://example.com/model.stl", Website: "example.com", ModelID: "123", Name: "model.stl"}
	for _, name := range []string{"../model.stl", "..", "a\\b", "/tmp/file"} {
		f := base
		f.Name = name
		if ValidateDownloadManifest([]DownloadFile{f}) == nil {
			t.Fatalf("accepted name %s", name)
		}
	}
	for _, url := range []string{"http://example.com/a", "https://127.0.0.1/a", "https://[::1]/a", "https://100.64.0.1/a", "https://user:secret@example.com/a"} {
		f := base
		f.URL = url
		if ValidateDownloadManifest([]DownloadFile{f}) == nil {
			t.Fatalf("accepted URL %s", url)
		}
	}
	for _, address := range []string{"127.0.0.1", "10.0.0.1", "169.254.1.2", "::1", "fc00::1", "100.64.0.1", "192.0.2.1"} {
		if publicIP(net.ParseIP(address)) {
			t.Fatalf("accepted IP %s", address)
		}
	}
	if !publicIP(net.ParseIP("1.1.1.1")) {
		t.Fatal("public IP rejected")
	}
	if ValidateDownloadManifest([]DownloadFile{base, base}) == nil {
		t.Fatal("duplicate accepted")
	}
	root := t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if safeDirectory(filepath.Join(root, "link", "nested")) == nil {
		t.Fatal("symlink accepted")
	}
}
func TestDownloadRetry(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			return
		}
		fmt.Fprint(w, "model")
	}))
	defer server.Close()
	o := testDownloadOptions(t, server, downloadTestFile(server, "model.stl"))
	o.Retries = 1
	summary, err := Download(context.Background(), o)
	if err != nil || summary.Downloaded != 1 || calls.Load() != 2 {
		t.Fatalf("retry failed: %+v %v", summary, err)
	}
}
func TestReadDownloadManifest(t *testing.T) {
	p := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(p, []byte(`[{"url":"https://example.com/file","website":"example.com","model_id":"1","name":"file.stl"}]`), 0600); err != nil {
		t.Fatal(err)
	}
	files, err := ReadDownloadManifest(p)
	if err != nil || len(files) != 1 {
		t.Fatalf("manifest failed %v", err)
	}
	if err := os.WriteFile(p, []byte(`[] {}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadDownloadManifest(p); err == nil {
		t.Fatal("trailing JSON accepted")
	}
}

type downloadRoundTripper func(*http.Request) (*http.Response, error)

func (f downloadRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDownloadProtectedRedirectAndDial(t *testing.T) {
	client := downloadHTTPClient()
	transport := client.Transport.(*http.Transport)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if conn, err := transport.DialContext(ctx, "tcp", "127.0.0.1:443"); err == nil {
		conn.Close()
		t.Fatal("protected dial accepted private IP")
	}
	client.Transport = downloadRoundTripper(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"https://127.0.0.1/secret"}}, Body: http.NoBody, Request: r}, nil
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/file", nil)
	if _, err := client.Do(req); err == nil {
		t.Fatal("protected client followed private redirect")
	}
}

func TestDownloadRetryAfterBudget(t *testing.T) {
	for _, header := range []string{"31", "999999999999999999999999999", time.Now().Add(time.Hour).UTC().Format(http.TimeFormat)} {
		if _, err := retryDelay(header, 0); err == nil {
			t.Fatalf("accepted Retry-After %s beyond budget", header)
		}
	}
	delay, err := retryDelay("2", 0)
	if err != nil || delay != 2*time.Second {
		t.Fatalf("Retry-After seconds: %v %v", delay, err)
	}
}

func TestDownloadOutputLock(t *testing.T) {
	root := t.TempDir()
	lock, err := downloadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if other, err := downloadLock(root); err == nil {
		other.Close()
		t.Fatal("overlapping process accepted")
	}
}
