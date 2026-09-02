package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestCollect(t *testing.T) {
	tokenSeen := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer access-token" {
			tokenSeen = true
		}
		name := r.URL.Path[len(r.URL.Path)-len("rangeliquid"):]
		if r.URL.Path[len(r.URL.Path)-3:] == "odo" {
			name = "odo"
		}
		if r.URL.Path[len(r.URL.Path)-16:] == "tanklevelpercent" {
			name = "tanklevelpercent"
		}
		_, _ = w.Write([]byte(`{"` + name + `":{"value":42,"timestamp":1700000000}}`))
	}))
	defer server.Close()
	registry := prometheus.NewRegistry()
	e := newExporter(config{apiURL: server.URL, vin: "VIN"}, server.Client(), fake.NewSimpleClientset(), registry, testLogger())
	e.setToken("access-token")
	if err := e.collect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !tokenSeen {
		t.Fatal("Mercedes API request did not include the access token")
	}
	if got := testutil.ToFloat64(e.odoValue); got != 42 {
		t.Fatalf("odo value = %v, want 42", got)
	}
	if got := testutil.ToFloat64(e.up); got != 1 {
		t.Fatalf("up = %v, want 1", got)
	}
}

func TestRefreshToken(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "client" || password != "secret" {
			t.Error("invalid basic authentication")
		}
		if err := r.ParseForm(); err != nil || r.Form.Get("refresh_token") != "old token+value" {
			t.Errorf("refresh token was not form encoded: %q", r.Form.Get("refresh_token"))
		}
		_, _ = w.Write([]byte(`{"access_token":"access","refresh_token":"new-refresh"}`))
	}))
	defer tokenServer.Close()
	kube := fake.NewSimpleClientset(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "credentials", Namespace: "monitoring"}, Data: map[string][]byte{"TOKEN": []byte("old token+value"), "CLIENT_ID": []byte("client"), "CLIENT_SECRET": []byte("secret")}})
	registry := prometheus.NewRegistry()
	e := newExporter(config{secret: "credentials", namespace: "monitoring", tokenURL: tokenServer.URL}, tokenServer.Client(), kube, registry, testLogger())
	if err := e.refreshToken(context.Background()); err != nil {
		t.Fatal(err)
	}
	if e.token() != "access" {
		t.Fatalf("access token = %q, want access", e.token())
	}
	secret, err := kube.CoreV1().Secrets("monitoring").Get(context.Background(), "credentials", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(secret.Data["TOKEN"]); got != "new-refresh" {
		t.Fatalf("stored refresh token = %q, want new-refresh", got)
	}
}

func TestCollectWithoutTokenMarksExporterDown(t *testing.T) {
	e := newExporter(config{}, http.DefaultClient, fake.NewSimpleClientset(), prometheus.NewRegistry(), testLogger())
	if err := e.collect(context.Background()); err == nil {
		t.Fatal("collect succeeded without an access token")
	}
	if got := testutil.ToFloat64(e.up); got != 0 {
		t.Fatalf("up = %v, want 0", got)
	}
}
