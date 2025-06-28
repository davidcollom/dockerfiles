package unifi

import "fmt"

func (c *UniFiClient) ListClients(site string) ([]Client, error) {
	endpoint := fmt.Sprintf("/api/s/%s/stat/sta", site)
	var clients []Client
	err := c.doRequest("GET", endpoint, nil, &clients)
	if err != nil {
		return nil, err
	}
	return clients, nil
}

func (c *UniFiClient) AuthorizeGuest(site, mac string, duration int) error {
	endpoint := fmt.Sprintf("/api/s/%s/cmd/stamgr", site)
	payload := map[string]interface{}{
		"cmd":     "authorize-guest",
		"mac":     mac,
		"minutes": duration,
	}
	return c.doRequest("POST", endpoint, payload, nil)
}
func (c *UniFiClient) UnauthorizeGuest(site, mac string) error {
	endpoint := fmt.Sprintf("/api/s/%s/cmd/stamgr", site)
	payload := map[string]interface{}{
		"cmd": "unauthorize-guest",
		"mac": mac,
	}
	return c.doRequest("POST", endpoint, payload, nil)
}
