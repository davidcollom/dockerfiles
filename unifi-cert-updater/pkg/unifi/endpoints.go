package unifi

// Authentication
const (
	EndpointLogin  = "/api/auth/login" // Login to UniFi OS or UDM
	EndpointLogout = "/api/logout"     // Logout from UniFi OS or UDM
	EndpointSelf   = "/api/users/self" // Get current user details
)

// Sites
const (
	EndpointListSites = "/api/self/sites"     // List all sites
	EndpointSiteStats = "/api/s/%s/stat/site" // %s = site name, stats for a site
)

// Devices
const (
	EndpointListDevices = "/api/s/%s/stat/device"    // %s = site name, list all devices
	EndpointGetDevice   = "/api/s/%s/stat/device/%s" // %s = site name, %s = device MAC, get specific device
)

// Clients
const (
	EndpointListClients      = "/api/s/%s/stat/sta"   // %s = site name, list clients
	EndpointAuthorizeGuest   = "/api/s/%s/cmd/stamgr" // %s = site name, authorize a guest
	EndpointUnauthorizeGuest = "/api/s/%s/cmd/stamgr" // %s = site name, unauthorize a guest
	EndpointReconnectClient  = "/api/s/%s/cmd/stamgr" // %s = site name, reconnect a client
	EndpointBlockClient      = "/api/s/%s/cmd/stamgr" // %s = site name, block a client
	EndpointUnblockClient    = "/api/s/%s/cmd/stamgr" // %s = site name, unblock a client
)

// Certificates
const (
	EndpointListCertificates    = "/api/userCertificates"           // List all certificates
	EndpointCreateCertificate   = "/api/userCertificates"           // Create/upload a certificate
	EndpointActivateCertificate = "/api/userCertificates/%s/status" // %s = certificate ID, activate a certificate
	EndpointDeleteCertificate   = "/api/userCertificates/%s"        // %s = certificate ID, delete a certificate
)

// Health and Metrics
const (
	EndpointListHealth = "/api/s/%s/stat/health" // %s = site name, health metrics
	EndpointDashboard  = "/api/s/%s/dashboard"   // %s = site name, dashboard metrics
)

// Voucher System
const (
	EndpointCreateVoucher = "/api/s/%s/cmd/hotspot"  // %s = site name, create a voucher
	EndpointListVouchers  = "/api/s/%s/stat/voucher" // %s = site name, list vouchers
	EndpointDeleteVoucher = "/api/s/%s/cmd/hotspot"  // %s = site name, delete a voucher
)

// User Management
const (
	EndpointListUsers = "/api/s/%s/list/user" // %s = site name, list all users
)

// Network Configuration
const (
	EndpointListNetworks = "/api/s/%s/rest/networkconf" // %s = site name, list network configurations
)
