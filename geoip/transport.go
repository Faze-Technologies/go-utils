package geoip

import (
	"context"

	"github.com/go-resty/resty/v2"
)

// AttachToResty registers a single OnBeforeRequest hook on the given resty
// client that copies the geo headers from the request context onto every
// outbound call. Callers must propagate the gin request context via
// .R().SetContext(c.Request.Context()) — the standard pattern in
// spinner-bff-service.
//
// Safe to call multiple times on the same client; each call adds a hook.
// Typical use: call once at boot after geoip.Init() against the shared client.
func (s *Service) AttachToResty(client *resty.Client) {
	if client == nil {
		return
	}
	prefix := s.config.HeaderPrefix
	client.OnBeforeRequest(func(_ *resty.Client, r *resty.Request) error {
		geo := FromContext(r.Context())
		if geo == nil {
			return nil
		}
		for k, v := range BuildHeaders(*geo, prefix) {
			r.SetHeader(k, v)
		}
		return nil
	})
}

// HeadersFromContext returns the geo headers built from whatever is attached
// to ctx, or nil if the middleware did not run for this context. Use it when
// building outbound requests with a non-resty HTTP client.
func (s *Service) HeadersFromContext(ctx context.Context) map[string]string {
	g := FromContext(ctx)
	if g == nil {
		return nil
	}
	return BuildHeaders(*g, s.config.HeaderPrefix)
}
