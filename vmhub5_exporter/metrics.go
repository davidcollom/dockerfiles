package main

import "github.com/prometheus/client_golang/prometheus"

const PromNamespace = "hub5"

var (
	ipv4Lease = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: PromNamespace,
		Name:      "provisioning_ipv4_lease_time",
		Help:      "IPv4 Lease Time",
	})
	ipv4Expire = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: PromNamespace,
		Name:      "provisioning_ipv4_expire_time",
		Help:      "IPv4 Expire Time",
	})
	ipv6Lease = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: PromNamespace,
		Name:      "provisioning_ipv6_lease_time",
		Help:      "IPv6 Lease Time",
	})
	ipv6Expire = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: PromNamespace,
		Name:      "provisioning_ipv6_expire_time",
		Help:      "IPv6 Expire Time",
	})
	dsLiteEnable = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: PromNamespace,
		Name:      "provisioning_dsLite_enable",
		Help:      "DSLite Enable",
	})
	ponUpTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: PromNamespace,
		Name:      "pon_status_up_time",
		Help:      "PON Status Up Time",
	})
	ponAccess = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: PromNamespace,
		Name:      "pon_status_access_allowed",
		Help:      "PON Status Access Allowed",
	})
	ponFirewall = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: PromNamespace,
		Name:      "pon_status_firewall",
		Help:      "PON Status Firewall",
	})
	ponIPData = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: PromNamespace,
		Name:      "pon_status_ip_data",
		Help:      "PON Status IP Data",
	})
	ponIPVoip = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: PromNamespace,
		Name:      "pon_status_ip_voip",
		Help:      "PON Status IP VoIP",
	})
	transceiverTemp = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: PromNamespace,
		Name:      "pon_interface_transceiver_temp",
		Help:      "PON Interface Transceiver Temperature",
	})
	transceiverVoltage = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: PromNamespace,
		Name:      "pon_interface_transceiver_voltage",
		Help:      "PON Interface Transceiver Voltage",
	})
	laserBiasCurrent = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: PromNamespace,
		Name:      "pon_interface_laser_bias_current",
		Help:      "PON Interface Laser Bias Current",
	})
	transmitOpticalLevel = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: PromNamespace,
		Name:      "pon_interface_transmit_optical_level",
		Help:      "PON Interface Transmit Optical Level",
	})
	opticalSignalLevel = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: PromNamespace,
		Name:      "pon_interface_optical_signal_level",
		Help:      "PON Interface Optical Signal Level",
	})
)

func init() {
	prometheus.MustRegister(
		ipv4Lease,
		ipv4Expire,
		ipv6Lease,
		ipv6Expire,
		dsLiteEnable,
		ponUpTime,
		ponAccess,
		ponFirewall,
		ponIPData,
		ponIPVoip,
		transceiverTemp,
		transceiverVoltage,
		laserBiasCurrent,
		transmitOpticalLevel,
		opticalSignalLevel,
	)
}
