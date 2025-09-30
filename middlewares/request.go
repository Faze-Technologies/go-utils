package middlewares

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/mileusna/useragent"
)

func ParseUserAgent() gin.HandlerFunc {
	return func(c *gin.Context) {
		parsedUA := useragent.Parse(c.Request.Header.Get("User-Agent"))
		ctx := context.WithValue(c.Request.Context(), "parsedUA", parsedUA)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func SetResponseHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Transfer-Encoding", "identity")
		c.Next()
	}
}
