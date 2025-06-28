package unifi

import "fmt"

func (c *UniFiClient) ListDevices(site string) ([]Device, error) {
	endpoint := fmt.Sprintf(EndpointListDevices, site)
	var devices []Device
	err := c.doRequest("GET", endpoint, nil, &devices)
	if err != nil {
		return nil, err
	}
	return devices, nil
}

func (c *UniFiClient) GetDevice(site, mac string) (Device, error) {
	endpoint := fmt.Sprintf("/api/s/%s/stat/device/%s", site, mac)
	var device Device
	err := c.doRequest("GET", endpoint, nil, &device)
	if err != nil {
		return device, err
	}
	return device, nil
}
