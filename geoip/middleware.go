package geoip

import (
	"context"

	"github.com/gin-gonic/gin"
)

type ctxKey struct{}

// FromContext returns the GeoResult attached to ctx by the middleware, or nil
// if the middleware did not run for the current request.
func FromContext(ctx context.Context) *GeoResult {
	v := ctx.Value(ctxKey{})
	if v == nil {
		return nil
	}
	g, ok := v.(*GeoResult)
	if !ok {
		return nil
	}
	return g
}

// Middleware returns a Gin handler that enriches the request context with
// geolocation data. Attach to specific route groups in routes.go — wherever
// attached, it will always run.
//
// Behaviour:
//  1. Strips any incoming X-Fc-Geo-* headers (anti-spoofing)
//  2. Resolves the client IP from X-Forwarded-For
//  3. Looks up the IP and stores the result in the request context
//
// Reads via FromContext(ctx) in handlers. Outbound resty calls that propagate
// the context get headers attached automatically via the OnBeforeRequest hook
// registered by AttachToResty.
func (s *Service) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.config.Enabled {
			c.Next()
			return
		}

		StripIncoming(c.Request.Header, s.config.HeaderPrefix)

		ip := ResolveClientIP(c.Request.Header, c.Request.RemoteAddr)
		var geo GeoResult
		if ip == "" {
			geo = GeoResult{IP: "", LookupStatus: StatusNotFound}
		} else {
			geo = s.engine.lookup(ip)
		}

		ctx := context.WithValue(c.Request.Context(), ctxKey{}, &geo)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
