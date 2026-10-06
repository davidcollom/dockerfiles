package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"time"
)

// Printables uses the public site's observed interfaces, not a supported API.
// Download-link requests increment the provider's normal download counters.
type PrintablesOptions struct {
	MaxFiles int
	Output   string
	client   *http.Client
	baseURL  string
	apiURL   string
	delay    time.Duration
}

type PrintablesDiscovery struct {
	Files       []DownloadFile `json:"-"` // Expiring URLs must not enter persistent reports.
	Existing    int            `json:"existing"`
	Restricted  int            `json:"restricted"`
	Unsupported int            `json:"unsupported"`
	Limited     int            `json:"limited"`
	Failed      int            `json:"failed"`
}

type printablesFile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Folder   string `json:"folder"`
	FileSize int64  `json:"fileSize"`
}

type printablesModel struct {
	ID         string           `json:"id"`
	Name       string           `json:"name"`
	Price      json.RawMessage  `json:"price"`
	Club       bool             `json:"club"`
	EduProject json.RawMessage  `json:"eduProject"`
	STLs       []printablesFile `json:"stls"`
	SLAs       []printablesFile `json:"slas"`
	OtherFiles []printablesFile `json:"otherFiles"`
}

var printablesScripts = regexp.MustCompile(`(?s)<script\b[^>]*\bdata-sveltekit-fetched\b[^>]*>(.*?)</script>`)
var printablesNumericID = regexp.MustCompile(`^[0-9]+$`)

// PrintablesUnavailableError stops provider discovery for this run. No link
// mutation is retried because generating links increments download counters.
type PrintablesUnavailableError struct {
	Status int
}

func (e *PrintablesUnavailableError) Error() string {
	return fmt.Sprintf("Printables returned HTTP %d; provider discovery stopped, retry a later scheduled run", e.Status)
}

// Extract only publicly rendered response bodies. Never evaluate page JavaScript.
func parsePrintablesPage(body []byte, expectedID string) (printablesModel, error) {
	var result printablesModel
	metadata, files := false, false
	for _, match := range printablesScripts.FindAllSubmatch(body, -1) {
		var envelope struct {
			Body string `json:"body"`
		}
		if json.Unmarshal(match[1], &envelope) != nil {
			continue
		}
		var payload struct {
			Data struct {
				Model json.RawMessage `json:"model"`
			} `json:"data"`
		}
		if json.Unmarshal([]byte(envelope.Body), &payload) != nil || len(payload.Data.Model) == 0 {
			continue
		}
		var model printablesModel
		if err := json.Unmarshal(payload.Data.Model, &model); err != nil {
			return result, errors.New("Printables model schema changed")
		}
		if model.ID != expectedID {
			continue
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(payload.Data.Model, &fields) != nil {
			continue
		}
		if _, ok := fields["stls"]; ok {
			result.STLs = model.STLs
			result.SLAs = model.SLAs
			result.OtherFiles = model.OtherFiles
			files = true
		}
		if _, ok := fields["price"]; ok {
			if _, ok := fields["club"]; !ok {
				return result, errors.New("Printables access metadata missing")
			}
			if _, ok := fields["eduProject"]; !ok {
				return result, errors.New("Printables access metadata missing")
			}
			result.ID = model.ID
			result.Name = model.Name
			result.Price = model.Price
			result.Club = model.Club
			result.EduProject = model.EduProject
			metadata = true
		}
	}
	if !metadata || !files {
		return result, errors.New("Printables public page lacks model or file metadata; provider schema may have changed")
	}
	return result, nil
}

const printablesDownloadQuery = `mutation GetDownloadLink($id: ID!, $modelId: ID!, $fileType: DownloadFileTypeEnum!, $source: DownloadSourceEnum!) {
 getDownloadLink(id:$id, printId:$modelId, fileType:$fileType, source:$source) {
  ok errors { field messages } output { link count ttl }
 }
}`

func printablesRequest(ctx context.Context, client *http.Client, method, endpoint string, data []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("invalid Printables request")
	}
	req.Header.Set("User-Agent", "three-drop-sync/0.1.0 (personal collection archive)")
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://www.printables.com")
	}
	response, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("Printables request failed (network or timeout)")
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusTooManyRequests || response.StatusCode == http.StatusServiceUnavailable {
		return nil, &PrintablesUnavailableError{Status: response.StatusCode}
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Printables returned HTTP %d; no access bypass attempted", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponse+1))
	if err != nil || len(body) > maxResponse {
		return nil, errors.New("Printables response incomplete or too large")
	}
	return body, nil
}

// DiscoverPrintables resolves free public files only. Upstream logins, paid files,
// educational access and restricted downloads require separate provider support.
func DiscoverPrintables(ctx context.Context, models []Model, options PrintablesOptions) (PrintablesDiscovery, error) {
	var result PrintablesDiscovery
	if options.MaxFiles < 0 {
		return result, errors.New("max-files must not be negative")
	}
	client := options.client
	if client == nil {
		client = downloadHTTPClient()
		defer client.CloseIdleConnections()
		options.delay = time.Second
	}
	if options.baseURL == "" {
		options.baseURL = "https://www.printables.com"
	}
	if options.apiURL == "" {
		options.apiURL = "https://api.printables.com/graphql/"
	}
	models = append([]Model(nil), models...)
	sort.Slice(models, func(i, j int) bool {
		return models[i].Website+models[i].ExternalID < models[j].Website+models[j].ExternalID
	})
	seen := map[string]bool{}
	var failures []error
	requested := false
	request := func(method, endpoint string, data []byte) ([]byte, error) {
		if requested {
			if err := wait(ctx, options.delay); err != nil {
				return nil, err
			}
		}
		requested = true
		return printablesRequest(ctx, client, method, endpoint, data)
	}
	for _, reference := range models {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if reference.Website != "printables" {
			result.Unsupported++
			continue
		}
		if seen[reference.ExternalID] {
			continue
		}
		seen[reference.ExternalID] = true
		if options.MaxFiles > 0 && len(result.Files) >= options.MaxFiles {
			result.Limited++
			continue
		}
		if !printablesNumericID.MatchString(reference.ExternalID) {
			result.Failed++
			failures = append(failures, errors.New("invalid Printables model ID"))
			continue
		}
		page, err := request(http.MethodGet, options.baseURL+"/model/"+reference.ExternalID+"/files", nil)
		if err != nil {
			result.Failed++
			failures = append(failures, err)
			var unavailable *PrintablesUnavailableError
			if errors.As(err, &unavailable) {
				return result, errors.Join(failures...)
			}
			continue
		}
		model, err := parsePrintablesPage(page, reference.ExternalID)
		if err != nil {
			result.Failed++
			failures = append(failures, err)
			continue
		}
		nonNull := func(raw json.RawMessage) bool { return len(raw) > 0 && string(raw) != "null" }
		if model.Club || nonNull(model.Price) || nonNull(model.EduProject) {
			result.Restricted++
			continue
		}
		groups := []struct {
			kind  string
			files []printablesFile
		}{{"stl", model.STLs}, {"sla", model.SLAs}, {"other", model.OtherFiles}}
		identities := map[string]bool{}
		for _, group := range groups {
			for _, file := range group.files {
				if options.MaxFiles > 0 && len(result.Files) >= options.MaxFiles {
					result.Limited++
					continue
				}
				// A file ID prefix preserves nested-folder/duplicate-basename identities.
				name := file.ID + "-" + file.Name
				item := DownloadFile{Website: "printables", ModelID: reference.ExternalID, Name: name}
				if !printablesNumericID.MatchString(file.ID) || !validComponent(name) || identities[name] {
					result.Failed++
					failures = append(failures, errors.New("invalid or duplicate Printables file identity"))
					continue
				}
				identities[name] = true
				if options.Output != "" && verifiedExisting(fileDirectory(options.Output, item), item) {
					result.Existing++
					continue
				}
				payload, _ := json.Marshal(map[string]any{"operationName": "GetDownloadLink", "query": printablesDownloadQuery, "variables": map[string]string{"id": file.ID, "modelId": model.ID, "fileType": group.kind, "source": "model_detail"}})
				response, err := request(http.MethodPost, options.apiURL, payload)
				if err != nil {
					result.Failed++
					failures = append(failures, err)
					var unavailable *PrintablesUnavailableError
					if errors.As(err, &unavailable) {
						return result, errors.Join(failures...)
					}
					continue
				}
				var link struct {
					Errors []json.RawMessage `json:"errors"`
					Data   struct {
						Link struct {
							OK     bool `json:"ok"`
							Output *struct {
								Link string `json:"link"`
								TTL  int    `json:"ttl"`
							} `json:"output"`
						} `json:"getDownloadLink"`
					} `json:"data"`
				}
				if json.Unmarshal(response, &link) != nil || len(link.Errors) > 0 {
					result.Failed++
					failures = append(failures, errors.New("Printables download response schema or GraphQL error"))
					continue
				}
				if !link.Data.Link.OK || link.Data.Link.Output == nil {
					result.Restricted++
					continue
				}
				output := link.Data.Link.Output
				parsed, err := url.Parse(output.Link)
				if err != nil || parsed.Scheme != "https" || parsed.Hostname() != "files.printables.com" || parsed.User != nil || parsed.Fragment != "" || output.TTL <= 0 {
					result.Failed++
					failures = append(failures, errors.New("Printables returned invalid or unexpected file URL"))
					continue
				}
				item.URL = output.Link
				result.Files = append(result.Files, item)
			}
		}
	}
	return result, errors.Join(failures...)
}
