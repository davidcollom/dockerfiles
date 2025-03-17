package main

type ProvisioningResponse struct {
	Provisioning struct {
		IPv4 struct {
			LeaseTime  float64 `json:"leaseTime"`
			ExpireTime float64 `json:"expireTime"`
		} `json:"ipv4"`
		IPv6 struct {
			LeaseTime  float64 `json:"leaseTime"`
			ExpireTime float64 `json:"expireTime"`
		} `json:"ipv6"`
		DSLite struct {
			Enable bool `json:"enable"`
		} `json:"dsLite"`
	} `json:"provisioning"`
}

type PonStateResponse struct {
	Pon struct {
		UpTime        float64 `json:"upTime"`
		AccessAllowed bool    `json:"accessAllowed"`
	} `json:"pon"`
}

type PonStatusResponse struct {
	Status struct {
		Firewall bool   `json:"firewall"`
		IPData   string `json:"ipData"`
		IPVoip   string `json:"ipVoip"`
	} `json:"status"`
	Interface struct {
		TransceiverTemp      string `json:"transceiverTemp"`
		TransceiverVoltage   string `json:"transceiverVoltage"`
		LaserBiasCurrent     string `json:"laserBiasCurrent"`
		TransmitOpticalLevel string `json:"transmitOpticalLevel"`
		OpticalSignalLevel   string `json:"opticalSignalLevel"`
	} `json:"interface"`
}
