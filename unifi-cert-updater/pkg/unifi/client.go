package unifi

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"

	"github.com/sirupsen/logrus"
)

type UniFiClient struct {
	BaseURL  string
	Username string
	Password string
	Site     string // Exported, defaults to "default"

	HTTPClient *http.Client

	token     string // Internal, unexported
	csrfToken string // Internal, unexported
	isUniFiOS bool   // Internal, unexported flag
}

// NewClient initializes a new UniFi API client
func NewClient(baseURL, username, password string, customHTTPClient *http.Client) (*UniFiClient, error) {
	var httpClient *http.Client

	// Use the provided custom HTTP client if passed
	if customHTTPClient != nil {
		if customHTTPClient.Jar == nil {
			// Ensure the custom client has a cookie jar
			jar, err := cookiejar.New(nil)
			if err != nil {
				return nil, fmt.Errorf("failed to create cookie jar for custom HTTP client: %v", err)
			}
			customHTTPClient.Jar = jar
		}
		httpClient = customHTTPClient
	} else {
		// Create a default HTTP client with a cookie jar
		jar, err := cookiejar.New(nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create default cookie jar: %v", err)
		}
		httpClient = &http.Client{Jar: jar}
	}

	return &UniFiClient{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		Username:   username,
		Password:   password,
		HTTPClient: httpClient,
	}, nil
}

func (c *UniFiClient) setToken(token string) {
	c.token = token
}

func (c *UniFiClient) setCSRFToken(csrfToken string) {
	c.csrfToken = csrfToken
}

func (c *UniFiClient) Login() error {
	logrus.Info("Attempting to log in to the UniFi API...")

	// List of potential login endpoints
	endpoints := []string{
		"/api/auth/login",      // UniFi OS
		"/api/login",           // Legacy UniFi Controller
		"/api/s/default/login", // Legacy API
	}

	var loginResponse struct {
		UniqueID    string `json:"unique_id"`
		CsrfToken   string `json:"csrfToken,omitempty"`
		AccessToken string `json:"access_token,omitempty"` // For future-proofing UniFi OS
	}

	// Iterate through endpoints and attempt login
	for _, endpoint := range endpoints {
		payload := map[string]string{
			"username": c.Username,
			"password": c.Password,
		}

		err := c.doRequest("POST", endpoint, payload, &loginResponse)
		if err != nil {
			logrus.WithError(err).Warnf("Login attempt failed for endpoint: %s", endpoint)
			continue
		}

		// Successfully logged in
		c.token = extractTokenFromCookies(c.HTTPClient.Jar, c.BaseURL)
		if loginResponse.CsrfToken != "" {
			c.csrfToken = loginResponse.CsrfToken
		}

		c.isUniFiOS = endpoint == "/api/auth/login"
		logrus.Infof("Login successful. Detected UniFi OS: %v", c.isUniFiOS)
		return nil
	}

	return fmt.Errorf("all login attempts failed")
}

// Helper to extract the TOKEN from cookies
func extractTokenFromCookies(jar http.CookieJar, baseURL string) string {
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		logrus.WithError(err).Error("Failed to parse base URL")
		return ""
	}

	for _, cookie := range jar.Cookies(parsedURL) {
		if cookie.Name == "TOKEN" {
			return cookie.Value
		}
	}
	return ""
}
