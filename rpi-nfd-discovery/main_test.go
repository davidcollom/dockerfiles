package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModelID(t *testing.T) {
	cases := map[string]string{
		"Raspberry Pi 3 Model B Plus Rev 1.3": "3-bplus",
		"Raspberry Pi 4 Model B Rev 1.4":      "4-b",
		"Raspberry Pi 5 Model B Rev 1.0":      "5-b",
		"Raspberry Pi 5 Rev 1.0":              "5",
	}
	for input, want := range cases {
		if got := modelID(input); got != want {
			t.Errorf("modelID(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestReadModel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model")
	if err := os.WriteFile(path, []byte("Raspberry Pi 3 Model B Plus Rev 1.3\x00"), 0600); err != nil {
		t.Fatal(err)
	}
	model, err := readModel(path)
	if err != nil || model != "Raspberry Pi 3 Model B Plus Rev 1.3" {
		t.Fatalf("model=%q err=%v", model, err)
	}
}

func TestNotRPi(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model")
	if err := os.WriteFile(path, []byte("Other Board\x00"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readModel(path); err == nil {
		t.Fatal("expected an error")
	}
}

func TestRevision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cpuinfo")
	if err := os.WriteFile(path, []byte("Hardware : BCM2835\nRevision : a020d3\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := revision(path); got != "a020d3" {
		t.Fatalf("revision=%q", got)
	}
}

func TestAtomicOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rpi")
	content := featureContent("Raspberry Pi 3 Model B Plus Rev 1.3", "a020d3")
	if err := writeAtomic(path, content); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "rpi-model=3-bplus") {
		t.Fatalf("unexpected features: %s", got)
	}
}

func TestNodeFeatureBody(t *testing.T) {
	body, err := nodeFeatureBody("pi1", "nfd", featureLabels("Raspberry Pi 3 Model B Plus Rev 1.3", "a020d3"))
	if err != nil {
		t.Fatal(err)
	}
	var o struct {
		APIVersion, Kind string
		Metadata         struct {
			Name, Namespace string
			Labels          map[string]string
		}
		Spec struct{ Labels map[string]string }
	}
	if err := json.Unmarshal(body, &o); err != nil {
		t.Fatal(err)
	}
	if o.APIVersion != "nfd.k8s-sigs.io/v1alpha1" || o.Kind != "NodeFeature" ||
		o.Metadata.Name != "rpi-discovery-pi1" || o.Metadata.Namespace != "nfd" ||
		o.Metadata.Labels["nfd.node.kubernetes.io/node-name"] != "pi1" ||
		o.Spec.Labels["feature.node.kubernetes.io/rpi-model"] != "3-bplus" ||
		o.Spec.Labels["feature.node.kubernetes.io/rpi-revision"] != "a020d3" ||
		o.Spec.Labels["feature.node.kubernetes.io/rpi-present"] != "true" {
		t.Fatalf("unexpected object: %s", body)
	}
}

func TestApplyNodeFeature(t *testing.T) {
	var method, path, ctype, auth, got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path, ctype, auth = r.Method, r.URL.Path, r.Header.Get("Content-Type"), r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		got = string(b)
	}))
	defer srv.Close()
	if err := applyNodeFeature(srv.Client(), srv.URL, "tok", "nfd", "pi1", []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if method != "PATCH" || path != "/apis/nfd.k8s-sigs.io/v1alpha1/namespaces/nfd/nodefeatures/rpi-discovery-pi1" ||
		ctype != "application/apply-patch+yaml" || auth != "Bearer tok" || got != "{}" {
		t.Fatalf("bad request: %s %s %s %s %s", method, path, ctype, auth, got)
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(403) }))
	defer bad.Close()
	if err := applyNodeFeature(bad.Client(), bad.URL, "tok", "nfd", "pi1", nil); err == nil {
		t.Fatal("expected error on 403")
	}
}
