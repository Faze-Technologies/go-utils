package geoip

import (
	"net/http"
	"strconv"
	"strings"
)

func boolHeader(b *bool) string {
	if b == nil {
		return ""
	}
	if *b {
		return "true"
	}
	return "false"
}

func floatHeader(f *float64) string {
	if f == nil {
		return ""
	}
	return strconv.FormatFloat(*f, 'f', -1, 64)
}

// BuildHeaders renders the geo result as a flat header map suitable for
// attaching to outbound requests. Empty values are omitted entirely; a missing
// header signals "we don't know" to downstream services.
func BuildHeaders(g GeoResult, prefix string) map[string]string {
	if prefix == "" {
		prefix = defaultHeaderPrefix
	}
	out := map[string]string{}
	put := func(suffix, value string) {
		if value != "" {
			out[prefix+suffix] = value
		}
	}
	put("Ip", g.IP)
	put("Country", g.Country)
	put("Country-Name", g.CountryName)
	put("Region", g.Region)
	put("City", g.City)
	put("Postal-Code", g.PostalCode)
	put("Latitude", floatHeader(g.Latitude))
	put("Longitude", floatHeader(g.Longitude))
	put("Timezone", g.Timezone)
	put("Asn", g.ASN)
	put("Asn-Org", g.ASNOrg)
	put("Isp", g.ISP)
	put("Is-Vpn", boolHeader(g.IsVPN))
	put("Is-Proxy", boolHeader(g.IsProxy))
	put("Is-Tor", boolHeader(g.IsTor))
	put("Is-Hosting", boolHeader(g.IsHostingProvider))
	put("Is-Anonymous", boolHeader(g.IsAnonymous))
	put("Lookup-Status", string(g.LookupStatus))
	return out
}

// StripIncoming removes any client-supplied geo headers from the request so a
// caller cannot spoof their own geolocation.
func StripIncoming(h http.Header, prefix string) {
	if prefix == "" {
		prefix = defaultHeaderPrefix
	}
	lower := strings.ToLower(prefix)
	for k := range h {
		if strings.HasPrefix(strings.ToLower(k), lower) {
			h.Del(k)
		}
	}
}
