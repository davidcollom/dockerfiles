package unifi

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListCertificates(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse string
		serverStatus   int
		expectedError  string
		expectedResult []Certificate
	}{
		{
			name:           "successful response",
			serverResponse: `[{"id":"1","name":"cert1","valid_from":"2021-01-01T00:00:00Z","valid_to":"2022-01-01T00:00:00Z","active":true,"subject_alt_name":{"DNS":["example.com"]}}]`,
			serverStatus:   http.StatusOK,
			expectedError:  "",
			expectedResult: []Certificate{
				{
					ID:         "1",
					Name:       "cert1",
					ValidFrom:  parseTime("2021-01-01T00:00:00Z"),
					ValidTo:    parseTime("2022-01-01T00:00:00Z"),
					Active:     true,
					SubjectAlt: SubjectAlt{DNS: []string{"example.com"}},
				},
			},
		},
		{
			name:           "server error",
			serverResponse: `{"error":"internal server error"}`,
			serverStatus:   http.StatusInternalServerError,
			expectedError:  `unexpected status code 500: {"error":"internal server error"}`,
			expectedResult: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, client := setupTestServer(tt.serverResponse, tt.serverStatus)
			defer server.Close()

			result, err := client.ListCertificates()
			if tt.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.expectedError)
			}

			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestCreateCertificate(t *testing.T) {
	tests := []struct {
		name           string
		certName       string
		cert           string
		key            string
		serverResponse string
		serverStatus   int
		expectedError  string
	}{
		{
			name:           "successful creation",
			certName:       "cert1",
			cert:           "cert_data",
			key:            "key_data",
			serverResponse: `{"id":"1"}`,
			serverStatus:   http.StatusCreated,
			expectedError:  "",
		},
		{
			name:           "server error",
			certName:       "cert2",
			cert:           "cert_data",
			key:            "key_data",
			serverResponse: `{"error":"internal server error"}`,
			serverStatus:   http.StatusInternalServerError,
			expectedError:  "unexpected status code: 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, client := setupTestServer(tt.serverResponse, tt.serverStatus)
			defer server.Close()

			_, err := client.CreateCertificate(tt.certName, tt.cert, tt.key)
			if (err != nil && err.Error() != tt.expectedError) || (err == nil && tt.expectedError != "") {
				t.Errorf("expected error %q, got %v", tt.expectedError, err)
			}
		})
	}
}

func TestActivateCertificate(t *testing.T) {
	tests := []struct {
		name           string
		certID         string
		serverResponse string
		serverStatus   int
		expectedError  string
	}{
		{
			name:           "successful activation",
			certID:         "1",
			serverResponse: "",
			serverStatus:   http.StatusOK,
			expectedError:  "",
		},
		{
			name:           "server error",
			certID:         "2",
			serverResponse: `{"error":"internal server error"}`,
			serverStatus:   http.StatusInternalServerError,
			expectedError:  `unexpected status code 500: {"error":"internal server error"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, client := setupTestServer(tt.serverResponse, tt.serverStatus)
			defer server.Close()

			err := client.ActivateCertificate(tt.certID)
			if (err != nil && err.Error() != tt.expectedError) || (err == nil && tt.expectedError != "") {
				t.Errorf("expected error %q, got %v", tt.expectedError, err)
			}
		})
	}
}
