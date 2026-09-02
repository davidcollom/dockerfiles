package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/robfig/cron/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultAPIURL = "https://api.mercedes-benz.com/vehicledata/v2"
	tokenURL      = "https://id.mercedes-benz.com/as/token.oauth2"
)

type config struct {
	listen, interval, vin, secret, namespace, apiURL, tokenURL string
	port                                                       int
	debug                                                      bool
}

type exporter struct {
	config config
	client *http.Client
	kube   kubernetes.Interface
	log    *slog.Logger

	mu          sync.RWMutex
	accessToken string

	odoValue, odoTimestamp         prometheus.Gauge
	fuelPercentage, fuelTimestamp  prometheus.Gauge
	rangeRemaining, rangeTimestamp prometheus.Gauge
	tokensRefreshed                prometheus.Counter
	up                             prometheus.Gauge
}

type resourceValue struct {
	Value     float64 `json:"value"`
	Timestamp float64 `json:"timestamp"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func getenv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(getenv(key, strconv.Itoa(fallback)))
	if err != nil {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value, err := strconv.ParseBool(getenv(key, strconv.FormatBool(fallback)))
	if err != nil {
		return fallback
	}
	return value
}

func parseConfig() config {
	cfg := config{}
	flag.StringVar(&cfg.listen, "listen", getenv("EXPORTER_LISTEN", "0.0.0.0"), "listen address")
	flag.StringVar(&cfg.listen, "l", getenv("EXPORTER_LISTEN", "0.0.0.0"), "listen address (shorthand)")
	flag.IntVar(&cfg.port, "port", envInt("EXPORTER_PORT", 9353), "listen port")
	flag.IntVar(&cfg.port, "p", envInt("EXPORTER_PORT", 9353), "listen port (shorthand)")
	flag.StringVar(&cfg.interval, "interval", getenv("EXPORTER_INTERVAL", "*/20 * * * *"), "metric collection cron expression")
	flag.StringVar(&cfg.interval, "i", getenv("EXPORTER_INTERVAL", "*/20 * * * *"), "metric collection cron expression (shorthand)")
	flag.StringVar(&cfg.vin, "vin", getenv("EXPORTER_VIN", ""), "vehicle identification number")
	flag.StringVar(&cfg.vin, "v", getenv("EXPORTER_VIN", ""), "vehicle identification number (shorthand)")
	flag.StringVar(&cfg.secret, "secret", getenv("EXPORTER_SECRET", "mercedesme"), "Kubernetes secret name")
	flag.StringVar(&cfg.secret, "t", getenv("EXPORTER_SECRET", "mercedesme"), "Kubernetes secret name (shorthand)")
	flag.StringVar(&cfg.namespace, "namespace", getenv("NAMESPACE", "monitoring"), "Kubernetes secret namespace")
	flag.StringVar(&cfg.namespace, "n", getenv("NAMESPACE", "monitoring"), "Kubernetes secret namespace (shorthand)")
	flag.BoolVar(&cfg.debug, "debug", envBool("EXPORTER_DEBUG", false), "enable debug logging")
	flag.BoolVar(&cfg.debug, "d", envBool("EXPORTER_DEBUG", false), "enable debug logging (shorthand)")
	flag.Parse()
	cfg.apiURL, cfg.tokenURL = defaultAPIURL, tokenURL
	return cfg
}

func newExporter(cfg config, client *http.Client, kube kubernetes.Interface, registry prometheus.Registerer, logger *slog.Logger) *exporter {
	newGauge := func(name, help string) prometheus.Gauge {
		g := prometheus.NewGauge(prometheus.GaugeOpts{Name: name, Help: help})
		registry.MustRegister(g)
		return g
	}
	e := &exporter{config: cfg, client: client, kube: kube, log: logger}
	e.odoValue = newGauge("mercedes_me_odo_value", "Odometer value in kilometres.")
	e.odoTimestamp = newGauge("mercedes_me_odo_timestamp", "Unix timestamp of the last odometer update.")
	e.fuelPercentage = newGauge("mercedes_me_fuel_percentage", "Fuel tank percentage full.")
	e.fuelTimestamp = newGauge("mercedes_me_fuel_timestamp", "Unix timestamp of the last fuel tank update.")
	e.rangeRemaining = newGauge("mercedes_me_range_remaining", "Remaining liquid-fuel range in kilometres.")
	e.rangeTimestamp = newGauge("mercedes_me_range_remaining_timestamp", "Unix timestamp of the last range update.")
	e.tokensRefreshed = prometheus.NewCounter(prometheus.CounterOpts{Name: "mercedes_me_tokens_refreshed", Help: "Number of successful authentication token refreshes."})
	registry.MustRegister(e.tokensRefreshed)
	e.up = newGauge("mercedes_me_exporter_up", "Whether the most recent Mercedes API collection succeeded.")
	return e
}

func (e *exporter) token() string         { e.mu.RLock(); defer e.mu.RUnlock(); return e.accessToken }
func (e *exporter) setToken(token string) { e.mu.Lock(); defer e.mu.Unlock(); e.accessToken = token }

func (e *exporter) fetchResource(ctx context.Context, name string) (resourceValue, error) {
	url := fmt.Sprintf("%s/vehicles/%s/resources/%s", strings.TrimRight(e.config.apiURL, "/"), e.config.vin, name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return resourceValue{}, err
	}
	req.Header.Set("Accept", "application/json;charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+e.token())
	resp, err := e.client.Do(req)
	if err != nil {
		return resourceValue{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resourceValue{}, fmt.Errorf("Mercedes API returned %s", resp.Status)
	}
	var payload map[string]resourceValue
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return resourceValue{}, err
	}
	value, ok := payload[name]
	if !ok {
		return resourceValue{}, fmt.Errorf("response did not contain %q", name)
	}
	return value, nil
}

func (e *exporter) collect(ctx context.Context) error {
	if e.token() == "" {
		e.up.Set(0)
		return errors.New("access token is not available")
	}
	type target struct {
		name             string
		value, timestamp prometheus.Gauge
	}
	targets := []target{{"odo", e.odoValue, e.odoTimestamp}, {"tanklevelpercent", e.fuelPercentage, e.fuelTimestamp}, {"rangeliquid", e.rangeRemaining, e.rangeTimestamp}}
	for _, target := range targets {
		value, err := e.fetchResource(ctx, target.name)
		if err != nil {
			e.up.Set(0)
			return fmt.Errorf("fetch %s: %w", target.name, err)
		}
		target.value.Set(value.Value)
		target.timestamp.Set(value.Timestamp)
		e.log.Debug("updated resource", "resource", target.name, "value", value.Value)
	}
	e.up.Set(1)
	return nil
}

func (e *exporter) refreshToken(ctx context.Context) error {
	secret, err := e.kube.CoreV1().Secrets(e.config.namespace).Get(ctx, e.config.secret, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("read Kubernetes secret: %w", err)
	}
	refresh, clientID, clientSecret := string(secret.Data["TOKEN"]), string(secret.Data["CLIENT_ID"]), string(secret.Data["CLIENT_SECRET"])
	if refresh == "" || clientID == "" || clientSecret == "" {
		return errors.New("secret must contain TOKEN, CLIENT_ID, and CLIENT_SECRET")
	}
	body := strings.NewReader(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}}.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.config.tokenURL, body)
	if err != nil {
		return err
	}
	req.SetBasicAuth(clientID, clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("refresh token: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("token endpoint returned %s", resp.Status)
	}
	var token tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return fmt.Errorf("decode token response: %w", err)
	}
	if token.AccessToken == "" || token.RefreshToken == "" {
		return errors.New("token response omitted access_token or refresh_token")
	}
	patch, _ := json.Marshal(map[string]any{"data": map[string]string{"TOKEN": base64.StdEncoding.EncodeToString([]byte(token.RefreshToken))}})
	if _, err := e.kube.CoreV1().Secrets(e.config.namespace).Patch(ctx, e.config.secret, types.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
		return fmt.Errorf("patch Kubernetes secret: %w", err)
	}
	e.setToken(token.AccessToken)
	e.tokensRefreshed.Inc()
	return nil
}

func kubeClient() (kubernetes.Interface, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		rules := clientcmd.NewDefaultClientConfigLoadingRules()
		cfg, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{}).ClientConfig()
	}
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}

func main() {
	cfg := parseConfig()
	level := slog.LevelInfo
	if cfg.debug {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	if cfg.vin == "" {
		logger.Error("VIN is required", "flag", "--vin", "environment", "EXPORTER_VIN")
		os.Exit(2)
	}
	kube, err := kubeClient()
	if err != nil {
		logger.Error("configure Kubernetes client", "error", err)
		os.Exit(1)
	}
	registry := prometheus.NewRegistry()
	e := newExporter(cfg, &http.Client{Timeout: 30 * time.Second}, kube, registry, logger)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	run := func(name string, fn func(context.Context) error) func() {
		return func() {
			if err := fn(ctx); err != nil {
				logger.Error(name+" failed", "error", err)
			}
		}
	}
	if err := e.refreshToken(ctx); err != nil {
		logger.Error("initial token refresh failed", "error", err)
	} else if err := e.collect(ctx); err != nil {
		logger.Error("initial collection failed", "error", err)
	}
	scheduler := cron.New()
	if _, err := scheduler.AddFunc("0 * * * *", run("token refresh", e.refreshToken)); err != nil {
		logger.Error("schedule token refresh", "error", err)
		os.Exit(1)
	}
	if _, err := scheduler.AddFunc(cfg.interval, run("metric collection", e.collect)); err != nil {
		logger.Error("invalid collection interval", "interval", cfg.interval, "error", err)
		os.Exit(2)
	}
	scheduler.Start()
	defer scheduler.Stop()
	server := &http.Server{Addr: fmt.Sprintf("%s:%d", cfg.listen, cfg.port), Handler: promhttp.HandlerFor(registry, promhttp.HandlerOpts{}), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	logger.Info("exporter listening", "address", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("metrics server failed", "error", err)
		os.Exit(1)
	}
}
