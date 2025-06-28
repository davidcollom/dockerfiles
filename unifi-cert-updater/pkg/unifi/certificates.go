package unifi

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

func (c *UniFiClient) ListCertificates() ([]Certificate, error) {
	if !c.isUniFiOS {
		return nil, fmt.Errorf("ListCertificates is only supported on UniFi OS systems")
	}

	var certificates []Certificate
	err := c.doRequest("GET", EndpointListCertificates, nil, &certificates)
	if err != nil {
		return nil, fmt.Errorf("failed to list certificates: %w", err)
	}
	return certificates, nil
}

// CreateCertificate uploads a new certificate
func (c *UniFiClient) CreateCertificate(name, cert, key string) (*Certificate, error) {
	if !c.isUniFiOS {
		return nil, fmt.Errorf("CreateCertificate is only supported on UniFi OS systems")
	}

	payload := map[string]string{
		"name": name,
		"cert": cert,
		"key":  key,
	}

	var createdCert Certificate
	err := c.doRequest("POST", EndpointCreateCertificate, payload, &createdCert)

	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	logrus.Infof("Certificate '%s' successfully created", createdCert.ID)
	return &createdCert, nil
}

func (c *UniFiClient) ActivateCertificate(certID string) error {
	if !c.isUniFiOS {
		return fmt.Errorf("ActivateCertificate is only supported on UniFi OS systems")
	}

	endpoint := fmt.Sprintf(EndpointActivateCertificate, certID)
	payload := map[string]bool{"active": true}

	err := c.doRequest("PUT", endpoint, payload, nil)
	if err != nil {
		return fmt.Errorf("failed to activate certificate with ID %s: %w", certID, err)
	}

	logrus.Infof("Certificate with ID %s successfully activated", certID)
	return nil
}
func (c *UniFiClient) DeleteCertificate(certID string) error {
	if !c.isUniFiOS {
		return fmt.Errorf("DeleteCertificate is only supported on UniFi OS systems")
	}

	endpoint := fmt.Sprintf(EndpointDeleteCertificate, certID)

	err := c.doRequest("DELETE", endpoint, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to delete certificate with ID %s: %w", certID, err)
	}

	logrus.Infof("Certificate with ID %s successfully deleted", certID)
	return nil
}
