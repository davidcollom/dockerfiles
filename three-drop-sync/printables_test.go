package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func printablesTestPage(t *testing.T, restricted bool) string {
	t.Helper()
	scripts := []any{
		map[string]any{"id": "123", "name": "Model", "price": nil, "club": restricted, "eduProject": nil},
		map[string]any{"id": "123", "stls": []any{map[string]any{"id": "456", "name": "part.stl", "fileSize": 100}, map[string]any{"id": "457", "name": "part.stl", "fileSize": 200}}, "slas": []any{}, "otherFiles": []any{}},
	}
	page := ""
	for _, model := range scripts {
		body, _ := json.Marshal(map[string]any{"data": map[string]any{"model": model}})
		outer, _ := json.Marshal(map[string]string{"body": string(body)})
		page += `<script type="application/json" data-sveltekit-fetched data-url="https://api.printables.com/graphql/">` + string(outer) + `</script>`
	}
	return page
}

func TestDiscoverPrintablesLimitAndVariables(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method == http.MethodGet {
			w.Write([]byte(printablesTestPage(t, false)))
			return
		}
		var request struct {
			Query     string            `json:"query"`
			Variables map[string]string `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request.Variables["id"] != "456" || request.Variables["fileType"] != "stl" || request.Variables["source"] != "model_detail" {
			t.Errorf("incorrect variables: %v", request.Variables)
		}
		if r.Header.Get("Cookie") != "" {
			t.Error("provider must not receive 3Drop session")
		}
		w.Write([]byte(`{"data":{"getDownloadLink":{"ok":true,"output":{"link":"https://files.printables.com/part.stl?signature=secret","ttl":86400}}}}`))
	}))
	defer server.Close()
	result, err := DiscoverPrintables(context.Background(), []Model{{Reference: Reference{Website: "printables", ExternalID: "123"}}, {Reference: Reference{Website: "makerworld", ExternalID: "999"}}}, PrintablesOptions{MaxFiles: 1, client: server.Client(), baseURL: server.URL, apiURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(result.Files) != 1 || result.Limited != 1 || result.Unsupported != 1 || result.Files[0].Name != "456-part.stl" {
		t.Fatalf("unexpected summary: %+v calls %d", result, calls)
	}
	data, _ := json.Marshal(result)
	if strings.Contains(string(data), "secret") {
		t.Fatal("signed URLs persisted")
	}
}

func TestDiscoverPrintablesRestrictionsAndErrors(t *testing.T) {
	for _, test := range []struct {
		name       string
		status     int
		page       string
		response   string
		restricted int
		wantError  bool
	}{
		{name: "paid or club", status: 200, page: printablesTestPage(t, true), restricted: 1},
		{name: "HTTP denial", status: 403, wantError: true},
		{name: "rate limit", status: 429, wantError: true},
		{name: "schema changed", status: 200, page: "<html>login</html>", wantError: true},
		{name: "GraphQL error", status: 200, page: printablesTestPage(t, false), response: `{"errors":[{"message":"denied"}]}`, wantError: true},
		{name: "download restricted", status: 200, page: printablesTestPage(t, false), response: `{"data":{"getDownloadLink":{"ok":false,"output":null}}}`, restricted: 2},
		{name: "unexpected host", status: 200, page: printablesTestPage(t, false), response: `{"data":{"getDownloadLink":{"ok":true,"output":{"link":"https://evil.example/part.stl","ttl":100}}}}`, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.status)
				if r.Method == http.MethodGet {
					w.Write([]byte(test.page))
				} else {
					w.Write([]byte(test.response))
				}
			}))
			defer server.Close()
			result, err := DiscoverPrintables(context.Background(), []Model{{Reference: Reference{Website: "printables", ExternalID: "123"}}}, PrintablesOptions{client: server.Client(), baseURL: server.URL, apiURL: server.URL})
			if (err != nil) != test.wantError || result.Restricted != test.restricted || len(result.Files) != 0 {
				t.Fatalf("result %+v error %v", result, err)
			}
		})
	}
}

func TestDiscoverPrintablesSkipsVerifiedExistingBeforeLink(t *testing.T) {
	output := t.TempDir()
	file := DownloadFile{Website: "printables", ModelID: "123", Name: "456-part.stl"}
	if err := safeDirectory(fileDirectory(output, file)); err != nil {
		t.Fatal(err)
	}
	if _, err := storeDownload(bytes.NewReader([]byte("existing model")), DownloadOptions{Output: output, MaxFileSize: 1024}, file); err != nil {
		t.Fatal(err)
	}
	mutations := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Write([]byte(printablesTestPage(t, false)))
			return
		}
		mutations++
		var request struct {
			Variables map[string]string `json:"variables"`
		}
		json.NewDecoder(r.Body).Decode(&request)
		if request.Variables["id"] != "457" {
			t.Errorf("existing file generated link: %v", request.Variables)
		}
		w.Write([]byte(`{"data":{"getDownloadLink":{"ok":true,"output":{"link":"https://files.printables.com/new.stl","ttl":86400}}}}`))
	}))
	defer server.Close()
	result, err := DiscoverPrintables(context.Background(), []Model{{Reference: Reference{Website: "printables", ExternalID: "123"}}}, PrintablesOptions{MaxFiles: 1, Output: output, client: server.Client(), baseURL: server.URL, apiURL: server.URL})
	if err != nil || result.Existing != 1 || len(result.Files) != 1 || mutations != 1 {
		t.Fatalf("summary %+v mutations %d error %v", result, mutations, err)
	}
}

func TestDiscoverPrintablesStopsProviderOnRateLimit(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		for _, denyOn := range []int{1, 3} {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if calls == denyOn {
					w.Header().Set("Retry-After", "3600")
					w.WriteHeader(status)
					return
				}
				if r.Method == http.MethodGet {
					w.Write([]byte(printablesTestPage(t, false)))
					return
				}
				w.Write([]byte(`{"data":{"getDownloadLink":{"ok":true,"output":{"link":"https://files.printables.com/part.stl","ttl":86400}}}}`))
			}))
			result, err := DiscoverPrintables(context.Background(), []Model{{Reference: Reference{Website: "printables", ExternalID: "123"}}, {Reference: Reference{Website: "printables", ExternalID: "124"}}}, PrintablesOptions{client: server.Client(), baseURL: server.URL, apiURL: server.URL})
			server.Close()
			var unavailable *PrintablesUnavailableError
			if !errors.As(err, &unavailable) || unavailable.Status != status || calls != denyOn || result.Failed != 1 {
				t.Fatalf("status=%d denyOn=%d calls=%d result=%+v err=%v", status, denyOn, calls, result, err)
			}
			expectedFiles := 0
			if denyOn == 3 {
				expectedFiles = 1
			}
			if len(result.Files) != expectedFiles {
				t.Fatalf("partial results lost: %+v", result)
			}
		}
	}
}
