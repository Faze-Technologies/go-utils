package server

import (
	"context"
	"github.com/Faze-Technologies/go-utils/request"
	"github.com/gin-gonic/gin"
	"github.com/mileusna/useragent"
	"go.uber.org/zap"
	"net"
	"net/http/httputil"
	"os"
	"runtime/debug"
	"strings"
	"time"
)

func GinLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		c.Set("logs", logger)

		c.Next()

		cost := time.Since(start)
		logger.Info(path,
			zap.Int("status", c.Writer.Status()),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.String("ip", c.ClientIP()),
			zap.String("user-agent", c.Request.UserAgent()),
			zap.String("errors", c.Errors.ByType(gin.ErrorTypePrivate).String()),
			zap.Duration("cost", cost),
		)
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
