package unifi

import "fmt"

func (c *UniFiClient) ListSites() ([]Site, error) {
	endpoint := "/api/self/sites"
	var sites []Site
	err := c.doRequest("GET", endpoint, nil, &sites)
	if err != nil {
		return nil, err
	}
	return sites, nil
}

func (c *UniFiClient) ListSiteStats(site string) (SiteStats, error) {
	endpoint := fmt.Sprintf("/api/s/%s/stat/site", site)
	var stats SiteStats
	err := c.doRequest("GET", endpoint, nil, &stats)
	if err != nil {
		return stats, err
	}
	return stats, nil
}
