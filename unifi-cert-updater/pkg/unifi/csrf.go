package unifi

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// extractCsrfToken decodes the JWT to extract the CSRF token
func extractCsrfToken(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid JWT structure")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("failed to decode JWT payload: %v", err)
	}

	var claims struct {
		CsrfToken string `json:"csrfToken"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("failed to parse JWT payload: %v", err)
	}

	return claims.CsrfToken, nil
}
