package unifi

import "fmt"

func (c *UniFiClient) CreateVoucher(site string, payload VoucherCreatePayload) error {
	endpoint := fmt.Sprintf("/api/s/%s/cmd/hotspot", site)
	payload.Cmd = "create-voucher" // Mandatory command
	return c.doRequest("POST", endpoint, payload, nil)
}
