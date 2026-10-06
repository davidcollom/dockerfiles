package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

const maxHandoffSize = 32 << 20

// Handoff is a complete browser-exported reference inventory, never a list of
// download URLs or credentials. Pointers distinguish an absent count from zero.
type Handoff struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Source        string              `json:"source"`
	CapturedAt    string              `json:"capturedAt"`
	Complete      *bool               `json:"complete"`
	Totals        HandoffTotals       `json:"totals"`
	Models        []HandoffModel      `json:"models"`
	Collections   []HandoffCollection `json:"collections"`
}
type HandoffTotals struct {
	Likes       *int `json:"likes"`
	Collections *int `json:"collections"`
	SavedModels *int `json:"savedModels"`
}
type HandoffModel struct {
	Reference
	Title         string   `json:"title"`
	SourceURL     string   `json:"sourceUrl"`
	Liked         bool     `json:"liked"`
	CollectionIDs []string `json:"collectionIds"`
}
type HandoffCollection struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ItemCount *int   `json:"itemCount"`
}

// ReadHandoff bounds input before validating it; errors never include source
// URLs, titles or other untrusted manifest content.
func ReadHandoff(path string) (Handoff, Snapshot, error) {
	f, err := os.Open(path)
	if err != nil {
		return Handoff{}, Snapshot{}, fmt.Errorf("open handoff: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxHandoffSize+1))
	if err != nil {
		return Handoff{}, Snapshot{}, fmt.Errorf("read handoff: %w", err)
	}
	return ParseHandoff(data)
}

func ParseHandoff(data []byte) (Handoff, Snapshot, error) {
	var h Handoff
	invalid := func(reason string) (Handoff, Snapshot, error) {
		return Handoff{}, Snapshot{}, fmt.Errorf("invalid handoff: %s", reason)
	}
	if len(data) > maxHandoffSize {
		return invalid("size exceeds 32 MiB")
	}
	if err := rejectDuplicateHandoffKeys(data); err != nil {
		return invalid("duplicate object keys or invalid JSON")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&h) != nil {
		return invalid("schema or JSON format")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return invalid("trailing JSON content")
	}
	if h.SchemaVersion != 1 || h.Source != "three-drop" {
		return invalid("unsupported schema version or source")
	}
	if _, err := time.Parse(time.RFC3339, h.CapturedAt); err != nil {
		return invalid("capturedAt must be RFC3339")
	}
	if h.Complete == nil || !*h.Complete {
		return invalid("inventory is incomplete")
	}
	var presence struct {
		Models []map[string]json.RawMessage `json:"models"`
	}
	if json.Unmarshal(data, &presence) != nil {
		return invalid("invalid models")
	}
	for _, model := range presence.Models {
		for _, field := range []string{"website", "externalId", "title", "sourceUrl", "liked", "collectionIds"} {
			value, ok := model[field]
			if !ok || string(value) == "null" {
				return invalid("model fields are required")
			}
		}
	}
	if h.Models == nil || h.Collections == nil {
		return invalid("models and collections arrays are required")
	}
	if h.Totals.Likes == nil || h.Totals.Collections == nil || h.Totals.SavedModels == nil {
		return invalid("all totals are required")
	}
	if *h.Totals.Likes < 0 || *h.Totals.Collections != len(h.Collections) || *h.Totals.SavedModels != len(h.Models) {
		return invalid("totals do not match inventory")
	}
	collections := make([]Collection, 0, len(h.Collections))
	indices := make(map[string]int, len(h.Collections))
	for _, c := range h.Collections {
		if !validHandoffText(c.ID, 512) || !validHandoffText(c.Name, 2048) || c.ItemCount == nil || *c.ItemCount < 0 {
			return invalid("collection ID, name or itemCount missing or invalid")
		}
		if _, exists := indices[c.ID]; exists {
			return invalid("duplicate collection ID")
		}
		indices[c.ID] = len(collections)
		collections = append(collections, Collection{ID: c.ID, Name: c.Name, Items: []Reference{}})
	}
	liked := []string{}
	seen := make(map[Reference]bool, len(h.Models))
	for _, m := range h.Models {
		if !validHandoffText(m.Website, 64) || !validHandoffText(m.ExternalID, 512) || strings.Contains(m.ExternalID, ":") || !validHandoffText(m.Title, 4096) || m.CollectionIDs == nil {
			return invalid("model fields missing or invalid")
		}
		if seen[m.Reference] {
			return invalid("duplicate model reference")
		}
		seen[m.Reference] = true
		if !validHandoffSourceURL(m) {
			return invalid("model source URL does not match its provider")
		}
		if m.Liked {
			liked = append(liked, m.Website+":"+m.ExternalID)
		}
		memberships := map[string]bool{}
		for _, id := range m.CollectionIDs {
			index, exists := indices[id]
			if !exists || memberships[id] {
				return invalid("unknown or duplicate collection membership")
			}
			memberships[id] = true
			collections[index].Items = append(collections[index].Items, m.Reference)
		}
		if !m.Liked && len(m.CollectionIDs) == 0 {
			return invalid("model is neither liked nor collected")
		}
	}
	if *h.Totals.Likes != len(liked) {
		return invalid("like total does not match inventory")
	}
	for i, c := range h.Collections {
		if *c.ItemCount != len(collections[i].Items) {
			return invalid("collection itemCount does not match membership")
		}
	}
	likesJSON, err := json.Marshal(struct {
		Liked []string `json:"liked"`
	}{liked})
	if err != nil {
		return invalid("cannot encode likes")
	}
	collectionsJSON, err := json.Marshal(struct {
		Collections []Collection `json:"collections"`
	}{collections})
	if err != nil {
		return invalid("cannot encode collections")
	}
	snapshot := Snapshot{Likes: likesJSON, Collections: collectionsJSON}
	if _, err := snapshot.Catalogue(); err != nil {
		return invalid("cannot construct catalogue")
	}
	return h, snapshot, nil
}

func validHandoffText(s string, max int) bool {
	return len(s) > 0 && len(s) <= max && strings.TrimSpace(s) == s && !strings.ContainsAny(s, "\x00\r\n")
}

var handoffNumericID = regexp.MustCompile(`^[0-9]+$`)

func validHandoffSourceURL(m HandoffModel) bool {
	u, err := url.Parse(m.SourceURL)
	if err != nil || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Port() != "" || u.RawPath != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	hosts := map[string]string{
		"printables": "printables.com", "thingiverse": "thingiverse.com", "cults": "cults3d.com", "makerworld": "makerworld.com", "thangs": "thangs.com", "makeronline": "makeronline.com", "crealitycloud": "crealitycloud.com", "grabcad": "grabcad.com", "myminifactory": "myminifactory.com",
	}
	expected, ok := hosts[m.Website]
	if !ok || (host != expected && host != "www."+expected) {
		return false
	}
	path := u.Path
	// Numeric-ID providers expose the identifier directly in their model URL.
	// Slug-based providers do not necessarily expose the internal numeric ID;
	// their reference is retained for future adapters without inventing a match.
	switch m.Website {
	case "printables", "makerworld":
		if !handoffNumericID.MatchString(m.ExternalID) {
			return false
		}
		marker := "/model/"
		if m.Website == "makerworld" {
			marker = "/models/"
		}
		_, tail, ok := strings.Cut(path, marker)
		return ok && (tail == m.ExternalID || strings.HasPrefix(tail, m.ExternalID+"-") || strings.HasPrefix(tail, m.ExternalID+"/"))
	case "thingiverse":
		return handoffNumericID.MatchString(m.ExternalID) && strings.TrimSuffix(path, "/") == "/thing:"+m.ExternalID
	case "cults":
		return strings.Contains(path, "/3d-model/") && !strings.HasSuffix(path, "/3d-model/")
	case "thangs":
		return strings.Contains(path, "/model/") && strings.HasSuffix(strings.TrimSuffix(path, "/"), "-"+m.ExternalID) && handoffNumericID.MatchString(m.ExternalID)
	case "grabcad":
		return strings.HasPrefix(path, "/library/") && len(path) > len("/library/")
	case "myminifactory":
		return strings.Contains(path, "/object/") && len(path) > len("/object/")
	default:
		return path != "" && path != "/" && !strings.Contains(path, "..")
	}
}

// Reject ambiguous JSON objects before decoding values. In particular a second
// complete or count field must never conceal a partial export.
func rejectDuplicateHandoffKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value func(int) error
	value = func(depth int) error {
		if depth > 64 {
			return fmt.Errorf("too deeply nested")
		}
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delim, container := token.(json.Delim)
		if !container {
			return nil
		}
		if delim == '{' {
			seen := map[string]bool{}
			for decoder.More() {
				key, err := decoder.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok || seen[name] {
					return fmt.Errorf("duplicate key")
				}
				seen[name] = true
				if err := value(depth + 1); err != nil {
					return err
				}
			}
		} else if delim == '[' {
			for decoder.More() {
				if err := value(depth + 1); err != nil {
					return err
				}
			}
		} else {
			return fmt.Errorf("unexpected delimiter")
		}
		_, err = decoder.Token()
		return err
	}
	if err := value(0); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return fmt.Errorf("trailing data")
	}
	return nil
}
