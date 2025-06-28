package main

import (
	"crypto/sha1"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
)

// Helper function to convert a byte slice to colon-separated hexadecimal
func formatColonSeparated(data []byte) []string {
	formatted := make([]string, len(data))
	for i, b := range data {
		formatted[i] = fmt.Sprintf("%02X", b)
	}
	return formatted
}

// calculateFingerprint generates a SHA256 fingerprint of a PEM-encoded certificate.
// The output is formatted as colon-separated hexadecimal (e.g., "58:95:C6:EA:...").
func calculateFingerprint(certPEM string) (string, error) {
	// Decode the PEM block
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM block")
	}

	// Parse the certificate
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Compute SHA256 hash of the raw certificate
	hash := sha1.Sum(cert.Raw)

	// Format the hash as colon-separated hexadecimal
	fingerprint := strings.ToUpper(strings.Join(formatColonSeparated(hash[:]), ":"))

	return fingerprint, nil
}
