package geoip

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/oschwald/geoip2-golang"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// readerCloseDelay is how long we wait after swapping in new readers before
// closing the old ones. geoip2-golang readers are backed by mmap; closing
// while another goroutine is mid-lookup would SIGBUS the process. 30s is
// safely larger than any realistic in-flight request lifetime.
const readerCloseDelay = 30 * time.Second

var (
	cityCandidates    = []string{"GeoIP2-City.mmdb", "GeoLite2-City.mmdb"}
	countryCandidates = []string{"GeoIP2-Country.mmdb", "GeoLite2-Country.mmdb"}
	ispCandidates     = []string{"GeoIP2-ISP.mmdb", "GeoIP2-ASN.mmdb", "GeoLite2-ASN.mmdb"}
	anonCandidates    = []string{"GeoIP2-Anonymous-IP.mmdb"}
)

type readerSet struct {
	city        *geoip2.Reader
	country     *geoip2.Reader
	asn         *geoip2.Reader
	isp         *geoip2.Reader
	anon        *geoip2.Reader
	loadedFiles []string
}

type lookupEngine struct {
	current atomic.Pointer[readerSet]
	logger  *zap.Logger
}

func newLookupEngine(logger *zap.Logger) *lookupEngine {
	e := &lookupEngine{logger: logger}
	e.current.Store(&readerSet{})
	return e
}

func firstExisting(dir string, candidates []string) string {
	for _, name := range candidates {
		full := filepath.Join(dir, name)
		if _, err := os.Stat(full); err == nil {
			return full
		}
	}
	return ""
}

func (e *lookupEngine) ready() bool {
	rs := e.current.Load()
	return rs.city != nil || rs.country != nil || rs.asn != nil || rs.isp != nil
}

func (e *lookupEngine) loadFromDirectory(dir string) error {
	// Resolve all paths up front; opens then run in parallel.
	cityPath := firstExisting(dir, cityCandidates)
	countryPath := ""
	if cityPath == "" {
		countryPath = firstExisting(dir, countryCandidates)
	}
	ispPath := firstExisting(dir, ispCandidates)
	anonPath := firstExisting(dir, anonCandidates)

	next := &readerSet{}
	var g errgroup.Group

	if cityPath != "" {
		g.Go(func() error {
			r, err := geoip2.Open(cityPath)
			if err != nil {
				return fmt.Errorf("open city db %s: %w", cityPath, err)
			}
			next.city = r
			return nil
		})
	}
	if countryPath != "" {
		g.Go(func() error {
			r, err := geoip2.Open(countryPath)
			if err != nil {
				return fmt.Errorf("open country db %s: %w", countryPath, err)
			}
			next.country = r
			return nil
		})
	}
	if ispPath != "" {
		g.Go(func() error {
			r, err := geoip2.Open(ispPath)
			if err != nil {
				return fmt.Errorf("open asn/isp db %s: %w", ispPath, err)
			}
			// ISP and ASN files share the underlying mmdb shape — pick by filename.
			if filepath.Base(ispPath) == "GeoIP2-ISP.mmdb" {
				next.isp = r
			} else {
				next.asn = r
			}
			return nil
		})
	}
	if anonPath != "" {
		g.Go(func() error {
			r, err := geoip2.Open(anonPath)
			if err != nil {
				return fmt.Errorf("open anon db %s: %w", anonPath, err)
			}
			next.anon = r
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	// Build loadedFiles deterministically (parallel goroutines can't safely
	// append to a shared slice).
	for _, p := range []string{cityPath, countryPath, ispPath, anonPath} {
		if p != "" {
			next.loadedFiles = append(next.loadedFiles, filepath.Base(p))
		}
	}

	if len(next.loadedFiles) == 0 {
		e.logger.Warn("[GEOIP] no .mmdb files found", zap.String("dir", dir))
		return nil
	}

	prev := e.current.Swap(next)
	go closeReadersAfter(prev, readerCloseDelay)
	e.logger.Info("[GEOIP] loaded readers", zap.String("dir", dir), zap.Strings("files", next.loadedFiles))
	return nil
}

func closeReadersAfter(rs *readerSet, delay time.Duration) {
	if rs == nil {
		return
	}
	time.Sleep(delay)
	for _, r := range []*geoip2.Reader{rs.city, rs.country, rs.asn, rs.isp, rs.anon} {
		if r != nil {
			_ = r.Close()
		}
	}
}

func boolPtr(b bool) *bool       { return &b }
func float64Ptr(f float64) *float64 { return &f }

func (e *lookupEngine) lookup(ipStr string) GeoResult {
	out := GeoResult{IP: ipStr, LookupStatus: StatusNotFound}
	if !e.ready() {
		out.LookupStatus = StatusDisabled
		return out
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return out
	}

	rs := e.current.Load()
	anyHit := false

	if rs.city != nil {
		rec, err := rs.city.City(ip)
		if err == nil && rec != nil {
			anyHit = true
			out.Country = rec.Country.IsoCode
			out.CountryName = rec.Country.Names["en"]
			if rec.City.Names != nil {
				out.City = rec.City.Names["en"]
			}
			if len(rec.Subdivisions) > 0 {
				out.Region = rec.Subdivisions[0].Names["en"]
			}
			out.PostalCode = rec.Postal.Code
			out.Latitude = float64Ptr(rec.Location.Latitude)
			out.Longitude = float64Ptr(rec.Location.Longitude)
			out.Timezone = rec.Location.TimeZone
		}
	} else if rs.country != nil {
		rec, err := rs.country.Country(ip)
		if err == nil && rec != nil {
			anyHit = true
			out.Country = rec.Country.IsoCode
			out.CountryName = rec.Country.Names["en"]
		}
	}

	if rs.isp != nil {
		rec, err := rs.isp.ISP(ip)
		if err == nil && rec != nil {
			anyHit = true
			if rec.AutonomousSystemNumber != 0 {
				out.ASN = fmt.Sprintf("AS%d", rec.AutonomousSystemNumber)
			}
			out.ASNOrg = rec.AutonomousSystemOrganization
			out.ISP = rec.ISP
		}
	} else if rs.asn != nil {
		rec, err := rs.asn.ASN(ip)
		if err == nil && rec != nil {
			anyHit = true
			if rec.AutonomousSystemNumber != 0 {
				out.ASN = fmt.Sprintf("AS%d", rec.AutonomousSystemNumber)
			}
			out.ASNOrg = rec.AutonomousSystemOrganization
		}
	}

	if rs.anon != nil {
		rec, err := rs.anon.AnonymousIP(ip)
		if err == nil && rec != nil {
			anyHit = true
			out.IsAnonymous = boolPtr(rec.IsAnonymous)
			out.IsVPN = boolPtr(rec.IsAnonymousVPN)
			out.IsProxy = boolPtr(rec.IsPublicProxy)
			out.IsTor = boolPtr(rec.IsTorExitNode)
			out.IsHostingProvider = boolPtr(rec.IsHostingProvider)
		}
	}

	if anyHit {
		out.LookupStatus = StatusOK
	}
	return out
}
