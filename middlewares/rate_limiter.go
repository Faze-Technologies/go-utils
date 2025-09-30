package middlewares

import (
	"context"
	"fmt"
	"time"

	"github.com/Faze-Technologies/go-utils/cache"
	"github.com/Faze-Technologies/go-utils/config"
	"github.com/Faze-Technologies/go-utils/request"
	"github.com/gin-gonic/gin"
)

func RateLimiter(cache *cache.Cache, redisKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()
		ip := c.ClientIP()
		key := fmt.Sprintf(redisKey, ip)

		rateLimitCount := config.GetInt("rate-limit.count")
		rateLimitDuration := time.Second * time.Duration(config.GetInt("rate-limit.duration"))
		count, err := cache.Incr(ctx, key, rateLimitDuration)
		if err != nil {
			request.SendServiceError(c, request.CreateInternalServerError(err))
			return
		}
		if int(count) > rateLimitCount {
			request.SendServiceError(c, request.CreateTooManyRequestsError(nil, "Rate Limit Exceeded"))
			return
		}

		c.Next()
	}
}
