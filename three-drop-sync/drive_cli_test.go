package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func currentHandoff(t *testing.T) (Handoff, Snapshot) {
	t.Helper()
	h := handoffFixture()
	h.CapturedAt = time.Now().UTC().Format(time.RFC3339)
	h, s, err := ParseHandoff(encodeHandoff(t, h))
	if err != nil {
		t.Fatal(err)
	}
	return h, s
}

func TestHandoffCLIImportAndPublicationDryRun(t *testing.T) {
	h, _ := currentHandoff(t)
	input := filepath.Join(t.TempDir(), "inventory.json")
	if err := os.WriteFile(input, encodeHandoff(t, h), 0600); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "library")
	if err := run(context.Background(), []string{"handoff", "--handoff-file", input, "--output", root}); err != nil {
		t.Fatal(err)
	}
	c, err := LoadCurrentCatalogue(root)
	if err != nil || len(c.Models) != 2 || len(c.Collections) != 2 {
		t.Fatalf("catalogue: %+v %v", c, err)
	}
	if _, err := os.Stat(filepath.Join(root, "latest-handoff.json")); err != nil {
		t.Fatal(err)
	}
	cmd := newCommand()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"drive", "publish", "--handoff-file", input, "--dry-run"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if output.Len() != 0 {
		t.Fatal("dry run emitted a purported Drive file ID")
	}
	partial := false
	h.Complete = &partial
	if err := os.WriteFile(input, encodeHandoff(t, h), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run(context.Background(), []string{"drive", "publish", "--handoff-file", input, "--dry-run"}); err == nil {
		t.Fatal("partial publication accepted")
	}
	unchanged, err := LoadCurrentCatalogue(root)
	if err != nil || len(unchanged.Models) != 2 {
		t.Fatal("partial changed archive")
	}
}

func TestHandoffFreshnessAndRollback(t *testing.T) {
	h, s := currentHandoff(t)
	root := t.TempDir()
	if err := saveHandoff(root, h, s); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(root, "latest.json"))
	if err != nil {
		t.Fatal(err)
	}
	old := h
	old.CapturedAt = time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	if err := saveHandoff(root, old, s); err == nil {
		t.Fatal("rollback accepted")
	}
	after, _ := os.ReadFile(filepath.Join(root, "latest.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("rollback changed pointer")
	}
	if err := checkHandoffAge(old, 30*time.Minute, time.Now()); err == nil {
		t.Fatal("stale accepted")
	}
	if err := checkHandoffAge(old, 0, time.Now()); err != nil {
		t.Fatal(err)
	}
	h.CapturedAt = time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	if err := checkHandoffAge(h, 0, time.Now()); err == nil {
		t.Fatal("future accepted")
	}
}

func TestHandoffFailureAndCancellation(t *testing.T) {
	h, s := currentHandoff(t)
	ctx, cancel := context.WithCancel(context.Background())
	err := runHandoff(ctx, handoffOptions{archiveOptions: archiveOptions{dryRun: true, interval: time.Minute}, load: func(context.Context) (Handoff, Snapshot, error) {
		cancel()
		return h, s, errors.New("transient fetch failure")
	}})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "absent")
	err = runHandoff(context.Background(), handoffOptions{archiveOptions: archiveOptions{output: root}, load: func(context.Context) (Handoff, Snapshot, error) {
		return Handoff{}, Snapshot{}, errors.New("incomplete")
	}})
	if err == nil {
		t.Fatal("failure ignored")
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("failure wrote output")
	}
}

func TestDriveCLIRequiredOptionsAndConfig(t *testing.T) {
	if err := run(context.Background(), []string{"drive", "sync", "--dry-run"}); err == nil || !strings.Contains(err.Error(), "file-id") {
		t.Fatal(err)
	}
	h, _ := currentHandoff(t)
	input := filepath.Join(t.TempDir(), "inventory.json")
	if err := os.WriteFile(input, encodeHandoff(t, h), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("THREE_DROP_HANDOFF_FILE", input)
	if err := run(context.Background(), []string{"handoff", "--dry-run", "--download", "--workers", "2", "--max-files", "1"}); err != nil {
		t.Fatal(err)
	}
	if err := run(context.Background(), []string{"handoff", "--dry-run", "--max-manifest-age", "-1h"}); err == nil {
		t.Fatal("negative age accepted")
	}
}
