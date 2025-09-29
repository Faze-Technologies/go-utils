package middlewares

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Faze-Technologies/go-utils/cache"
	"github.com/Faze-Technologies/go-utils/request"
	"github.com/MicahParks/keyfunc"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
)

var jwks *keyfunc.JWKS

// ------------------------ JWKS Init ------------------------
// Call this once at app startup
func InitJWKS(auth0Domain string) {
	jwksURL := auth0Domain + ".well-known/jwks.json"
	var err error
	jwks, err = keyfunc.Get(jwksURL, keyfunc.Options{
		RefreshInterval: time.Hour,
	})
	if err != nil {
		panic("Failed to load JWKS: " + err.Error())
	}
}

// ------------------------ Authentication Middleware ------------------------
func AuthenticationAdminUser(ca *cache.Cache) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := c.GetHeader("x_auth_token")
		if tokenStr == "" {
			request.SendServiceError(c, &request.ServiceError{
				HttpStatus: http.StatusUnauthorized,
				ErrorCode:  "TOKEN_MISSING",
				Message:    "Missing authentication token",
			})
			c.Abort()
			return
		}

		token, err := jwt.Parse(tokenStr, jwks.Keyfunc)
		if err != nil || !token.Valid {
			request.SendServiceError(c, &request.ServiceError{
				HttpStatus: http.StatusUnauthorized,
				ErrorCode:  "INVALID_TOKEN",
				Message:    "Authentication failed",
			})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			request.SendServiceError(c, &request.ServiceError{
				HttpStatus: http.StatusUnauthorized,
				ErrorCode:  "INVALID_CLAIMS",
				Message:    "Invalid token claims",
			})
			c.Abort()
			return
		}

		// Extract userId
		userIdRaw, ok := claims["sub"]
		if !ok {
			request.SendServiceError(c, &request.ServiceError{
				HttpStatus: http.StatusUnauthorized,
				ErrorCode:  "INVALID_CLAIMS",
				Message:    "Missing subject claim",
			})
			c.Abort()
			return
		}
		userId := fmt.Sprintf("%v", userIdRaw)

		// Extract issuedAt (iat)
		var issuedAt int64
		switch v := claims["iat"].(type) {
		case float64:
			issuedAt = int64(v)
		case int64:
			issuedAt = v
		default:
			request.SendServiceError(c, &request.ServiceError{
				HttpStatus: http.StatusUnauthorized,
				ErrorCode:  "INVALID_CLAIMS",
				Message:    "Invalid iat claim",
			})
			c.Abort()
			return
		}

		// Check token revocation in Redis
		ctx := context.Background()
		storedIATStr, _ := ca.Get(ctx, "revoked:"+userId)
		if storedIATStr != "" {
			storedIATInt, err := strconv.ParseInt(storedIATStr, 10, 64)
			if err == nil && storedIATInt >= issuedAt {
				request.SendServiceError(c, &request.ServiceError{
					HttpStatus: http.StatusUnauthorized,
					ErrorCode:  "OLD_TOKEN",
					Message:    "Token is expired or revoked",
				})
				c.Abort()
				return
			}
		}

		// Normalize permissions
		var permissions []string
		if perms, ok := claims["access"]; ok {
			switch v := perms.(type) {
			case []interface{}:
				for _, p := range v {
					permissions = append(permissions, fmt.Sprintf("%v", p))
				}
			case []string:
				permissions = v
			case string:
				permissions = []string{v}
			}
		}

		// Set context values
		c.Set("userId", userId)
		c.Set("iat", issuedAt)
		c.Set("email", claims["email"])
		c.Set("permissions", permissions)
		c.Set("iss", claims["iss"])
		c.Request.Header.Set("userId", userId)
		c.Request.Header.Set("admin", "true")

		c.Next()
	}
}
