package geoip

import "time"

type LookupStatus string

const (
	StatusOK       LookupStatus = "ok"
	StatusNotFound LookupStatus = "not_found"
	StatusError    LookupStatus = "error"
	StatusDisabled LookupStatus = "disabled"
)

type GeoResult struct {
	IP                string       `json:"ip"`
	Country           string       `json:"country,omitempty"`
	CountryName       string       `json:"countryName,omitempty"`
	Region            string       `json:"region,omitempty"`
	City              string       `json:"city,omitempty"`
	PostalCode        string       `json:"postalCode,omitempty"`
	Latitude          *float64     `json:"latitude,omitempty"`
	Longitude         *float64     `json:"longitude,omitempty"`
	Timezone          string       `json:"timezone,omitempty"`
	ASN               string       `json:"asn,omitempty"`
	ASNOrg            string       `json:"asnOrg,omitempty"`
	ISP               string       `json:"isp,omitempty"`
	IsVPN             *bool        `json:"isVpn,omitempty"`
	IsProxy           *bool        `json:"isProxy,omitempty"`
	IsTor             *bool        `json:"isTor,omitempty"`
	IsHostingProvider *bool        `json:"isHostingProvider,omitempty"`
	IsAnonymous       *bool        `json:"isAnonymous,omitempty"`
	LookupStatus      LookupStatus `json:"lookupStatus"`
}

type Config struct {
	Enabled         bool
	DBPath          string
	GCSBucket       string
	GCSPrefix       string
	RefreshInterval time.Duration
	HeaderPrefix    string
}
