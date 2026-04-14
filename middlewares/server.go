package middlewares

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/Faze-Technologies/go-utils/logs"
	"github.com/Faze-Technologies/go-utils/request"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)


// responseBodyWriter wraps gin.ResponseWriter to capture the response body.
type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (r *responseBodyWriter) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

var sensitiveKeys = map[string]bool{
	"password": true, "token": true, "otp": true,
	"cardNumber": true, "cvv": true, "privateKey": true,
	"secret": true, "authorization": true,
}

// resolveInternalUserID extracts userId from headers/query for internal service-to-service calls.
// c.GetHeader is case-insensitive for hyphenated headers (user-id == User-Id == USER-ID),
// but "userId" has a different canonical form so it needs its own check.
func resolveInternalUserID(c *gin.Context) string {
	for _, h := range []string{"user-id", "userId"} {
		if v := c.GetHeader(h); v != "" {
			return v
		}
	}
	return c.Query("userId")
}

// redactBody parses JSON body and masks sensitive fields before attaching to spans/logs.
func redactBody(body []byte) string {
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return "[non-json body]"
	}
	for key := range data {
		if sensitiveKeys[strings.ToLower(key)] {
			data[key] = "[REDACTED]"
		}
	}
	out, _ := json.Marshal(data)
	return string(out)
}

func GinLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		c.Set("logs", logger)

		// Buffer request body so handler can still read it
		var bodyBytes []byte
		contentType := c.Request.Header.Get("Content-Type")
		if c.Request.Body != nil && !strings.Contains(contentType, "multipart") {
			bodyBytes, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		// Wrap the response writer to capture the response body
		respWriter := &responseBodyWriter{body: &bytes.Buffer{}, ResponseWriter: c.Writer}
		c.Writer = respWriter

		c.Next()

		statusCode := c.Writer.Status()
		cost := time.Since(start)

		// Resolve client IP from X-Forwarded-For header (first IP is the real client),
		// falling back to Gin's ClientIP() which depends on SetTrustedProxies config.
		clientIP := ""
		if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
			clientIP = strings.TrimSpace(strings.Split(xff, ",")[0])
		}
		if clientIP == "" {
			clientIP = c.ClientIP()
		}

		span := trace.SpanFromContext(c.Request.Context())

		// ── Span attributes (searchable/filterable in SigNoz) ──────────────────
		// These are set on the span itself so you can filter traces by userId,
		// appVersion, platform, etc. across ALL requests from SigNoz's search UI.
		span.SetAttributes(
			attribute.String("http.method", c.Request.Method),
			attribute.String("http.path", path),
			attribute.Int("http.status_code", statusCode),
			attribute.String("http.client_ip", clientIP),
			attribute.String("http.user_agent", c.Request.UserAgent()),
		)

		// Resolve user once — used for both span attributes and structured log
		var userID string
		if user, err := GetAuthUser(c); err == nil {
			userID = user.Id
			span.SetAttributes(
				attribute.String("user.id", user.Id),
				attribute.String("user.email", user.Email),
				attribute.String("user.mobile", user.Mobile),
				attribute.Bool("user.kyc", user.KycStatus),
				attribute.String("user.kyc_country", user.KycCountry),
			)
		} else if id := resolveInternalUserID(c); id != "" {
			userID = id
			span.SetAttributes(attribute.String("user.id", id))
		}

		if v := c.GetHeader("appversion"); v != "" {
			span.SetAttributes(attribute.String("app.version", v))
		}
		if v := c.GetHeader("appid"); v != "" {
			span.SetAttributes(attribute.String("app.id", v))
		}
		if v := c.GetHeader("source"); v != "" {
			span.SetAttributes(attribute.String("app.platform", v))
		}

		// Span event: request body, query, path params, response body on errors
		var eventAttrs []attribute.KeyValue
		if len(bodyBytes) > 0 {
			eventAttrs = append(eventAttrs, attribute.String("http.request.body", redactBody(bodyBytes)))
		}
		if query != "" {
			eventAttrs = append(eventAttrs, attribute.String("http.request.query", query))
		}
		if params := c.Params; len(params) > 0 {
			paramMap := make(map[string]string, len(params))
			for _, p := range params {
				paramMap[p.Key] = p.Value
			}
			if b, err := json.Marshal(paramMap); err == nil {
				eventAttrs = append(eventAttrs, attribute.String("http.request.params", string(b)))
			}
		}
		if statusCode >= 400 {
			if respBody := respWriter.body.Bytes(); len(respBody) > 0 {
				eventAttrs = append(eventAttrs, attribute.String("http.response.body", string(respBody)))
			}
		}
		if len(eventAttrs) > 0 {
			span.AddEvent("request.payload", trace.WithAttributes(eventAttrs...))
		}

		if statusCode >= 500 {
			span.SetStatus(codes.Error, http.StatusText(statusCode))
		}

		logFields := []zap.Field{
			zap.Int("status", statusCode),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.String("ip", clientIP),
			zap.String("user-agent", c.Request.UserAgent()),
			zap.String("errors", c.Errors.ByType(gin.ErrorTypePrivate).String()),
			zap.Duration("cost", cost),
		}
		if userID != "" {
			logFields = append(logFields, zap.String("userId", userID))
		}
		logs.WithContext(c.Request.Context()).Info(path, logFields...)
	}
}

func GinRecovery(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Check for a broken connection, as it is not really a
				// condition that warrants a panic stack trace.
				var brokenPipe bool
				if ne, ok := err.(*net.OpError); ok {
					if se, ok := ne.Err.(*os.SyscallError); ok {
						if strings.Contains(strings.ToLower(se.Error()), "broken pipe") || strings.Contains(strings.ToLower(se.Error()), "connection reset by peer") {
							brokenPipe = true
						}
					}
				}

				httpRequest, _ := httputil.DumpRequest(c.Request, true)
				if brokenPipe {
					logger.Sugar().Error(c.Request.URL.Path, zap.Any("error", err))
					logger.Sugar().Error(string(httpRequest))
					// If the connection is dead, we can't write a status to it.
					c.Error(err.(error))
					c.Abort()
					return
				}
				logger.Sugar().Error(err)
				logger.Sugar().Error(string(debug.Stack()))
				logger.Sugar().Error("[raw http request] ", string(httpRequest))
				request.SendServiceError(c, request.CreateInternalServerError(nil))
			}
		}()
		c.Next()
	}
}
