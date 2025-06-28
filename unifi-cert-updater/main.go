package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"

	"github.com/davidcollom/dockerfiles/unifi-cert-updater/pkg/unifi"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Config holds application configuration
type Config struct {
	UniFiAPIURL string
	Username    string
	Password    string
	Namespace   string
	SecretName  string
	LogLevel    string
	MaxCerts    int
}

var logger *logrus.Logger

func init() {
	// load environment variables
	if _, err := os.Stat(".env"); err == nil {
		if err := godotenv.Load(); err != nil {
			logger.Warn("Error loading .env file, using environment variables instead")
		}
	}

	// Initialize logrus
	logger = logrus.New()
	setupLogger(logger)

}

func main() {
	// Load configuration from environment variables
	config := Config{
		UniFiAPIURL: os.Getenv("UNIFI_API_URL"),
		Username:    os.Getenv("UNIFI_USERNAME"),
		Password:    os.Getenv("UNIFI_PASSWORD"),
		Namespace:   os.Getenv("NAMESPACE"),
		SecretName:  os.Getenv("SECRET_NAME"),
	}

	if config.MaxCerts, _ = strconv.Atoi(os.Getenv("MAX_CERTS")); config.MaxCerts == 0 {
		config.MaxCerts = 5
	}

	// Validate required environment variables
	missingEnvVars := validateEnvVars(config)
	if len(missingEnvVars) > 0 {
		logger.Errorf("Missing required environment variables: %s", missingEnvVars)
		os.Exit(1)
	}

	logger.Info("Environment variables validated successfully.")

	// Initialize retryablehttp client
	retryClient := retryablehttp.NewClient()
	retryClient.CheckRetry = func(ctx context.Context, resp *http.Response, err error) (bool, error) {
		if resp.StatusCode == http.StatusNotFound {
			return true, nil
		}
		return retryablehttp.DefaultRetryPolicy(ctx, resp, err)
	}
	retryClient.RetryMax = 5
	retryClient.Logger = logger
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	retryClient.HTTPClient.Transport = tr

	// Initialize UniFi client
	unifiClient, err := unifi.NewClient(config.UniFiAPIURL, config.Username, config.Password, retryClient.StandardClient())
	if err != nil {
		logger.Errorf("Error creating UniFi client: %v", err)
		os.Exit(1)
	}

	// Log in to UniFi
	logger.Debug("Logging in to UniFi...")
	if err := unifiClient.Login(); err != nil {
		logger.Errorf("Login failed: %v", err)
		os.Exit(1)
	}
	logger.Info("Login successful.")

	// Initialize Kubernetes client
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))

	k8sClient, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		logger.Errorf("Error creating Kubernetes client: %v", err)
		os.Exit(1)
	}

	// Fetch certificate and key from Kubernetes secret
	logger.Debugf("Fetching certificate from namespace '%s', secret '%s'", config.Namespace, config.SecretName)
	cert, key, err := fetchCertAndKeyFromSecret(context.Background(), k8sClient, config.Namespace, config.SecretName, logger)
	if err != nil {
		logger.Errorf("Error fetching certificate and key: %v", err)
		os.Exit(1)
	}

	// Check existing certificates and upload only if fingerprint differs
	logger.Debug("Checking existing certificates and uploading if necessary.")
	newCertID, err := checkAndUploadCertificate(unifiClient, cert, key, logger)
	if err != nil {
		logger.Errorf("Error during certificate upload: %v", err)
		os.Exit(1)
	}

	// Activate the new certificate if not already active
	err = ensureCertificateActivated(unifiClient, newCertID, logger)
	if err != nil {
		logger.Fatalf("Error ensuring certificate activation: %v", err)
	}

	// Enforce the maximum certificate limit
	err = enforceCertificateLimit(unifiClient, config.MaxCerts)
	if err != nil {
		logrus.Fatalf("Error enforcing certificate limit: %v", err)
	}

	logrus.Info("Certificate successfully managed.")
}

func setupLogger(logger *logrus.Logger) {
	// Default log level
	logLevel := os.Getenv("LOG_LEVEL")
	switch logLevel {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
	}

	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
}

func validateEnvVars(config Config) []string {
	missingEnvVars := []string{}
	if config.UniFiAPIURL == "" {
		missingEnvVars = append(missingEnvVars, "UNIFI_API_URL")
	}
	if config.Username == "" {
		missingEnvVars = append(missingEnvVars, "UNIFI_USERNAME")
	}
	if config.Password == "" {
		missingEnvVars = append(missingEnvVars, "UNIFI_PASSWORD")
	}
	if config.Namespace == "" {
		missingEnvVars = append(missingEnvVars, "NAMESPACE")
	}
	if config.SecretName == "" {
		missingEnvVars = append(missingEnvVars, "SECRET_NAME")
	}
	return missingEnvVars
}

func fetchCertAndKeyFromSecret(ctx context.Context, k8sClient client.Client, namespace, secretName string, logger *logrus.Logger) (string, string, error) {
	var secret corev1.Secret
	secretKey := client.ObjectKey{Namespace: namespace, Name: secretName}
	if err := k8sClient.Get(ctx, secretKey, &secret); err != nil {
		logger.Errorf("Failed to fetch Kubernetes secret: %v", err)
		return "", "", fmt.Errorf("failed to fetch secret: %v", err)
	}

	cert, certOk := secret.Data["tls.crt"]
	key, keyOk := secret.Data["tls.key"]
	if !certOk || !keyOk {
		err := fmt.Errorf("secret is missing tls.crt or tls.key")
		logger.Errorf("Invalid secret data: %v", err)
		return "", "", err
	}

	logger.Infof("Certificate and key fetched successfully from namespace '%s', secret '%s'", namespace, secretName)
	return string(cert), string(key), nil
}

func ensureCertificateActivated(client *unifi.UniFiClient, certID string, logger *logrus.Logger) error {
	logger.Infof("Ensuring certificate with ID %s is active...", certID)

	// Fetch the list of certificates to find the active one
	existingCerts, err := client.ListCertificates()
	if err != nil {
		return fmt.Errorf("failed to list certificates: %w", err)
	}

	for _, cert := range existingCerts {
		if cert.Active {
			logger.Infof("Certificate with ID %s is already active.", cert.ID)
			if cert.ID == certID {
				// The uploaded certificate is already active
				return nil
			}
			logger.Warnf("A different certificate with ID %s is active.", cert.ID)
		}
	}

	// Activate the certificate if it's not active
	logger.Infof("Activating certificate with ID %s.", certID)
	if err := client.ActivateCertificate(certID); err != nil {
		return fmt.Errorf("failed to activate certificate with ID %s: %w", certID, err)
	}

	logger.Infof("Certificate with ID %s activated successfully.", certID)
	return nil
}

func checkAndUploadCertificate(client *unifi.UniFiClient, cert, key string, logger *logrus.Logger) (string, error) {
	// Get existing certificates
	existingCerts, err := client.ListCertificates()
	if err != nil {
		return "", fmt.Errorf("failed to list existing certificates: %v", err)
	}
	logger.Infof("Existing certificates fetched successfully: %v.", len(existingCerts))

	// Calculate the fingerprint of the new certificate
	newFingerprint, err := calculateFingerprint(cert)
	if err != nil {
		return "", fmt.Errorf("failed to calculate certificate fingerprint: %v", err)
	}
	logger.Infof("New certificate fingerprint: %s", newFingerprint)

	// Check if the certificate already exists
	for _, existingCert := range existingCerts {
		logger.Debugf("Checking certificate with fingerprint: %s", existingCert.Fingerprint)
		if existingCert.Fingerprint == newFingerprint {
			logger.Info("Certificate with the same fingerprint already exists. No action required.")
			return existingCert.ID, nil
		}
	}
	logger.Info("No matching certificate found. Uploading new certificate...")

	// Upload the new certificate
	certObj, err := client.CreateCertificate(newFingerprint, cert, key)
	if err != nil {
		logger.Errorf("Failed to upload certificate: %v", err)
		return "", fmt.Errorf("failed to upload certificate: %v", err)
	}

	logger.Info("Certificate uploaded successfully.")
	return certObj.ID, nil
}

func enforceCertificateLimit(client *unifi.UniFiClient, maxCertificates int) error {
	logrus.Infof("Enforcing certificate limit of %d...", maxCertificates)

	// Fetch all certificates
	certificates, err := client.ListCertificates()
	if err != nil {
		return fmt.Errorf("failed to list certificates: %w", err)
	}

	// Find the active certificate ID
	var activeCertID string
	for _, cert := range certificates {
		if cert.Active {
			activeCertID = cert.ID
			break
		}
	}

	// Sort certificates by `ValidFrom` (oldest first)
	sort.Slice(certificates, func(i, j int) bool {
		return certificates[i].ValidFrom.Before(certificates[j].ValidFrom)
	})

	// Identify certificates to delete
	excessCount := len(certificates) - maxCertificates
	if excessCount <= 0 {
		logrus.WithFields(logrus.Fields{
			"current_count": len(certificates),
			"max_count":     maxCertificates,
		}).Info("No excess certificates to delete.")
		return nil
	}

	logrus.Warnf("Deleting %d excess certificates to enforce limit of %d", excessCount, maxCertificates)

	// Delete oldest certificates, skipping the active one
	for _, cert := range certificates {
		if excessCount == 0 {
			break
		}

		if cert.ID == activeCertID {
			logrus.Infof("Skipping active certificate with ID %s", cert.ID)
			continue
		}

		err := client.DeleteCertificate(cert.ID)
		if err != nil {
			logrus.WithError(err).Warnf("Failed to delete certificate with ID %s", cert.ID)
		} else {
			logrus.Infof("Deleted certificate with ID %s", cert.ID)
			excessCount--
		}
	}

	return nil
}
