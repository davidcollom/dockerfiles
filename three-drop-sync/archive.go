package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

type Snapshot struct {
	Likes       json.RawMessage
	Collections json.RawMessage
}

type Reference struct {
	Website    string `json:"website"`
	ExternalID string `json:"externalId"`
}

type Collection struct {
	ID    string      `json:"id"`
	Name  string      `json:"name"`
	Items []Reference `json:"items"`
}

type Model struct {
	Reference
	Liked         bool     `json:"liked"`
	CollectionIDs []string `json:"collectionIds"`
}

type Catalogue struct {
	SchemaVersion int          `json:"schemaVersion"`
	Models        []Model      `json:"models"`
	Collections   []Collection `json:"collections"`
}

// LoadCurrentCatalogue resolves the immutable snapshot from a validated digest,
// verifies its raw response checksum, and reconstructs the reference catalogue.
func LoadCurrentCatalogue(root string) (Catalogue, error) {
	data, err := os.ReadFile(filepath.Join(root, "latest.json"))
	if err != nil {
		return Catalogue{}, fmt.Errorf("read current archive pointer: %w", err)
	}
	var pointer struct {
		SHA256 string `json:"sha256"`
	}
	if err := json.Unmarshal(data, &pointer); err != nil {
		return Catalogue{}, fmt.Errorf("invalid archive pointer")
	}
	digest, err := hex.DecodeString(pointer.SHA256)
	if err != nil || len(digest) != sha256.Size {
		return Catalogue{}, fmt.Errorf("invalid archive snapshot digest")
	}
	// Ignore the human-readable snapshot path in the pointer; never trust it as
	// a filesystem path or permit it to escape the archive root.
	snapshot, err := ReadImport(filepath.Join(root, "snapshots", hex.EncodeToString(digest)))
	if err != nil {
		return Catalogue{}, err
	}
	likes, err := canonical(snapshot.Likes)
	if err != nil {
		return Catalogue{}, err
	}
	collections, err := canonical(snapshot.Collections)
	if err != nil {
		return Catalogue{}, err
	}
	hash := sha256.New()
	hash.Write(likes)
	hash.Write([]byte{0})
	hash.Write(collections)
	if hex.EncodeToString(hash.Sum(nil)) != hex.EncodeToString(digest) {
		return Catalogue{}, fmt.Errorf("current archive snapshot is corrupt")
	}
	return snapshot.Catalogue()
}

func decodeObject(data []byte, field string, target any) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return fmt.Errorf("invalid account JSON object")
	}
	value, exists := object[field]
	if !exists || strings.TrimSpace(string(value)) == "null" {
		return fmt.Errorf("account response missing %q; schema may have changed", field)
	}
	if err := json.Unmarshal(value, target); err != nil {
		return fmt.Errorf("unexpected %q format; schema may have changed", field)
	}
	return nil
}

func (s Snapshot) Catalogue() (Catalogue, error) {
	var liked []string
	var collections []Collection
	if err := decodeObject(s.Likes, "liked", &liked); err != nil {
		return Catalogue{}, err
	}
	if err := decodeObject(s.Collections, "collections", &collections); err != nil {
		return Catalogue{}, err
	}
	models := make(map[Reference]*Model)
	getModel := func(ref Reference) (*Model, error) {
		if ref.Website == "" || ref.ExternalID == "" {
			return nil, fmt.Errorf("model reference has no website or external ID")
		}
		if models[ref] == nil {
			models[ref] = &Model{Reference: ref, CollectionIDs: []string{}}
		}
		return models[ref], nil
	}
	for _, key := range liked {
		website, id, ok := strings.Cut(key, ":")
		if !ok {
			return Catalogue{}, fmt.Errorf("unexpected liked model key format")
		}
		model, err := getModel(Reference{Website: website, ExternalID: id})
		if err != nil {
			return Catalogue{}, err
		}
		model.Liked = true
	}
	seen := make(map[string]bool)
	for _, collection := range collections {
		if collection.ID == "" || seen[collection.ID] || collection.Items == nil {
			return Catalogue{}, fmt.Errorf("collection ID duplicated or missing, or items absent")
		}
		seen[collection.ID] = true
		members := make(map[Reference]bool)
		for _, ref := range collection.Items {
			model, err := getModel(ref)
			if err != nil {
				return Catalogue{}, err
			}
			if !members[ref] {
				model.CollectionIDs = append(model.CollectionIDs, collection.ID)
				members[ref] = true
			}
		}
	}
	catalogue := Catalogue{SchemaVersion: 1, Models: []Model{}, Collections: collections}
	for _, model := range models {
		sort.Strings(model.CollectionIDs)
		catalogue.Models = append(catalogue.Models, *model)
	}
	sort.Slice(catalogue.Models, func(i, j int) bool {
		a, b := catalogue.Models[i], catalogue.Models[j]
		if a.Website != b.Website {
			return a.Website < b.Website
		}
		return a.ExternalID < b.ExternalID
	})
	return catalogue, nil
}

func readJSONFile(path string) (json.RawMessage, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxResponse+1))
	if err != nil || len(data) > maxResponse || !json.Valid(data) {
		return nil, fmt.Errorf("invalid or oversized JSON input")
	}
	return data, nil
}

// ReadImport accepts saved responses from the same two account GET endpoints.
// It does not claim compatibility with an unverified Excel export format.
func ReadImport(directory string) (Snapshot, error) {
	likes, err := readJSONFile(filepath.Join(directory, "likes.json"))
	if err != nil {
		return Snapshot{}, fmt.Errorf("read likes.json: %w", err)
	}
	collections, err := readJSONFile(filepath.Join(directory, "collections.json"))
	if err != nil {
		return Snapshot{}, fmt.Errorf("read collections.json: %w", err)
	}
	return Snapshot{Likes: likes, Collections: collections}, nil
}

func canonical(data []byte) ([]byte, error) {
	var value any
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return json.MarshalIndent(value, "", "  ")
}

// SaveArchive retains immutable, content-addressed snapshots. The latest pointer
// changes only after both responses and the derived catalogue are safely saved.
func SaveArchive(root string, snapshot Snapshot) (bool, error) {
	catalogue, err := snapshot.Catalogue()
	if err != nil {
		return false, err
	}
	likes, err := canonical(snapshot.Likes)
	if err != nil {
		return false, err
	}
	collections, err := canonical(snapshot.Collections)
	if err != nil {
		return false, err
	}
	index, err := json.MarshalIndent(catalogue, "", "  ")
	if err != nil {
		return false, err
	}
	hash := sha256.New()
	hash.Write(likes)
	hash.Write([]byte{0})
	hash.Write(collections)
	digest := hex.EncodeToString(hash.Sum(nil))
	if err := os.MkdirAll(filepath.Join(root, "snapshots"), 0750); err != nil {
		return false, err
	}
	lock, err := os.OpenFile(filepath.Join(root, ".sync.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return false, err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return false, fmt.Errorf("archive already in use, or filesystem does not support locking")
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) //nolint:errcheck
	latestPath := filepath.Join(root, "latest.json")
	var previous struct {
		SHA256 string `json:"sha256"`
	}
	if data, err := os.ReadFile(latestPath); err == nil {
		if err := json.Unmarshal(data, &previous); err != nil {
			return false, fmt.Errorf("latest.json is invalid; repair it before archiving")
		}
	} else if !os.IsNotExist(err) {
		return false, err
	}
	destination := filepath.Join(root, "snapshots", digest)
	files := map[string][]byte{"likes.json": likes, "collections.json": collections, "catalogue.json": index}
	if _, err := os.Stat(destination); os.IsNotExist(err) {
		staging, err := os.MkdirTemp(filepath.Join(root, "snapshots"), ".pending-")
		if err != nil {
			return false, err
		}
		defer os.RemoveAll(staging)
		if err := os.Chmod(staging, 0750); err != nil {
			return false, err
		}
		for name, content := range files {
			if err := writeAtomic(staging, name, content); err != nil {
				return false, err
			}
		}
		if err := os.Rename(staging, destination); err != nil {
			return false, err
		}
		if err := syncDirectory(filepath.Join(root, "snapshots")); err != nil {
			return false, err
		}
	} else if err != nil {
		return false, err
	} else {
		// Detect NAS corruption or removed files even when input hasn't changed.
		for name, expected := range files {
			actual, err := os.ReadFile(filepath.Join(destination, name))
			if err != nil || string(actual) != string(expected) {
				return false, fmt.Errorf("existing snapshot %s is missing or corrupt", name)
			}
		}
	}
	if previous.SHA256 == digest {
		return false, nil
	}
	pointer, err := json.MarshalIndent(struct {
		SHA256     string `json:"sha256"`
		Snapshot   string `json:"snapshot"`
		CapturedAt string `json:"capturedAt"`
	}{digest, "snapshots/" + digest, time.Now().UTC().Format(time.RFC3339)}, "", "  ")
	if err != nil {
		return false, err
	}
	if err := writeAtomic(root, "latest.json", pointer); err != nil {
		return false, err
	}
	return true, nil
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func writeAtomic(directory, name string, data []byte) error {
	file, err := os.CreateTemp(directory, ".write-")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	defer file.Close()
	if err := file.Chmod(0640); err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporary, filepath.Join(directory, name)); err != nil {
		return err
	}
	return syncDirectory(directory)
}
