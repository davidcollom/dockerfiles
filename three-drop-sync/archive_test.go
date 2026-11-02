package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Fixtures reflect keys read by the public frontend, not an authenticated
// account export. Live compatibility remains an explicit integration task.
func fixture() Snapshot {
	return Snapshot{
		Likes:       json.RawMessage(`{"liked":["thingiverse:123","makerworld:456"]}`),
		Collections: json.RawMessage(`{"collections":[{"id":"collection-a","name":"Workshop","description":"Preserved in raw backup","items":[{"website":"thingiverse","externalId":"123"},{"website":"printables","externalId":"789"},{"website":"thingiverse","externalId":"123"}]}]}`),
	}
}

func TestCatalogueDeduplicatesMembership(t *testing.T) {
	catalogue, err := fixture().Catalogue()
	if err != nil {
		t.Fatal(err)
	}
	if len(catalogue.Models) != 3 {
		t.Fatalf("got %d models", len(catalogue.Models))
	}
	for _, model := range catalogue.Models {
		if model.Website == "thingiverse" && (!model.Liked || len(model.CollectionIDs) != 1) {
			t.Fatalf("wrong shared model membership: %+v", model)
		}
	}
}

func TestRejectIncompleteResponses(t *testing.T) {
	for _, data := range []string{`{}`, `{"liked":null}`, `{"liked":[12]}`, `{"liked":["missing-separator"]}`, `{"liked":[":123"]}`, `{"liked":["thingiverse:"]}`, `<html>Login</html>`} {
		t.Run(data, func(t *testing.T) {
			snapshot := fixture()
			snapshot.Likes = json.RawMessage(data)
			if _, err := snapshot.Catalogue(); err == nil {
				t.Fatal("accepted invalid likes response")
			}
		})
	}
	for _, data := range []string{`{}`, `{"collections":null}`, `{"collections":[{"id":"a"}]}`, `{"collections":[{"id":"a","items":[{"website":"thingiverse"}]}]}`, `{"collections":[{"id":"a","items":[]},{"id":"a","items":[]}]}`} {
		snapshot := fixture()
		snapshot.Collections = json.RawMessage(data)
		if _, err := snapshot.Catalogue(); err == nil {
			t.Fatalf("accepted invalid collections: %s", data)
		}
	}
}

func TestArchiveIdempotencyHistoryAndCorruption(t *testing.T) {
	root := t.TempDir()
	snapshot := fixture()
	changed, err := SaveArchive(root, snapshot)
	if err != nil || !changed {
		t.Fatalf("first save: changed=%t, err=%v", changed, err)
	}
	before, err := os.ReadFile(filepath.Join(root, "latest.json"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Likes = json.RawMessage("{\n\"liked\": [\"thingiverse:123\", \"makerworld:456\"]\n}")
	changed, err = SaveArchive(root, snapshot)
	if err != nil || changed {
		t.Fatalf("whitespace generated snapshot: changed=%t, err=%v", changed, err)
	}
	invalid := snapshot
	invalid.Collections = json.RawMessage(`{"error":"Unavailable"}`)
	if _, err := SaveArchive(root, invalid); err == nil {
		t.Fatal("accepted partial snapshot")
	}
	after, _ := os.ReadFile(filepath.Join(root, "latest.json"))
	if string(before) != string(after) {
		t.Fatal("invalid response changed latest pointer")
	}
	snapshot.Likes = json.RawMessage(`{"liked":[]}`)
	if changed, err := SaveArchive(root, snapshot); err != nil || !changed {
		t.Fatalf("changed membership: %t %v", changed, err)
	}
	entries, err := os.ReadDir(filepath.Join(root, "snapshots"))
	if err != nil || len(entries) != 2 {
		t.Fatalf("old backup was not retained: %d %v", len(entries), err)
	}
	latest, _ := os.ReadFile(filepath.Join(root, "latest.json"))
	var pointer struct{ Snapshot string }
	if err := json.Unmarshal(latest, &pointer); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, pointer.Snapshot, "likes.json"), []byte(`{}`), 0640); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveArchive(root, snapshot); err == nil {
		t.Fatal("failed to detect corrupted NAS backup")
	}
}

func TestClientAuthenticatedReadOnlyFetch(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.Method != http.MethodGet || r.Header.Get("Cookie") != "session=private" {
			t.Error("wrong request method or session")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case likesEndpoint:
			w.Write(fixture().Likes)
		case collectionsEndpoint:
			w.Write(fixture().Collections)
		default:
			t.Error("unexpected endpoint")
		}
	}))
	defer server.Close()
	client := NewClient("session=private")
	client.baseURL, client.delay = server.URL, 0
	if _, err := client.Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || paths[0] != likesEndpoint || paths[1] != collectionsEndpoint {
		t.Fatalf("wrong requests: %v", paths)
	}
}

func TestClientNeverLeaksCookieViaRedirect(t *testing.T) {
	var called bool
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer destination.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusFound)
	}))
	defer server.Close()
	client := NewClient("session=private")
	client.baseURL = server.URL
	if _, err := client.Fetch(context.Background()); err == nil || called {
		t.Fatalf("redirect followed: called=%t, err=%v", called, err)
	}
}

func TestClientFailureAndRetry(t *testing.T) {
	for _, status := range []int{401, 403, 404, 429, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(status)
				w.Write([]byte("session=private"))
			}))
			defer server.Close()
			client := NewClient("session=private")
			client.baseURL = server.URL
			_, err := client.Fetch(context.Background())
			if err == nil || strings.Contains(err.Error(), "private") {
				t.Fatalf("missing or unsafe error: %v", err)
			}
			expected := 1
			if status == 429 || status == 500 {
				expected = 3
			}
			if calls != expected {
				t.Fatalf("got %d requests, want %d", calls, expected)
			}
		})
	}
}

func TestClientStopsForLongRetryAfter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(429)
	}))
	defer server.Close()
	client := NewClient("session=private")
	client.baseURL = server.URL
	if _, err := client.Fetch(context.Background()); err == nil {
		t.Fatal("ignored long retry delay")
	}
}

func TestClientRejectsLoginHTMLAndPartialCollections(t *testing.T) {
	for _, body := range []string{`<html>Login</html>`, `{"error":"Unknown"}`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == likesEndpoint {
				w.Write(fixture().Likes)
			} else {
				w.Write([]byte(body))
			}
		}))
		client := NewClient("session=private")
		client.baseURL, client.delay = server.URL, 0
		if _, err := client.Fetch(context.Background()); err == nil {
			t.Fatal("accepted incomplete collection response")
		}
		server.Close()
	}
}

func TestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := wait(ctx, time.Hour); err == nil {
		t.Fatal("cancellation ignored")
	}
}

func TestImportAndDryRun(t *testing.T) {
	input := t.TempDir()
	output := filepath.Join(t.TempDir(), "archive")
	for name, content := range map[string]json.RawMessage{"likes.json": fixture().Likes, "collections.json": fixture().Collections} {
		if err := os.WriteFile(filepath.Join(input, name), content, 0600); err != nil {
			t.Fatal(err)
		}
	}
	args := []string{"import", "--input", input, "--output", output, "--dry-run"}
	if err := run(context.Background(), args); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("dry run wrote output")
	}
	if err := run(context.Background(), args[:len(args)-1]); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(output, "latest.json")); err != nil {
		t.Fatal(err)
	}
}

func TestReadCookieAndRejectInvalidSecrets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cookie")
	for _, data := range []string{"", "secret", "a=b\nc=d", strings.Repeat("a", 17000) + "=b"} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadCookie(path); err == nil {
			t.Fatal("accepted invalid secret file")
		}
	}
	if err := os.WriteFile(path, []byte("session=value\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if value, err := ReadCookie(path); err != nil || value != "session=value" {
		t.Fatalf("wrong session read: %v", err)
	}
}

func TestLoadCatalogueChecksIntegrityAndIgnoresUntrustedPath(t *testing.T) {
	root := t.TempDir()
	if _, err := SaveArchive(root, fixture()); err != nil {
		t.Fatal(err)
	}
	latestPath := filepath.Join(root, "latest.json")
	data, err := os.ReadFile(latestPath)
	if err != nil {
		t.Fatal(err)
	}
	var pointer map[string]any
	if err := json.Unmarshal(data, &pointer); err != nil {
		t.Fatal(err)
	}
	pointer["snapshot"] = "../../outside"
	data, _ = json.Marshal(pointer)
	if err := os.WriteFile(latestPath, data, 0640); err != nil {
		t.Fatal(err)
	}
	catalogue, err := LoadCurrentCatalogue(root)
	if err != nil || len(catalogue.Models) != 3 {
		t.Fatalf("unexpected catalogue: %d %v", len(catalogue.Models), err)
	}
	hash := pointer["sha256"].(string)
	if err := os.WriteFile(filepath.Join(root, "snapshots", hash, "likes.json"), []byte(`{"liked":[]}`), 0640); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCurrentCatalogue(root); err == nil {
		t.Fatal("accepted modified snapshot contents")
	}
	if err := os.WriteFile(latestPath, []byte(`{"sha256":"../../outside"}`), 0640); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCurrentCatalogue(root); err == nil {
		t.Fatal("accepted invalid snapshot digest")
	}
}

func TestIntervalSyncKeepsScheduledRetryAfterTransferFailure(t *testing.T) {
	input := t.TempDir()
	for name, content := range map[string]json.RawMessage{"likes.json": fixture().Likes, "collections.json": fixture().Collections} {
		if err := os.WriteFile(filepath.Join(input, name), content, 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	calls := 0
	err := runArchive(ctx, archiveOptions{
		input: input, output: filepath.Join(t.TempDir(), "archive"), interval: time.Millisecond,
		afterArchive: func(context.Context, Catalogue) error {
			calls++
			if calls == 1 {
				return errors.New("provider unavailable")
			}
			cancel()
			return nil
		},
	})
	if !errors.Is(err, context.Canceled) || calls != 2 {
		t.Fatalf("scheduled retry did not run: calls=%d, err=%v", calls, err)
	}
}
