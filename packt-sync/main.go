package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"
)

const defaultBaseURL = "https://services.packtpub.com"

type config struct {
	username, password, downloadPath, baseURL string
	extensions                                []string
	concurrency                               int
}

type client struct {
	baseURL, token string
	httpClient     *http.Client
	log            *slog.Logger
}

type book struct {
	ProductName string `json:"productName"`
	ProductID   string `json:"productId"`
}

type downloadJob struct {
	book      book
	extension string
	path      string
}

func getenv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func loadConfig() (config, error) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return config{}, err
	}
	cfg := config{
		username:     os.Getenv("USERNAME"),
		password:     os.Getenv("PASSWORD"),
		downloadPath: getenv("DOWNLOAD_PATH", workingDirectory),
		baseURL:      strings.TrimRight(getenv("PACKT_BASE_URL", defaultBaseURL), "/"),
		concurrency:  4,
	}
	for _, extension := range strings.Split(getenv("EXTENSIONS", "epub,mobi,pdf"), ",") {
		extension = strings.TrimSpace(strings.TrimPrefix(extension, "."))
		if extension != "" && isSafeExtension(extension) {
			cfg.extensions = append(cfg.extensions, extension)
		} else if extension != "" {
			return config{}, fmt.Errorf("invalid extension %q", extension)
		}
	}
	if value := os.Getenv("DOWNLOAD_CONCURRENCY"); value != "" {
		cfg.concurrency, err = strconv.Atoi(value)
		if err != nil || cfg.concurrency < 1 {
			return config{}, errors.New("DOWNLOAD_CONCURRENCY must be a positive integer")
		}
	}
	if cfg.username == "" || cfg.password == "" {
		return config{}, errors.New("USERNAME and PASSWORD are required")
	}
	if len(cfg.extensions) == 0 {
		return config{}, errors.New("EXTENSIONS must contain at least one format")
	}
	return cfg, nil
}

func isSafeExtension(extension string) bool {
	for _, character := range extension {
		if !unicode.IsLetter(character) && !unicode.IsDigit(character) {
			return false
		}
	}
	return extension != ""
}

func (c *client) request(ctx context.Context, method, endpoint string, body []byte, authenticated bool) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		if len(body) > 0 {
			req.Header.Set("Content-Type", "application/json")
		}
		if authenticated {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
		response, err := c.httpClient.Do(req)
		if err == nil && response.StatusCode < 500 && response.StatusCode != http.StatusTooManyRequests {
			return response, nil
		}
		if response != nil {
			_ = response.Body.Close()
			lastErr = fmt.Errorf("server returned %s", response.Status)
		} else {
			lastErr = err
		}
		if err := sleepContext(ctx, time.Duration(attempt+1)*time.Second); err != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func checkResponse(response *http.Response) error {
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}
	message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	return fmt.Errorf("Packt API returned %s: %s", response.Status, strings.TrimSpace(string(message)))
}

func (c *client) authenticate(ctx context.Context, username, password string) error {
	payload, err := json.Marshal(map[string]string{"username": username, "password": password})
	if err != nil {
		return err
	}
	response, err := c.request(ctx, http.MethodPost, "/auth-v1/users/tokens", payload, false)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if err := checkResponse(response); err != nil {
		return err
	}
	var result struct {
		Data struct {
			Access string `json:"access"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode authentication response: %w", err)
	}
	if result.Data.Access == "" {
		return errors.New("authentication response did not contain an access token")
	}
	c.token = result.Data.Access
	return nil
}

func (c *client) books(ctx context.Context) ([]book, error) {
	const pageSize = 20
	var books []book
	for offset := 0; ; {
		endpoint := fmt.Sprintf("/entitlements-v1/users/me/products?sort=createdAt%%3ADESC&limit=%d&offset=%d", pageSize, offset)
		response, err := c.request(ctx, http.MethodGet, endpoint, nil, true)
		if err != nil {
			return nil, err
		}
		if err := checkResponse(response); err != nil {
			response.Body.Close()
			return nil, err
		}
		var page struct {
			Count int    `json:"count"`
			Data  []book `json:"data"`
		}
		err = json.NewDecoder(response.Body).Decode(&page)
		response.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("decode products response: %w", err)
		}
		books = append(books, page.Data...)
		c.log.Debug("received product page", "offset", offset, "products", len(page.Data), "total", page.Count)
		if len(page.Data) == 0 || len(books) >= page.Count {
			return books, nil
		}
		offset += len(page.Data)
	}
}

func safeBookName(name string) string {
	name = strings.TrimSpace(name)
	if strings.HasSuffix(strings.ToLower(name), "[ebook]") {
		name = strings.TrimSpace(name[:len(name)-len("[ebook]")])
	}
	name = strings.Map(func(character rune) rune {
		switch {
		case character == '/' || character == '\\':
			return '_'
		case unicode.IsLetter(character), unicode.IsDigit(character), character == ' ', character == '-', character == '.', character == '_':
			return character
		default:
			return -1
		}
	}, name)
	name = strings.Trim(name, " .")
	if name == "" || name == "." || name == ".." {
		return "untitled"
	}
	return name
}

func (c *client) downloadURL(ctx context.Context, productID, extension string) (string, error) {
	endpoint := fmt.Sprintf("/products-v1/products/%s/files/%s", url.PathEscape(productID), url.PathEscape(extension))
	response, err := c.request(ctx, http.MethodGet, endpoint, nil, true)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if err := checkResponse(response); err != nil {
		return "", err
	}
	var result struct {
		Data string `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode file response: %w", err)
	}
	parsed, err := url.Parse(result.Data)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("file response did not contain an HTTP download URL")
	}
	return result.Data, nil
}

func (c *client) download(ctx context.Context, job downloadJob) error {
	fileURL, err := c.downloadURL(ctx, job.book.ProductID, job.extension)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return err
	}
	response, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if err := checkResponse(response); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(job.path), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(job.path), ".packt-download-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if _, err := io.Copy(temporary, response.Body); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Chmod(temporaryName, 0o644); err != nil {
		return err
	}
	return os.Rename(temporaryName, job.path)
}

func syncBooks(ctx context.Context, cfg config, api *client, books []book) error {
	jobs := make(chan downloadJob)
	errorsChannel := make(chan error, cfg.concurrency)
	var workers sync.WaitGroup
	for range cfg.concurrency {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for job := range jobs {
				api.log.Info("downloading", "book", job.book.ProductName, "format", job.extension)
				if err := api.download(ctx, job); err != nil {
					select {
					case errorsChannel <- fmt.Errorf("download %s as %s: %w", job.book.ProductName, job.extension, err):
					case <-ctx.Done():
					}
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, item := range books {
			name := safeBookName(item.ProductName)
			for _, extension := range cfg.extensions {
				path := filepath.Join(cfg.downloadPath, name, name+"."+extension)
				if _, err := os.Stat(path); err == nil {
					api.log.Debug("already downloaded", "path", path)
					continue
				} else if !errors.Is(err, os.ErrNotExist) {
					errorsChannel <- fmt.Errorf("inspect %s: %w", path, err)
					continue
				}
				select {
				case jobs <- downloadJob{book: item, extension: extension, path: path}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	done := make(chan struct{})
	go func() { workers.Wait(); close(errorsChannel); close(done) }()
	var failures []error
	for err := range errorsChannel {
		failures = append(failures, err)
	}
	<-done
	return errors.Join(failures...)
}

func main() {
	level := slog.LevelInfo
	if strings.EqualFold(os.Getenv("LOG_LEVEL"), "DEBUG") {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	cfg, err := loadConfig()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	api := &client{baseURL: cfg.baseURL, httpClient: &http.Client{Timeout: 10 * time.Minute}, log: logger}
	logger.Info("authenticating", "username", cfg.username)
	if err := api.authenticate(ctx, cfg.username, cfg.password); err != nil {
		logger.Error("authentication failed", "error", err)
		os.Exit(1)
	}
	books, err := api.books(ctx)
	if err != nil {
		logger.Error("list books", "error", err)
		os.Exit(1)
	}
	logger.Info("synchronising library", "books", len(books), "formats", cfg.extensions, "concurrency", cfg.concurrency, "destination", cfg.downloadPath)
	if err := syncBooks(ctx, cfg, api, books); err != nil {
		logger.Error("library sync completed with errors", "error", err)
		os.Exit(1)
	}
	logger.Info("library is in sync")
}
