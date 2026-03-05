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
	"go.uber.org/zap"
)

func RateLimiter(cache *cache.Cache, redisKey string) gin.HandlerFunc {
	logger := logs.GetLogger()

	whitelistedOrigins := make(map[string]bool)
	for _, origin := range config.GetSlice("rate-limit.whitelisted-origins") {
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
		ip := c.ClientIP()
		key := fmt.Sprintf(redisKey, ip)

		rateLimitCount := config.GetInt("rate-limit.count")
		rateLimitDuration := time.Second * time.Duration(config.GetInt("rate-limit.duration"))
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
