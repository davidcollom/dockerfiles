package main

import (
	"log"
	"net"
	"net/http"
	"strconv"

	"io"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/robfig/cron/v3"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	ispInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "isp_info",
		Help: "Information about my ISP which is in use.",
	}, []string{"hostname", "ip_address"})

	ispInfoRunning = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "isp_info_running",
		Help: "ISP Info is running when 1",
	})

	ispInfoLastRun = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "isp_info_last_run",
		Help: "ISP Info Last Checked/Updated",
	})

	ispInfoUpdateTime = prometheus.NewSummary(prometheus.SummaryOpts{
		Name: "isp_info_update_time",
		Help: "Summary of update duration",
	})
)

func init() {
	prometheus.MustRegister(ispInfo, ispInfoRunning, ispInfoLastRun, ispInfoUpdateTime)
}

func lookupISP() {
	timer := prometheus.NewTimer(ispInfoUpdateTime)
	defer timer.ObserveDuration()

	ispInfoRunning.Set(1)
	defer ispInfoRunning.Set(0)

	log.Println("Updating ISP info")

	ip := "127.0.0.1"
	hostname := "unknown"

	resp, err := http.Get("https://api.ipify.org")
	if err != nil {
		log.Printf("Error getting external IP: %v", err)
	} else {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		ip = string(body)
		log.Printf("Got IP of %s", ip)
	}

	names, err := net.LookupAddr(ip)
	if err != nil {
		log.Printf("Error resolving hostname: %v", err)
	} else if len(names) > 0 {
		hostname = names[0]
		log.Printf("Resolved hostname: %s", hostname)
	}

	ispInfoLastRun.SetToCurrentTime()
	ispInfo.WithLabelValues(hostname, ip).SetToCurrentTime()
	log.Println("ISP info metrics updated")
}

func main() {
	// Flags and env config
	pflag.String("listen", "0.0.0.0", "Listen address")
	pflag.Int("port", 9353, "Port to expose metrics")
	pflag.String("interval", "*/20 * * * *", "Cron interval")
	pflag.Bool("debug", false, "Enable debug logging")
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

	// Prometheus metrics endpoint
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		addr := listen + ":" + strconv.Itoa(port)
		log.Printf("Starting HTTP server on %s", addr)
		log.Fatal(http.ListenAndServe(addr, nil))
	}()

	// Cron job
	c := cron.New()
	_, err := c.AddFunc(interval, lookupISP)
	if err != nil {
		log.Fatalf("Error scheduling job: %v", err)
	}
	c.Start()

	// Run once immediately
	lookupISP()

	select {} // block forever
}
