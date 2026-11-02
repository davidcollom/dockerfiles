package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func handoffInt(n int) *int { return &n }
func handoffFixture() Handoff {
	complete := true
	return Handoff{SchemaVersion: 1, Source: "three-drop", CapturedAt: "2026-10-06T22:00:00Z", Complete: &complete,
		Totals: HandoffTotals{Likes: handoffInt(1), Collections: handoffInt(2), SavedModels: handoffInt(2)},
		Models: []HandoffModel{
			{Reference: Reference{Website: "printables", ExternalID: "123"}, Title: "First model", SourceURL: "https://www.printables.com/model/123-first", Liked: true, CollectionIDs: []string{"a", "b"}},
			{Reference: Reference{Website: "thingiverse", ExternalID: "456"}, Title: "Collected model", SourceURL: "https://www.thingiverse.com/thing:456", CollectionIDs: []string{"b"}},
		}, Collections: []HandoffCollection{{ID: "a", Name: "Repeated name", ItemCount: handoffInt(1)}, {ID: "b", Name: "Repeated name", ItemCount: handoffInt(2)}}}
}
func encodeHandoff(t *testing.T, h Handoff) []byte {
	t.Helper()
	data, err := json.Marshal(h)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func TestParseHandoffComplete(t *testing.T) {
	h, s, err := ParseHandoff(encodeHandoff(t, handoffFixture()))
	if err != nil {
		t.Fatal(err)
	}
	catalogue, err := s.Catalogue()
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Models) != 2 || len(catalogue.Models) != 2 || !catalogue.Models[0].Liked || len(catalogue.Collections[1].Items) != 2 {
		t.Fatalf("unexpected catalogue: %+v", catalogue)
	}
}
func TestParseHandoffEmptyComplete(t *testing.T) {
	h := handoffFixture()
	h.Models = []HandoffModel{}
	h.Collections = []HandoffCollection{}
	h.Totals = HandoffTotals{handoffInt(0), handoffInt(0), handoffInt(0)}
	_, s, err := ParseHandoff(encodeHandoff(t, h))
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.Catalogue()
	if err != nil || len(c.Models) != 0 || len(c.Collections) != 0 {
		t.Fatalf("empty catalogue: %+v, %v", c, err)
	}
}
func TestParseHandoffRejectInvalid(t *testing.T) {
	tests := map[string]func(*Handoff){
		"partial":                   func(h *Handoff) { v := false; h.Complete = &v },
		"missing complete":          func(h *Handoff) { h.Complete = nil },
		"missing likes count":       func(h *Handoff) { h.Totals.Likes = nil },
		"missing collections count": func(h *Handoff) { h.Totals.Collections = nil },
		"missing model count":       func(h *Handoff) { h.Totals.SavedModels = nil },
		"missing item count":        func(h *Handoff) { h.Collections[0].ItemCount = nil },
		"wrong item count":          func(h *Handoff) { h.Collections[0].ItemCount = handoffInt(0) },
		"wrong likes count":         func(h *Handoff) { h.Totals.Likes = handoffInt(0) },
		"wrong models count":        func(h *Handoff) { h.Totals.SavedModels = handoffInt(0) },
		"duplicate reference":       func(h *Handoff) { h.Models[1].Reference = h.Models[0].Reference },
		"unknown collection":        func(h *Handoff) { h.Models[0].CollectionIDs = []string{"unknown"} },
		"duplicate membership":      func(h *Handoff) { h.Models[0].CollectionIDs = []string{"a", "a"} },
		"duplicate collection ID":   func(h *Handoff) { h.Collections[1].ID = "a" },
		"unsaved model":             func(h *Handoff) { h.Models[1].CollectionIDs = []string{} },
		"invalid timestamp":         func(h *Handoff) { h.CapturedAt = "yesterday" },
		"source":                    func(h *Handoff) { h.Source = "anything" },
		"schema":                    func(h *Handoff) { h.SchemaVersion = 2 },
		"null models":               func(h *Handoff) { h.Models = nil },
		"null collections":          func(h *Handoff) { h.Collections = nil },
		"wrong source URL ID":       func(h *Handoff) { h.Models[0].SourceURL = "https://www.printables.com/model/1234-other" },
		"wrong source host":         func(h *Handoff) { h.Models[0].SourceURL = "https://evil.example/model/123" },
		"source subdomain":          func(h *Handoff) { h.Models[0].SourceURL = "https://files.printables.com/model/123" },
		"source credentials":        func(h *Handoff) { h.Models[0].SourceURL = "https://secret@www.printables.com/model/123" },
		"source query":              func(h *Handoff) { h.Models[0].SourceURL = "https://www.printables.com/model/123?token=secret" },
		"source fragment":           func(h *Handoff) { h.Models[0].SourceURL = "https://www.printables.com/model/123#secret" },
		"source plain HTTP":         func(h *Handoff) { h.Models[0].SourceURL = "http://www.printables.com/model/123" },
		"source port":               func(h *Handoff) { h.Models[0].SourceURL = "https://www.printables.com:443/model/123" },
		"source unknown provider":   func(h *Handoff) { h.Models[0].Website = "evil" },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			h := handoffFixture()
			change(&h)
			if _, _, err := ParseHandoff(encodeHandoff(t, h)); err == nil {
				t.Fatal("invalid inventory accepted")
			}
		})
	}
}
func TestReadHandoffAndStrictJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inventory.json")
	data := encodeHandoff(t, handoffFixture())
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadHandoff(path); err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{append(data, []byte(" {}")...), []byte(`{"secret":"value"}`), make([]byte, maxHandoffSize+1)} {
		if _, _, err := ParseHandoff(data); err == nil {
			t.Fatal("invalid JSON accepted")
		}
	}
}

func TestParseHandoffRejectAmbiguousOrMissingFields(t *testing.T) {
	data := encodeHandoff(t, handoffFixture())
	inputs := [][]byte{
		bytes.Replace(data, []byte(`"complete":true`), []byte(`"complete":false,"complete":true`), 1),
		bytes.Replace(data, []byte(`"likes":1`), []byte(`"likes":2,"likes":1`), 1),
		bytes.Replace(data, []byte(`"liked":true,`), nil, 1),
		bytes.Replace(data, []byte(`"likes":1,`), nil, 1),
	}
	for _, input := range inputs {
		if _, _, err := ParseHandoff(input); err == nil {
			t.Fatal("ambiguous or missing field accepted")
		}
	}
}
