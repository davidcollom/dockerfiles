package unifi

import "time"

// Common Types

// ResponseMeta represents the metadata in API responses.
type ResponseMeta struct {
	RC  string `json:"rc"`  // Return Code: "ok" or "error"
	Msg string `json:"msg"` // Error message (if any)
}

// ErrorResponse represents a generic error response from UniFi.
type ErrorResponse struct {
	Meta ResponseMeta  `json:"meta"`
	Data []interface{} `json:"data"`
}

// SuccessfulResponse represents a generic successful response.
type SuccessfulResponse[T any] struct {
	Meta ResponseMeta `json:"meta"`
	Data []T          `json:"data"`
}

// Authentication Types

// LoginPayload represents the data for login requests.
type LoginPayload struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Users

// User represents a UniFi user object.
type User struct {
	ID       string `json:"_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Email    string `json:"email"`
	Disabled bool   `json:"disabled"`
}

// Sites

// Site represents a UniFi site object.
type Site struct {
	ID          string `json:"_id"`
	Name        string `json:"name"`
	Description string `json:"desc"`
	Role        string `json:"role"`
}

// Devices

// Device represents a UniFi device object.
type Device struct {
	ID            string  `json:"_id"`
	Name          string  `json:"name"`
	Model         string  `json:"model"`
	MAC           string  `json:"mac"`
	IP            string  `json:"ip"`
	Adopted       bool    `json:"adopted"`
	LastSeen      int64   `json:"last_seen"`
	Firmware      string  `json:"firmware"`
	Uptime        int64   `json:"uptime"`
	Status        string  `json:"status"`
	DownlinkCount int     `json:"num_sta"`
	Uplink        *Uplink `json:"uplink,omitempty"` // Optional, only for devices with uplinks
}

// Uplink represents the uplink information for a device.
type Uplink struct {
	Mac  string `json:"uplink_mac"`
	IP   string `json:"uplink_ip"`
	Name string `json:"uplink_name"`
}

// Certificates

// Certificate represents a UniFi certificate object.
type Certificate struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	SerialNumber string     `json:"serial_number"`
	Fingerprint  string     `json:"fingerprint"`
	Subject      Subject    `json:"subject"`
	Issuer       Issuer     `json:"issuer"`
	SubjectAlt   SubjectAlt `json:"subject_alt_name"`
	ValidFrom    time.Time  `json:"valid_from"`
	ValidTo      time.Time  `json:"valid_to"`
	Active       bool       `json:"active"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// Subject contains certificate subject details.
type Subject struct {
	CN string `json:"CN"`
}

// Issuer contains certificate issuer details.
type Issuer struct {
	C  string `json:"C"`
	O  string `json:"O"`
	CN string `json:"CN"`
}

// SubjectAlt represents the Subject Alternative Name of a certificate.
type SubjectAlt struct {
	DNS []string `json:"DNS"`
}

// Clients

// Client represents a UniFi client object.
type Client struct {
	ID        string `json:"_id"`
	Mac       string `json:"mac"`
	IP        string `json:"ip"`
	Hostname  string `json:"hostname"`
	Connected bool   `json:"connected"`
	SiteID    string `json:"site_id"`
}

// Health Metrics

// HealthMetric represents health statistics for a UniFi site.
type HealthMetric struct {
	ID     string `json:"_id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// Dashboard

// DashboardMetric represents a dashboard metric.
type DashboardMetric struct {
	ID        string `json:"_id"`
	Name      string `json:"name"`
	Value     int    `json:"value"`
	Timestamp int64  `json:"timestamp"`
}

// Voucher System

// Voucher represents a guest voucher.
type Voucher struct {
	Code      string `json:"code"`
	Duration  int    `json:"duration"`
	Quota     int    `json:"quota"`
	Note      string `json:"note"`
	CreatedAt int64  `json:"created_at"`
	Status    string `json:"status"`
}

// VoucherCreatePayload represents the payload to create a voucher.
type VoucherCreatePayload struct {
	Cmd       string `json:"cmd"`
	Minutes   int    `json:"minutes"`
	Quota     int    `json:"quota"`
	Note      string `json:"note"`
	UpLimit   int    `json:"up_limit,omitempty"`
	DownLimit int    `json:"down_limit,omitempty"`
}

// SiteStats represents the statistics for a specific site.
type SiteStats struct {
	ID              string    `json:"_id"`
	Name            string    `json:"name"`
	Description     string    `json:"desc"`
	Health          []Health  `json:"health"`
	NumClients      int       `json:"num_clients"`
	NumDevices      int       `json:"num_devices"`
	NumGuestDevices int       `json:"num_guest_devices"`
	Uptime          int64     `json:"uptime"`
	LastSeen        time.Time `json:"last_seen"`
}

// Health represents the health statistics of a site.
type Health struct {
	ID        string `json:"_id"`
	Status    string `json:"status"`
	Name      string `json:"name"`
	SubSystem string `json:"subsystem"`
	NumErrors int    `json:"num_errors"`
}
