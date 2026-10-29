package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const maxResponse = 32 << 20

// These GET endpoints and response keys were observed in the site's public
// frontend on 6 October 2026. They are not a supported public API contract.
const likesEndpoint = "/api/favorites/likes/check-batch"
const collectionsEndpoint = "/api/favorites/collections/item-refs"

type Client struct {
	baseURL string
	cookie  string
	http    *http.Client
	delay   time.Duration
}

func NewClient(cookie string) *Client {
	return &Client{
		baseURL: "https://three-drop.com",
		cookie:  cookie,
		delay:   time.Second,
		http: &http.Client{
			Timeout: 45 * time.Second,
			// Never forward a session secret to an unexpected redirect destination.
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func ReadCookie(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open session file: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (16<<10)+1))
	if err != nil {
		return "", fmt.Errorf("read session file: %w", err)
	}
	value := strings.TrimSpace(string(data))
	if len(data) > 16<<10 || value == "" || !strings.Contains(value, "=") || strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("session file must contain one Cookie header value, maximum 16 KiB")
	}
	return value, nil
}

func wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *Client) get(ctx context.Context, endpoint string) (json.RawMessage, error) {
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("create account request")
		}
		req.Header.Set("Cookie", c.cookie)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "three-drop-sync/0.1.0 (personal collection archive)")
		response, err := c.http.Do(req)
		if err != nil {
			// Do not print URLs, request headers, session values or server bodies.
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("account request failed (network or timeout)")
		}
		status := response.StatusCode
		if status == http.StatusUnauthorized || status == http.StatusForbidden {
			response.Body.Close()
			return nil, fmt.Errorf("3Drop denied account access (HTTP %d); renew your local session file", status)
		}
		if status == http.StatusTooManyRequests || status >= 500 {
			response.Body.Close()
			if attempt == 2 {
				return nil, fmt.Errorf("account endpoint unavailable after retries (HTTP %d)", status)
			}
			delay := time.Duration(1<<attempt) * time.Second
			if header := response.Header.Get("Retry-After"); header != "" {
				if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
					if seconds > 60 {
						return nil, fmt.Errorf("3Drop requested a long retry delay; retry a later scheduled run")
					}
					delay = time.Duration(seconds) * time.Second
				} else if until, err := http.ParseTime(header); err == nil {
					delay = max(time.Until(until), delay)
				}
			}
			if delay > time.Minute {
				return nil, fmt.Errorf("3Drop requested a long retry delay; retry a later scheduled run")
			}
			if err := wait(ctx, delay); err != nil {
				return nil, err
			}
			continue
		}
		if status != http.StatusOK {
			response.Body.Close()
			return nil, fmt.Errorf("unexpected account response (HTTP %d)", status)
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, maxResponse+1))
		response.Body.Close()
		if readErr != nil || len(body) > maxResponse {
			return nil, fmt.Errorf("account response incomplete or larger than 32 MiB")
		}
		if !json.Valid(body) {
			return nil, fmt.Errorf("account response is not JSON; archive was not updated")
		}
		return body, nil
	}
	return nil, fmt.Errorf("account request failed")
}

func (c *Client) Fetch(ctx context.Context) (Snapshot, error) {
	likes, err := c.get(ctx, likesEndpoint)
	if err != nil {
		return Snapshot{}, err
	}
	if err := wait(ctx, c.delay); err != nil {
		return Snapshot{}, err
	}
	collections, err := c.get(ctx, collectionsEndpoint)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot := Snapshot{Likes: likes, Collections: collections}
	_, err = snapshot.Catalogue()
	return snapshot, err
}
