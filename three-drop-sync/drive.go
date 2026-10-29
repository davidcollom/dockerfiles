package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const driveTokenEndpoint = "https://oauth2.googleapis.com/token"

// DriveConfig selects an ordinary Drive file and one authentication
// source. Neither credentials nor a publicly shared file URL is accepted inline.
type DriveConfig struct {
	FileID          string
	FolderID        string
	Name            string
	TokenFile       string
	CredentialsFile string
}

type driveTransport struct {
	http      *http.Client
	apiURL    string
	uploadURL string
	tokenURL  string
}

func newDriveTransport() *driveTransport {
	return &driveTransport{
		http:      &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }},
		apiURL:    "https://www.googleapis.com/drive/v3/files/",
		uploadURL: "https://www.googleapis.com/upload/drive/v3/files/",
		tokenURL:  driveTokenEndpoint,
	}
}

var driveFileID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,256}$`)

// FetchDriveManifest reads JSON or CSV bytes from an existing ordinary Drive
// file. The caller validates the manifest before writing a library snapshot.
func FetchDriveManifest(ctx context.Context, cfg DriveConfig) ([]byte, error) {
	return newDriveTransport().fetch(ctx, cfg)
}

// PublishDriveManifest creates a manifest with default domain visibility disabled
// (folder permissions still apply), or replaces the contents of an
// explicitly selected existing file. It never changes sharing permissions.
func PublishDriveManifest(ctx context.Context, cfg DriveConfig, data []byte) (string, error) {
	return newDriveTransport().publish(ctx, cfg, data)
}

func validateDriveConfig(cfg DriveConfig) error {
	if cfg.FileID != "" && !driveFileID.MatchString(cfg.FileID) {
		return errors.New("Drive file ID must contain only letters, digits, underscores and hyphens")
	}
	if cfg.FolderID != "" && !driveFileID.MatchString(cfg.FolderID) {
		return errors.New("invalid Drive folder ID")
	}
	if (cfg.TokenFile == "") == (cfg.CredentialsFile == "") {
		return errors.New("configure exactly one Drive token file or OAuth credentials file")
	}
	return nil
}

func readDriveSecret(path string) ([]byte, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, errors.New("cannot open Drive credentials file")
	}
	f := os.NewFile(uintptr(fd), "Drive credentials")
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Mode().Perm()&0077 != 0 {
		return nil, errors.New("Drive credentials must be a regular private file (chmod 600)")
	}
	data, err := io.ReadAll(io.LimitReader(f, (64<<10)+1))
	if err != nil || len(data) > 64<<10 {
		return nil, errors.New("cannot read Drive credentials or file exceeds 64 KiB")
	}
	return data, nil
}

func validDriveToken(token string) bool {
	if token == "" || len(token) > 16<<10 {
		return false
	}
	for _, r := range token {
		if r < 33 || r > 126 {
			return false
		}
	}
	return true
}

func (d *driveTransport) token(ctx context.Context, cfg DriveConfig) (string, error) {
	if cfg.TokenFile != "" {
		b, err := readDriveSecret(cfg.TokenFile)
		if err != nil {
			return "", err
		}
		token := strings.TrimSpace(string(b))
		if !validDriveToken(token) {
			return "", errors.New("invalid Drive access token file")
		}
		return token, nil
	}
	data, err := readDriveSecret(cfg.CredentialsFile)
	if err != nil {
		return "", err
	}
	var credentials struct {
		Type         string `json:"type"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		RefreshToken string `json:"refresh_token"`
		TokenURI     string `json:"token_uri"`
	}
	if json.Unmarshal(data, &credentials) != nil || credentials.ClientID == "" || credentials.ClientSecret == "" || credentials.RefreshToken == "" {
		return "", errors.New("Drive credentials require client_id, client_secret and refresh_token")
	}
	if credentials.Type != "" && credentials.Type != "authorized_user" {
		return "", errors.New("Drive credentials must be authorised-user OAuth credentials")
	}
	if credentials.TokenURI != "" && credentials.TokenURI != driveTokenEndpoint {
		return "", errors.New("Drive credentials token_uri must be Google's OAuth token endpoint")
	}
	form := url.Values{"grant_type": {"refresh_token"}, "client_id": {credentials.ClientID}, "client_secret": {credentials.ClientSecret}, "refresh_token": {credentials.RefreshToken}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", errors.New("cannot create Drive token request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := d.http.Do(req)
	if err != nil {
		return "", driveRequestError(ctx, "Drive OAuth refresh")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Drive OAuth refresh returned HTTP %d; check consent and credentials", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, (64<<10)+1))
	if err != nil || len(b) > 64<<10 {
		return "", errors.New("invalid Drive OAuth response")
	}
	var token struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		RefreshToken string `json:"refresh_token"`
	}
	if json.Unmarshal(b, &token) != nil || !validDriveToken(token.AccessToken) || !strings.EqualFold(token.TokenType, "Bearer") {
		return "", errors.New("invalid Drive OAuth token response")
	}
	// Google normally keeps refresh tokens stable. Do not silently discard a
	// replacement token or rewrite a mounted secret: explicit reprovisioning is
	// required if the identity service unexpectedly rotates it.
	if token.RefreshToken != "" && token.RefreshToken != credentials.RefreshToken {
		return "", errors.New("Drive returned a rotated refresh token; re-authorise and reprovision the credentials file")
	}
	return token.AccessToken, nil
}

func driveRequestError(ctx context.Context, operation string) error {
	if ctx.Err() != nil {
		return fmt.Errorf("%s: %w", operation, ctx.Err())
	}
	return fmt.Errorf("%s request failed", operation)
}

func (d *driveTransport) fetch(ctx context.Context, cfg DriveConfig) ([]byte, error) {
	if cfg.FileID == "" {
		return nil, errors.New("Drive download requires a file ID")
	}
	if err := validateDriveConfig(cfg); err != nil {
		return nil, err
	}
	token, err := d.token(ctx, cfg)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.apiURL+cfg.FileID+"?alt=media&supportsAllDrives=true", nil)
	if err != nil {
		return nil, errors.New("cannot create Drive download request")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := d.http.Do(req)
	if err != nil {
		return nil, driveRequestError(ctx, "Drive manifest download")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Drive manifest download returned HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > maxResponse {
		return nil, errors.New("Drive manifest exceeds 32 MiB")
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil {
		return nil, driveRequestError(ctx, "Drive manifest read")
	}
	if len(b) > maxResponse {
		return nil, errors.New("Drive manifest exceeds 32 MiB")
	}
	return b, nil
}

func (d *driveTransport) publish(ctx context.Context, cfg DriveConfig, data []byte) (string, error) {
	if err := validateDriveConfig(cfg); err != nil {
		return "", err
	}
	if len(data) == 0 || len(data) > maxResponse {
		return "", errors.New("Drive manifest must contain between 1 byte and 32 MiB")
	}
	token, err := d.token(ctx, cfg)
	if err != nil {
		return "", err
	}
	method, endpoint, contentType := http.MethodPatch, d.uploadURL+cfg.FileID+"?uploadType=media&supportsAllDrives=true", "application/json"
	body := data
	if cfg.FileID == "" {
		name := cfg.Name
		if name == "" {
			name = "three-drop-catalogue.json"
		}
		if len(name) > 255 || strings.ContainsAny(name, "\r\n") {
			return "", errors.New("invalid Drive manifest name")
		}
		metadata := map[string]any{"name": name, "mimeType": "application/json"}
		if cfg.FolderID != "" {
			metadata["parents"] = []string{cfg.FolderID}
		}
		meta, _ := json.Marshal(metadata)
		var buffer bytes.Buffer
		writer := multipart.NewWriter(&buffer)
		header := textproto.MIMEHeader{}
		header.Set("Content-Type", "application/json; charset=UTF-8")
		part, err := writer.CreatePart(header)
		if err != nil {
			return "", errors.New("cannot encode Drive metadata")
		}
		_, _ = part.Write(meta)
		header = textproto.MIMEHeader{}
		header.Set("Content-Type", "application/json")
		part, err = writer.CreatePart(header)
		if err != nil {
			return "", errors.New("cannot encode Drive manifest")
		}
		_, _ = part.Write(data)
		_ = writer.Close()
		body = buffer.Bytes()
		method, endpoint, contentType = http.MethodPost, strings.TrimSuffix(d.uploadURL, "/")+"?uploadType=multipart&supportsAllDrives=true&ignoreDefaultVisibility=true", "multipart/related; boundary="+writer.Boundary()
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", errors.New("cannot create Drive upload request")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", contentType)
	resp, err := d.http.Do(req)
	if err != nil {
		return "", driveRequestError(ctx, "Drive manifest upload")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && !(cfg.FileID == "" && resp.StatusCode == http.StatusCreated) {
		return "", fmt.Errorf("Drive manifest upload returned HTTP %d", resp.StatusCode)
	}
	if cfg.FileID != "" {
		return cfg.FileID, nil
	}
	response, err := io.ReadAll(io.LimitReader(resp.Body, (64<<10)+1))
	if err != nil || len(response) > 64<<10 {
		return "", errors.New("invalid Drive upload response")
	}
	var created struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(response, &created) != nil || !driveFileID.MatchString(created.ID) {
		return "", errors.New("Drive upload response omitted a valid file ID")
	}
	return created.ID, nil
}
