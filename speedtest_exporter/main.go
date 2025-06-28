package main

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/robfig/cron/v3"
	"github.com/showwin/speedtest-go/speedtest"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	speedtestDownloadBits = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "speedtest_download_bits",
		Help: "Download bandwidth in bit/s",
	})
	speedtestUploadBits = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "speedtest_upload_bits",
		Help: "Upload bandwidth in bit/s",
	})
	speedtestDownloadBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "speedtest_download_bytes",
		Help: "Download usage capacity (bytes)",
	})
	speedtestUploadBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "speedtest_upload_bytes",
		Help: "Upload usage capacity (bytes)",
	})
	speedtestPing = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "speedtest_ping",
		Help: "ICMP latency (ms)",
	})
	speedtestUp = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "speedtest_up",
		Help: "Exporter is up (1) or down (0)",
	})
)

func init() {
	prometheus.MustRegister(
		speedtestDownloadBits,
		speedtestUploadBits,
		speedtestDownloadBytes,
		speedtestUploadBytes,
		speedtestPing,
		speedtestUp,
	)
}

func fetchMetrics(serverID string) func() {
	return func() {
		log.Println("Running speedtest...")
		user, _ := speedtest.FetchUserInfo()
		log.Printf("User info: %+v", user)
		_ = user // Suppress unused variable warning
		var targetServers speedtest.Servers
		var targetServer speedtest.Server
		var ptrServer *speedtest.Server
		var err error

		if serverID != "" {
			ptrServer, err = speedtest.FetchServerByID(serverID)
			targetServer = *ptrServer
		} else {
			targetServers, err = speedtest.FetchServers()
			availableServers := targetServers.Available()
			servers := *availableServers
			targetServer = *servers[0]
		}
		if err != nil {
			log.Printf("Error finding server: %v", err)
			speedtestUp.Set(0)
			return
		}

		log.Printf("Found %d servers, using server ID %v", len(targetServers), targetServers[0].ID)

		err = targetServer.PingTest(func(latency time.Duration) {
			log.Printf("Ping: %v", latency)
			speedtestPing.Set(float64(latency.Milliseconds()))
		})
		if err != nil {
			log.Printf("Ping failed: %v", err)
			speedtestUp.Set(0)
			return
		}

		log.Printf("Starting Download and Upload tests for server ID %v", targetServer.ID)

		err = targetServer.DownloadTest()
		if err != nil {
			log.Printf("Download failed: %v", err)
			speedtestUp.Set(0)
			return
		}

		err = targetServer.UploadTest()
		if err != nil {
			log.Printf("Upload failed: %v", err)
			speedtestUp.Set(0)
			return
		}

		speedtestUp.Set(1)
		speedtestDownloadBits.Set(float64(targetServer.DLSpeed))
		speedtestUploadBits.Set(float64(targetServer.ULSpeed))
		speedtestDownloadBytes.Set(float64(targetServer.DLSpeed) / 8) // rough conversion
		speedtestUploadBytes.Set(float64(targetServer.ULSpeed) / 8)
		speedtestPing.Set(float64(targetServer.Latency.Milliseconds()))
		log.Printf("Download speed: %s, Upload speed: %s, Ping: %d ms", targetServer.DLSpeed.String(), targetServer.ULSpeed.String(), targetServer.Latency.Milliseconds())
		log.Println("Speedtest completed and metrics updated")
	}
}

func main() {
	// CLI + Env flags
	pflag.String("listen", "0.0.0.0", "Listen address")
	pflag.Int("port", 9353, "Port to expose metrics")
	pflag.String("interval", "*/20 * * * *", "Cron interval")
	pflag.String("server", "", "Speedtest server ID")
	pflag.Bool("debug", false, "Enable debug logs")
	pflag.Parse()

	viper.BindPFlags(pflag.CommandLine)
	viper.AutomaticEnv()

	if viper.GetBool("debug") {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		log.Println("Debug logging enabled")
	}

	listen := viper.GetString("listen")
	port := viper.GetInt("port")
	interval := viper.GetString("interval")
	serverID := viper.GetString("server")

	// Start metrics endpoint
	go func() {
		addr := listen + ":" + strconv.Itoa(port)
		log.Printf("Starting HTTP server at %s", addr)
		http.Handle("/metrics", promhttp.Handler())
		log.Fatal(http.ListenAndServe(addr, nil))
	}()

	// Cron job
	c := cron.New()
	_, err := c.AddFunc(interval, fetchMetrics(serverID))
	if err != nil {
		log.Fatalf("Error scheduling job: %v", err)
	}
	c.Start()

	// Run immediately once
	fetchMetrics(serverID)()

	select {} // block forever
}
