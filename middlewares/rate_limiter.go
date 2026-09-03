package middlewares

import (
	"context"
	"fmt"
	"time"

	"github.com/Faze-Technologies/go-utils/cache"
	"github.com/Faze-Technologies/go-utils/config"
	"github.com/Faze-Technologies/go-utils/logs"
	"github.com/Faze-Technologies/go-utils/request"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"go.uber.org/zap"
)

// rateLimitConfig returns the rate limiter settings, reading whichever shape
// the service provides. Preferred: "rate-limit", how the static CONFIG blobs
// and the secretConfig secret spell it. Fallback: "RATE_LIMIT", which is how a
// ConfigMap has to spell it - a key named "rate-limit" is skipped by Kubernetes
// envFrom (not a valid env var name), so it arrives as RATE_LIMIT and config's
// generic env pass merges it under "rate_limit", which this lookup matches
// because viper is case-insensitive.
func rateLimitConfig() map[string]interface{} {
	if settings := config.GetMap("rate-limit"); len(settings) > 0 {
		return settings
	}
	return config.GetMap("RATE_LIMIT")
}

func RateLimiter(cache *cache.Cache, redisKey string) gin.HandlerFunc {
	logger := logs.GetLogger()

	whitelistedOrigins := make(map[string]bool)
	for _, origin := range cast.ToStringSlice(rateLimitConfig()["whitelisted-origins"]) {
		whitelistedOrigins[origin] = true
	}
	logger.Info("Rate limiter initialized", zap.Int("whitelisted_origins", len(whitelistedOrigins)))

	return func(c *gin.Context) {
		logger := logs.WithContext(c.Request.Context())
		origin := c.GetHeader("Origin")
		if whitelistedOrigins[origin] {
			logger.Debug("Rate limit skipped for whitelisted origin",
				zap.String("origin", origin),
				zap.String("ip", c.ClientIP()),
				zap.String("path", c.Request.URL.Path),
			)
			c.Next()
			return
		}

		ctx := context.Background()
		ip := resolveClientIP(c)
		key := fmt.Sprintf(redisKey, ip)

		settings := rateLimitConfig()
		rateLimitCount := cast.ToInt(settings["count"])
		rateLimitDuration := time.Second * time.Duration(cast.ToInt(settings["duration"]))
		count, err := cache.Incr(ctx, key, rateLimitDuration)
		if err != nil {
			logger.Error("Rate limiter Redis error", zap.Error(err), zap.String("ip", ip))
			request.SendServiceError(c, request.CreateInternalServerError(err))
			return
		}
		if int(count) > rateLimitCount {
			logger.Warn("Rate limit exceeded",
				zap.String("ip", ip),
				zap.String("origin", origin),
				zap.String("path", c.Request.URL.Path),
				zap.Int64("count", count),
				zap.Int("limit", rateLimitCount),
			)
			request.SendServiceError(c, request.CreateTooManyRequestsError(nil, "Rate Limit Exceeded"))
			return
		}

		c.Next()
	}
}
