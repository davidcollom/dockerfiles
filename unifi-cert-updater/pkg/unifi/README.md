# Go UniFi API Client

This is a Go client library for interacting with the UniFi Controller/UniFi OS API. The library provides methods to manage UniFi devices, certificates, sites, and more. It supports modern UniFi OS endpoints and includes features for logging in, managing certificates, and retrieving data.

---

## Features

- Authenticate with UniFi OS or legacy UniFi Network APIs.
- Manage certificates (upload, list, activate, delete).
- Query UniFi sites, devices, and statistics.
- Flexible HTTP client support (e.g., `retryablehttp`).
- `logrus` integration for structured logging.
- Written in idiomatic Go for performance and maintainability.

---

## Installation

Install the library using `go get`:

```bash
go get github.com/yourusername/unifi-api-client
```

## Usage

### Basic Example

```go
package main

import (
  "log"
  "os"

  "github.com/yourusername/unifi-api-client/pkg/unifi"
)

func main() {
  // Load environment variables
  baseURL := os.Getenv("UNIFI_API_URL")
  username := os.Getenv("UNIFI_USERNAME")
  password := os.Getenv("UNIFI_PASSWORD")

  // Create a UniFi client
  client, err := unifi.NewClient(baseURL, username, password)
  if err != nil {
    log.Fatalf("Failed to create UniFi client: %v", err)
  }

  // Log in to UniFi
  if err := client.Login(); err != nil {
    log.Fatalf("Failed to login: %v", err)
  }

  // List certificates
  certs, err := client.ListCertificates()
  if err != nil {
    log.Fatalf("Failed to list certificates: %v", err)
  }

  for _, cert := range certs {
    log.Printf("Certificate: %s, Fingerprint: %s", cert.Name, cert.Fingerprint)
  }
}
```

### Advanced Example: Managing Certificates

### Upload a New Certificate

```go
cert := `-----BEGIN CERTIFICATE-----
...
-----END CERTIFICATE-----`

key := `-----BEGIN RSA PRIVATE KEY-----
...
-----END RSA PRIVATE KEY-----`

err := client.CreateCertificate("MyNewCert", cert, key)
if err != nil {
  log.Fatalf("Failed to upload certificate: %v", err)
}
log.Println("Certificate uploaded successfully!")
```

#### Activate a Certificate

```go
certID := "b4a8a55e-d850-46fb-9a90-bf1bc72decc2"

err := client.ActivateCertificate(certID)
if err != nil {
  log.Fatalf("Failed to activate certificate: %v", err)
}
log.Println("Certificate activated successfully!")
```

### Delete a Certificate

```go
certID := "b4a8a55e-d850-46fb-9a90-bf1bc72decc2"

err := client.DeleteCertificate(certID)
if err != nil {
  log.Fatalf("Failed to delete certificate: %v", err)
}
log.Println("Certificate deleted successfully!")
```

## Environment Variables

Set the following environment variables to simplify authentication:

|Variable | Description|
|----|----|
|UNIFI_API_URL | Base URL for the UniFi API|
|UNIFI_USERNAME | Username for UniFi API login|
|UNIFI_PASSWORD | Password for UniFi API login|

## Logging

This client uses logrus for structured logging. By default, it logs at the Info level. Adjust the logging level via an environment variable:

```sh
export LOG_LEVEL=debug
```

## Testing

Run unit tests using:

```sh
go test ./pkg/unifi/...
```

## Contribution

Contributions are welcome! Open issues or submit pull requests to improve functionality or documentation.

## License

This project is licensed under the MIT License. See the LICENSE file for details.

Let me know if you'd like further adjustments or additional sections! 🚀
