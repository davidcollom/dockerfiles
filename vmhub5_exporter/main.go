package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	host     string
	interval time.Duration
	logger   logr.Logger
)

func scheduleMetricsRefresh(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fetchAndUpdateMetrics()
		}
	}
}

func fetchAndUpdateMetrics() {
	logger.Info("Refreshing metrics from APIs")

	// Fetch provisioning data
	provisioningURL := fmt.Sprintf("http://%s/rest/v1/system/gateway/provisioning", host)
	var provRes ProvisioningResponse
	if err := fetchJSON(provisioningURL, &provRes); err == nil {
		ipv4Lease.Set(provRes.Provisioning.IPv4.LeaseTime)
		logger.V(1).Info("Setting metric", "provisioning_ipv4_lease_time", provRes.Provisioning.IPv4.LeaseTime)

		ipv4Expire.Set(provRes.Provisioning.IPv4.ExpireTime)
		logger.V(1).Info("Setting metric", "provisioning_ipv4_expire_time", provRes.Provisioning.IPv4.ExpireTime)

		ipv6Lease.Set(provRes.Provisioning.IPv6.LeaseTime)
		logger.V(1).Info("Setting metric", "provisioning_ipv6_lease_time", provRes.Provisioning.IPv6.LeaseTime)

		ipv6Expire.Set(provRes.Provisioning.IPv6.ExpireTime)
		logger.V(1).Info("Setting metric", "provisioning_ipv6_expire_time", provRes.Provisioning.IPv6.ExpireTime)

		if provRes.Provisioning.DSLite.Enable {
			dsLiteEnable.Set(1)
		} else {
			dsLiteEnable.Set(0)
		}
		logger.V(1).Info("Setting metric", "provisioning_dsLite_enable", provRes.Provisioning.DSLite.Enable)
	}

	// Fetch PON state
	ponStateURL := fmt.Sprintf("http://%s/rest/v1/pon/state", host)
	var ponStateRes PonStateResponse
	if err := fetchJSON(ponStateURL, &ponStateRes); err == nil {
		ponUpTime.Set(ponStateRes.Pon.UpTime)
		logger.V(1).Info("Setting metric", "pon_status_up_time", ponStateRes.Pon.UpTime)

		ponAccess.Set(boolToFloat(ponStateRes.Pon.AccessAllowed))
		logger.V(1).Info("Setting metric", "pon_status_access_allowed", ponStateRes.Pon.AccessAllowed)
	}

	// Fetch PON status
	ponStatusURL := fmt.Sprintf("http://%s/rest/v1/pon/status", host)
	var ponStatusRes PonStatusResponse
	if err := fetchJSON(ponStatusURL, &ponStatusRes); err == nil {
		ponFirewall.Set(boolToFloat(ponStatusRes.Status.Firewall))
		logger.V(1).Info("Setting metric", "pon_status_firewall", ponStatusRes.Status.Firewall)

		ponIPData.Set(boolToFloat(ponStatusRes.Status.IPData == "Up"))
		logger.V(1).Info("Setting metric", "pon_status_ip_data", ponStatusRes.Status.IPData)

		ponIPVoip.Set(boolToFloat(ponStatusRes.Status.IPVoip == "Up"))
		logger.V(1).Info("Setting metric", "pon_status_ip_voip", ponStatusRes.Status.IPVoip)

		transceiverTemp.Set(parseStringToFloat(ponStatusRes.Interface.TransceiverTemp))
		logger.V(1).Info("Setting metric", "pon_interface_transceiver_temp", ponStatusRes.Interface.TransceiverTemp)

		transceiverVoltage.Set(parseStringToFloat(ponStatusRes.Interface.TransceiverVoltage))
		logger.V(1).Info("Setting metric", "pon_interface_transceiver_voltage", ponStatusRes.Interface.TransceiverVoltage)

		laserBiasCurrent.Set(parseStringToFloat(ponStatusRes.Interface.LaserBiasCurrent))
		logger.V(1).Info("Setting metric", "pon_interface_laser_bias_current", ponStatusRes.Interface.LaserBiasCurrent)

		transmitOpticalLevel.Set(parseStringToFloat(ponStatusRes.Interface.TransmitOpticalLevel))
		logger.V(1).Info("Setting metric", "pon_interface_transmit_optical_level", ponStatusRes.Interface.TransmitOpticalLevel)

		opticalSignalLevel.Set(parseStringToFloat(ponStatusRes.Interface.OpticalSignalLevel))
		logger.V(1).Info("Setting metric", "pon_interface_optical_signal_level", ponStatusRes.Interface.OpticalSignalLevel)
	}
}

func fetchJSON(url string, target interface{}) error {
	resp, err := http.Get(url)
	if err != nil {
		logger.Error(err, "Failed to fetch URL", "url", url)
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(target)
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func parseStringToFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func main() {
	zapLog, _ := zap.NewDevelopment()
	logger = zapr.NewLogger(zapLog)

	var rootCmd = &cobra.Command{
		Use:   "vmhub5_exporter",
		Short: "Prometheus exporter for VMHub5 router",
		Run: func(cmd *cobra.Command, args []string) {
			logger.Info("Starting exporter")
			// registerMetrics()

			http.Handle("/metrics", promhttp.Handler())

			// Initial metrics fetch
			fetchAndUpdateMetrics()

			go scheduleMetricsRefresh(interval)

			logger.Info("Exporter running on :8080")
			http.ListenAndServe(":8080", nil)
		},
	}

	rootCmd.Flags().StringVarP(&host, "host", "H", "192.168.0.1", "Router host or IP")
	rootCmd.Flags().DurationVarP(&interval, "interval", "i", 10*time.Second, "Refresh interval")

	if err := rootCmd.Execute(); err != nil {
		logger.Error(err, "Failed to execute command")
		os.Exit(1)
	}
}
