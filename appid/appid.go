// Package appid is the one place that knows how the app of a request is named, carried and checked.
//
// The app a request is for travels in the app-id request header and nowhere else: never a query string, never a body.
// The edge service (a BFF or gateway) resolves it once with Middleware and sends it on to every service behind it with
// Forward. A service behind the edge reads it with Require, or FromHeader on a route that works without an app.
package appid

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/Faze-Technologies/go-utils/request"
	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
)

// Header is the request header that says which app a request is for.
const Header = "app-id"

var pattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

// Valid reports whether a trimmed value can name an app: 1 to 64 characters, letters, digits, "_" and "-", starting
// with a letter or a digit. Every service that checks an app id uses this one rule.
func Valid(id string) bool {
	return pattern.MatchString(id)
}

// FromHeader returns the trimmed app-id header of the request, "" when it is absent.
func FromHeader(c *gin.Context) string {
	return strings.TrimSpace(c.GetHeader(Header))
}

// Require returns the app-id header of the request. When it is missing it answers 400 and returns false: the caller
// must stop.
func Require(c *gin.Context) (string, bool) {
	id := FromHeader(c)
	if id == "" {
		message := Header + " header is required"
		request.SendServiceError(c, request.CreateBadRequestError(errors.New(message), message))
		return "", false
	}
	return id, true
}

type key struct{}

// With returns ctx carrying the resolved app.
func With(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, key{}, id)
}

// From returns the app With stored in ctx, "" when there is none.
func From(ctx context.Context) string {
	id, _ := ctx.Value(key{}).(string)
	return id
}

// Middleware resolves the app of every request at the edge: the app-id header, or fallback when it is absent. A value
// that is not a valid app id is refused with 400. The result is kept in the request context (From) for the handlers
// and for Forward.
func Middleware(fallback string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := FromHeader(c)
		if id == "" {
			id = fallback
		}
		if !Valid(id) {
			message := Header + " must be 1 to 64 characters: letters, digits, _ and -, starting with a letter or digit"
			request.SendServiceError(c, request.CreateBadRequestError(errors.New(message), message))
			c.Abort()
			return
		}
		c.Request = c.Request.WithContext(With(c.Request.Context(), id))
		c.Next()
	}
}

// Forward makes every call on client send the app of the request as the app-id header, read from the request context
// (calls pass it with .SetContext(c.Request.Context())). It is the only place the app is sent downstream.
func Forward(client *resty.Client) {
	client.OnBeforeRequest(func(_ *resty.Client, r *resty.Request) error {
		r.SetHeader(Header, From(r.Context()))
		return nil
	})
}
