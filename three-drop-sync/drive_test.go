package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func driveSecret(t *testing.T, data string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "credentials")
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func testDrive(t *testing.T, handler http.HandlerFunc) *driveTransport {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	d := newDriveTransport()
	d.apiURL = server.URL + "/files/"
	d.uploadURL = server.URL + "/upload/files/"
	d.tokenURL = server.URL + "/token"
	return d
}

func TestDriveFetchAndPublish(t *testing.T) {
	cfg := DriveConfig{FileID: "abc_123", TokenFile: driveSecret(t, "access-token\n")}
	d := testDrive(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer access-token" {
			t.Error("missing bearer")
		}
		if r.Method == http.MethodGet {
			if r.URL.Path != "/files/abc_123" || r.URL.Query().Get("alt") != "media" {
				t.Error("wrong GET")
			}
			_, _ = io.WriteString(w, `{"schema_version":1}`)
		} else {
			if r.Method != http.MethodPatch || r.URL.Query().Get("uploadType") != "media" {
				t.Error("wrong PATCH")
			}
			data, _ := io.ReadAll(r.Body)
			if string(data) != "manifest" {
				t.Error("wrong upload")
			}
		}
	})
	got, err := d.fetch(context.Background(), cfg)
	if err != nil || string(got) != `{"schema_version":1}` {
		t.Fatalf("%s %v", got, err)
	}
	id, err := d.publish(context.Background(), cfg, []byte("manifest"))
	if err != nil || id != cfg.FileID {
		t.Fatalf("%s %v", id, err)
	}
}

func TestDriveCreate(t *testing.T) {
	cfg := DriveConfig{FolderID: "folder", TokenFile: driveSecret(t, "token")}
	d := testDrive(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/upload/files" || r.URL.Query().Get("uploadType") != "multipart" {
			t.Error("wrong POST")
		}
		typ, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || typ != "multipart/related" {
			t.Fatal("wrong multipart type")
		}
		reader := multipart.NewReader(r.Body, params["boundary"])
		part, err := reader.NextPart()
		if err != nil {
			t.Fatal(err)
		}
		var metadata struct {
			Name    string
			Parents []string
		}
		if json.NewDecoder(part).Decode(&metadata) != nil || metadata.Name != "three-drop-catalogue.json" || len(metadata.Parents) != 1 || metadata.Parents[0] != "folder" {
			t.Error("wrong metadata")
		}
		part, err = reader.NextPart()
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(part)
		if string(data) != "{}" {
			t.Error("wrong content")
		}
		_, _ = io.WriteString(w, `{"id":"created-ID"}`)
	})
	id, err := d.publish(context.Background(), cfg, []byte("{}"))
	if err != nil || id != "created-ID" {
		t.Fatalf("%s %v", id, err)
	}
}

func TestDriveRefresh(t *testing.T) {
	cfg := DriveConfig{FileID: "file", CredentialsFile: driveSecret(t, `{"type":"authorized_user","client_id":"client","client_secret":"secret","refresh_token":"refresh"}`)}
	calls := 0
	d := testDrive(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path == "/token" {
			if r.ParseForm() != nil || r.Form.Get("refresh_token") != "refresh" || r.Form.Get("client_secret") != "secret" || r.Form.Get("grant_type") != "refresh_token" {
				t.Error("wrong refresh form")
			}
			_, _ = io.WriteString(w, `{"access_token":"renewed","token_type":"Bearer"}`)
		} else {
			if r.Header.Get("Authorization") != "Bearer renewed" {
				t.Error("wrong refreshed token")
			}
			_, _ = io.WriteString(w, "[]")
		}
	})
	if _, err := d.fetch(context.Background(), cfg); err != nil || calls != 2 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestDriveFailuresRedactAndDoNotRedirect(t *testing.T) {
	for _, code := range []int{401, 403, 404, 429, 500, 302} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			cfg := DriveConfig{FileID: "file", TokenFile: driveSecret(t, "secret-token")}
			calls := 0
			d := testDrive(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Location", "/other")
				w.WriteHeader(code)
				_, _ = io.WriteString(w, "secret-token secret-server-body")
			})
			_, err := d.fetch(context.Background(), cfg)
			if err == nil || strings.Contains(err.Error(), "secret-") || calls != 1 {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
		})
	}
}

func TestDriveRefreshFailures(t *testing.T) {
	for _, body := range []string{`{}`, `{"access_token":"secret-token","token_type":"wrong"}`, `{"access_token":"secret-token","token_type":"Bearer","refresh_token":"rotated-secret"}`, strings.Repeat("x", (64<<10)+1)} {
		cfg := DriveConfig{FileID: "file", CredentialsFile: driveSecret(t, `{"client_id":"client","client_secret":"secret","refresh_token":"refresh"}`)}
		d := testDrive(t, func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, body) })
		_, err := d.fetch(context.Background(), cfg)
		if err == nil || strings.Contains(err.Error(), "secret-token") || strings.Contains(err.Error(), "rotated-secret") {
			t.Fatalf("%v", err)
		}
	}
	cfg := DriveConfig{FileID: "file", CredentialsFile: driveSecret(t, `{"client_id":"client","client_secret":"secret","refresh_token":"refresh"}`)}
	d := testDrive(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(400); _, _ = io.WriteString(w, "secret") })
	if _, err := d.fetch(context.Background(), cfg); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("%v", err)
	}
}

func TestDriveValidationAndPrivateFiles(t *testing.T) {
	path := driveSecret(t, "token")
	for _, cfg := range []DriveConfig{{FileID: "../bad", TokenFile: path}, {FileID: "file"}, {FileID: "file", TokenFile: path, CredentialsFile: path}, {FileID: "file", FolderID: "bad/path", TokenFile: path}} {
		if validateDriveConfig(cfg) == nil {
			t.Fatalf("accepted %#v", cfg)
		}
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := readDriveSecret(path); err == nil {
		t.Fatal("accepted public credentials")
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := readDriveSecret(link); err == nil {
		t.Fatal("accepted symlink")
	}
	bad := driveSecret(t, `{"client_id":"id","client_secret":"secret","refresh_token":"refresh","token_uri":"https://evil.example/token"}`)
	if _, err := newDriveTransport().token(context.Background(), DriveConfig{CredentialsFile: bad}); err == nil {
		t.Fatal("accepted token endpoint")
	}
}

func TestDriveBoundsAndCancellation(t *testing.T) {
	cfg := DriveConfig{FileID: "file", TokenFile: driveSecret(t, "token")}
	for _, declared := range []bool{false, true} {
		d := testDrive(t, func(w http.ResponseWriter, r *http.Request) {
			if declared {
				w.Header().Set("Content-Length", "33554433")
			}
			_, _ = io.WriteString(w, strings.Repeat("x", maxResponse+1))
		})
		if _, err := d.fetch(context.Background(), cfg); err == nil {
			t.Fatal("accepted large manifest")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d := newDriveTransport()
	if _, err := d.fetch(ctx, cfg); !errors.Is(err, context.Canceled) {
		t.Fatalf("%v", err)
	}
	if _, err := d.publish(context.Background(), cfg, make([]byte, maxResponse+1)); err == nil {
		t.Fatal("accepted large upload")
	}
}

func TestDrivePublishFailureAndInvalidCreateResponse(t *testing.T) {
	for _, body := range []string{`{}`, `{"id":"bad/id"}`, `{"id":"private-secret"}`} {
		d := testDrive(t, func(w http.ResponseWriter, r *http.Request) {
			if body == `{"id":"private-secret"}` {
				w.WriteHeader(http.StatusForbidden)
			}
			_, _ = io.WriteString(w, body)
		})
		cfg := DriveConfig{TokenFile: driveSecret(t, "access-secret")}
		if _, err := d.publish(context.Background(), cfg, []byte("{}")); err == nil || strings.Contains(err.Error(), "private-secret") {
			t.Fatalf("%v", err)
		}
	}
	cfg := DriveConfig{CredentialsFile: driveSecret(t, `{"client_id":"id","client_secret":"secret","refresh_token":"refresh"}`), FileID: "file"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := newDriveTransport().fetch(ctx, cfg); !errors.Is(err, context.Canceled) {
		t.Fatalf("%v", err)
	}
}
