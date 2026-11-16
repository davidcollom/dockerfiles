package main

import (
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
