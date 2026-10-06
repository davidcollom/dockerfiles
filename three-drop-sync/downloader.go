package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"
)

// DownloadFile describes an authorised direct file download. URLs are never archived.
type DownloadFile struct {
	URL     string `json:"url"`
	Website string `json:"website"`
	ModelID string `json:"model_id"`
	Name    string `json:"name"`
	SHA256  string `json:"sha256,omitempty"`
}

type DownloadOptions struct {
	Files       []DownloadFile
	Output      string
	Concurrency int
	MaxFiles    int   // Zero means unlimited; verified existing files do not consume this limit.
	MaxFileSize int64 // Zero defaults to 1 GiB.
	Retries     int
	client      *http.Client // Test-only injection; production always uses the protected transport.
}

type DownloadSummary struct {
	Downloaded int   `json:"downloaded"`
	Skipped    int   `json:"skipped"`
	Limited    int   `json:"limited"`
	Failed     int   `json:"failed"`
	Bytes      int64 `json:"bytes"`
}

func ReadDownloadManifest(path string) ([]DownloadFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > 16<<20 {
		return nil, errors.New("download manifest exceeds 16 MiB")
	}
	data, err := io.ReadAll(io.LimitReader(f, (16<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 16<<20 {
		return nil, errors.New("download manifest exceeds 16 MiB")
	}
	var files []DownloadFile
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&files); err != nil {
		return nil, fmt.Errorf("invalid download manifest: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return nil, errors.New("download manifest must contain exactly one JSON array")
	}
	return files, nil
}

// ValidateDownloadManifest checks public URLs, checksums and unique safe file identities.
func ValidateDownloadManifest(files []DownloadFile) error {
	seen := make(map[string]bool, len(files))
	for i, f := range files {
		if err := validateDownload(f, false); err != nil {
			return fmt.Errorf("manifest entry %d: %w", i, err)
		}
		key := fileDirectory("", f)
		if seen[key] {
			return errors.New("manifest has duplicate file identities")
		}
		seen[key] = true
	}
	return nil
}

func validComponent(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return s != "" && s != "." && s != ".." && !strings.ContainsAny(s, "/\\\x00") && len(s) <= 180
}

func validateDownload(f DownloadFile, injected bool) error {
	if !validComponent(f.Website) || !validComponent(f.ModelID) || !validComponent(f.Name) {
		return errors.New("website, model_id and name must be safe single path components")
	}
	if f.SHA256 != "" {
		b, err := hex.DecodeString(f.SHA256)
		if err != nil || len(b) != sha256.Size {
			return errors.New("invalid SHA256 checksum")
		}
	}
	u, err := url.Parse(f.URL)
	if err != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" || (u.Scheme != "https" && !injected) {
		return errors.New("download requires an HTTPS URL without user credentials or fragment")
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return errors.New("unsupported download URL scheme")
	}
	if !injected {
		if err := publicURL(u); err != nil {
			return err
		}
	}
	return nil
}

func publicURL(u *url.URL) error {
	if u.Scheme != "https" || u.User != nil || u.Hostname() == "" {
		return errors.New("redirect requires public HTTPS URL")
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return errors.New("local download destinations are forbidden")
	}
	if ip := net.ParseIP(host); ip != nil && !publicIP(ip) {
		return errors.New("non-public download destinations are forbidden")
	}
	return nil
}

func publicIP(ip net.IP) bool {
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
		return false
	}
	// CGNAT, benchmark, documentation and reserved ranges are not public destinations.
	for _, cidr := range []string{"100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32"} {
		_, block, _ := net.ParseCIDR(cidr)
		if block.Contains(ip) {
			return false
		}
	}
	return true
}

func downloadHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	tr := &http.Transport{TLSHandshakeTimeout: 15 * time.Second, ResponseHeaderTimeout: 30 * time.Second, MaxIdleConns: 32, MaxIdleConnsPerHost: 8, ForceAttemptHTTP2: true}
	tr.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, errors.New("invalid download address")
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, errors.New("download DNS lookup failed")
		}
		if len(ips) == 0 {
			return nil, errors.New("download DNS returned no addresses")
		}
		for _, ip := range ips {
			if !publicIP(ip.IP) {
				return nil, errors.New("download DNS resolved to a non-public address")
			}
		}
		var last error
		for _, ip := range ips {
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
			if err == nil {
				return conn, nil
			}
			last = err
		}
		return nil, last
	}
	return &http.Client{Transport: tr, Timeout: 30 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("too many download redirects")
		}
		return publicURL(req.URL)
	}}
}

// safeDirectory rejects symlink components, so a mounted library cannot redirect writes.
func safeDirectory(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	current := string(filepath.Separator)
	for _, part := range strings.Split(strings.TrimPrefix(abs, string(filepath.Separator)), string(filepath.Separator)) {
		current = filepath.Join(current, part)
		err := os.Mkdir(current, 0750)
		if err != nil && !os.IsExist(err) {
			return err
		}
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("download output contains a symlink or non-directory component")
		}
	}
	return nil
}

func fileDirectory(root string, f DownloadFile) string {
	return filepath.Join(root, "downloads", f.Website, f.ModelID, f.Name)
}
func checksumFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("download is not a regular file")
	}
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func verifiedExisting(dir string, f DownloadFile) bool {
	info, err := os.Lstat(filepath.Join(dir, "current.sha256"))
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	b, err := os.ReadFile(filepath.Join(dir, "current.sha256"))
	if err != nil {
		return false
	}
	hash := strings.TrimSpace(string(b))
	decoded, err := hex.DecodeString(hash)
	if err != nil || len(decoded) != 32 {
		return false
	}
	if f.SHA256 != "" && !strings.EqualFold(hash, f.SHA256) {
		return false
	}
	path := filepath.Join(dir, hash, f.Name)
	info, err = os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	actual, err := checksumFile(path)
	return err == nil && actual == hash
}

// Download archives immutable content-addressed files using a bounded worker pool.
// Existing content is verified before it is skipped. Individual failures are aggregated.
func Download(ctx context.Context, options DownloadOptions) (DownloadSummary, error) {
	var summary DownloadSummary
	if options.Output == "" {
		return summary, errors.New("download output directory is required")
	}
	if options.Concurrency == 0 {
		options.Concurrency = 4
	}
	if options.Concurrency < 1 || options.Concurrency > 128 {
		return summary, errors.New("download concurrency must be between 1 and 128")
	}
	if options.MaxFiles < 0 || options.MaxFileSize < 0 || options.Retries < 0 || options.Retries > 10 {
		return summary, errors.New("invalid download limits or retries")
	}
	if options.MaxFileSize > (1<<63)-2 {
		return summary, errors.New("maximum file size is too large")
	}
	if options.MaxFileSize == 0 {
		options.MaxFileSize = 1 << 30
	}
	files := append([]DownloadFile(nil), options.Files...)
	sort.Slice(files, func(i, j int) bool {
		a, b := files[i], files[j]
		return a.Website+"\x00"+a.ModelID+"\x00"+a.Name < b.Website+"\x00"+b.ModelID+"\x00"+b.Name
	})
	for i, f := range files {
		if err := validateDownload(f, options.client != nil); err != nil {
			return summary, fmt.Errorf("manifest entry %d: %w", i, err)
		}
		if i > 0 && fileDirectory(options.Output, files[i-1]) == fileDirectory(options.Output, f) {
			return summary, errors.New("manifest has duplicate file identities")
		}
	}
	if err := safeDirectory(options.Output); err != nil {
		return summary, err
	}
	lock, err := downloadLock(options.Output)
	if err != nil {
		return summary, err
	}
	defer lock.Close()
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)

	selected := make([]DownloadFile, 0, len(files))
	for _, f := range files {
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		dir := fileDirectory(options.Output, f)
		if err := safeDirectory(dir); err != nil {
			return summary, err
		}
		if verifiedExisting(dir, f) {
			summary.Skipped++
			continue
		}
		if options.MaxFiles > 0 && len(selected) >= options.MaxFiles {
			summary.Limited++
			continue
		}
		selected = append(selected, f)
	}
	client := options.client
	if client == nil {
		client = downloadHTTPClient()
		defer client.CloseIdleConnections()
	}
	jobs := make(chan DownloadFile)
	var mu sync.Mutex
	var wg sync.WaitGroup
	var failures []error
	for n := 0; n < options.Concurrency; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range jobs {
				bytes, err := downloadOne(ctx, client, options, f)
				mu.Lock()
				if err != nil {
					summary.Failed++
					failures = append(failures, fmt.Errorf("download %s/%s/%s: %w", f.Website, f.ModelID, f.Name, err))
				} else {
					summary.Downloaded++
					summary.Bytes += bytes
				}
				mu.Unlock()
			}
		}()
	}
send:
	for _, f := range selected {
		select {
		case jobs <- f:
		case <-ctx.Done():
			break send
		}
	}
	close(jobs)
	wg.Wait()
	if ctx.Err() != nil {
		failures = append(failures, ctx.Err())
	}
	return summary, errors.Join(failures...)
}

func retryDelay(header string, attempt int) (time.Duration, error) {
	delay := time.Duration(1<<attempt) * time.Second
	if header != "" {
		numeric := true
		for _, r := range header {
			if r < '0' || r > '9' {
				numeric = false
				break
			}
		}
		if numeric {
			seconds, err := strconv.ParseUint(header, 10, 64)
			if err != nil || seconds > 30 {
				return 0, errors.New("server Retry-After exceeds retry budget; rerun later")
			}
			delay = time.Duration(seconds) * time.Second
		} else if at, err := http.ParseTime(header); err == nil {
			delay = time.Until(at)
		}
	}
	if delay < 0 {
		delay = 0
	}
	if delay > 30*time.Second {
		return 0, errors.New("server Retry-After exceeds retry budget; rerun later")
	}
	return delay, nil
}

func downloadLock(root string) (*os.File, error) {
	fd, err := syscall.Open(filepath.Join(root, ".downloads.lock"), syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), "download lock")
	if err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, errors.New("another download process holds the output lock")
	}
	return f, nil
}
func downloadSyncDirectory(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

func downloadOne(ctx context.Context, client *http.Client, o DownloadOptions, f DownloadFile) (int64, error) {
	for attempt := 0; attempt <= o.Retries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.URL, nil)
		if err != nil {
			return 0, errors.New("cannot create download request")
		}
		req.Header.Set("User-Agent", "three-drop-sync/1")
		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return 0, ctx.Err()
			}
			if attempt == o.Retries {
				return 0, errors.New("download request failed after retries (URL withheld)")
			}
			delay, _ := retryDelay("", attempt)
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return 0, ctx.Err()
			case <-timer.C:
			}
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			delay, delayErr := retryDelay(resp.Header.Get("Retry-After"), attempt)
			resp.Body.Close()
			if delayErr != nil {
				return 0, delayErr
			}
			if attempt == o.Retries {
				return 0, fmt.Errorf("HTTP %d after retries", resp.StatusCode)
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return 0, ctx.Err()
			case <-timer.C:
			}
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return 0, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		if resp.ContentLength > o.MaxFileSize {
			resp.Body.Close()
			return 0, errors.New("file exceeds maximum file size")
		}
		n, err := storeDownload(resp.Body, o, f)
		resp.Body.Close()
		return n, err
	}
	return 0, errors.New("download retry exhausted")
}
func storeDownload(body io.Reader, o DownloadOptions, f DownloadFile) (int64, error) {
	dir := fileDirectory(o.Output, f)
	tmp, err := os.CreateTemp(dir, ".download-*")
	if err != nil {
		return 0, err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(body, o.MaxFileSize+1))
	if err != nil {
		return 0, errors.New("download body read failed")
	}
	if n > o.MaxFileSize {
		return 0, errors.New("file exceeds maximum file size")
	}
	hash := hex.EncodeToString(h.Sum(nil))
	if f.SHA256 != "" && !strings.EqualFold(hash, f.SHA256) {
		return 0, errors.New("download checksum mismatch")
	}
	if err = tmp.Chmod(0640); err != nil {
		return 0, err
	}
	if err = tmp.Sync(); err != nil {
		return 0, err
	}
	if err = tmp.Close(); err != nil {
		return 0, err
	}
	contentDir := filepath.Join(dir, hash)
	if err = safeDirectory(contentDir); err != nil {
		return 0, err
	}
	target := filepath.Join(contentDir, f.Name)
	if info, e := os.Lstat(target); e == nil {
		if !info.Mode().IsRegular() {
			return 0, errors.New("existing download path is not a regular file")
		}
		existing, e := checksumFile(target)
		if e != nil {
			return 0, e
		}
		if existing != hash {
			backup, err := os.CreateTemp(contentDir, ".corrupt-*")
			if err != nil {
				return 0, err
			}
			backup.Close()
			if err = os.Rename(target, backup.Name()); err != nil {
				return 0, err
			}
		}
	}
	if err = os.Rename(tmp.Name(), target); err != nil {
		return 0, err
	}
	if err = downloadSyncDirectory(contentDir); err != nil {
		return 0, err
	}
	sidecar, err := os.CreateTemp(dir, ".checksum-*")
	if err != nil {
		return 0, err
	}
	defer os.Remove(sidecar.Name())
	if _, err = io.WriteString(sidecar, hash+"\n"); err != nil {
		sidecar.Close()
		return 0, err
	}
	if err = sidecar.Sync(); err != nil {
		sidecar.Close()
		return 0, err
	}
	if err = sidecar.Close(); err != nil {
		return 0, err
	}
	if info, e := os.Lstat(filepath.Join(dir, "current.sha256")); e == nil && !info.Mode().IsRegular() {
		return 0, errors.New("checksum sidecar is not a regular file")
	}
	if err = os.Rename(sidecar.Name(), filepath.Join(dir, "current.sha256")); err != nil {
		return 0, err
	}
	if err = downloadSyncDirectory(dir); err != nil {
		return 0, err
	}
	return n, nil
}
