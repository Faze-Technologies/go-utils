package middlewares

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Map HTTP methods to access operations
var MethodAccessMap = map[string]string{
	"POST":   "c",
	"PUT":    "u",
	"PATCH":  "u",
	"GET":    "r",
	"FETCH":  "r",
	"DELETE": "d",
}

// AuthorizationConfig holds global flags
type AuthorizationConfig struct {
	DisableAuth0     bool
	MatchMethodLevel bool
}

// Global config (can be loaded from env/config)
var AuthConfig = AuthorizationConfig{
	DisableAuth0:     false,
	MatchMethodLevel: true,
}

// AuthorizationAdminUser middleware
func AuthorizationAdminUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		if AuthConfig.DisableAuth0 {
			c.Next()
			return
		}

		// Route permission (set earlier in route config)
		apiPermissionRaw, exists := c.Get("route_permission")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"message": "No required user permission defined to access API Route",
			})
			return
		}
		apiPermission, ok := apiPermissionRaw.(string)
		if !ok || apiPermission == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"message": "Invalid route permission format",
			})
			return
		}

		// User permissions (set by Authentication middleware)
		userPermissionsRaw, exists := c.Get("permissions")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"message": "User does not have any permissions associated. Please ask your admin to provide access",
			})
			return
		}

		userPermissions, ok := userPermissionsRaw.([]string)
		if !ok || len(userPermissions) == 0 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"message": "User does not have any permissions associated. Please ask your admin to provide access",
			})
			return
		}

		splitPermission := strings.Split(apiPermission, "::")
		if len(splitPermission) < 3 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"message": "Permission format invalid. Must be <prefix>::<module>::<feature>::<operation>",
			})
			return
		}

		module, feature, operation := splitPermission[0], splitPermission[1], splitPermission[2]

		// Match method level
		if AuthConfig.MatchMethodLevel {
			methodOp, ok := MethodAccessMap[strings.ToUpper(c.Request.Method)]
			if ok && methodOp != operation {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"message": "HTTP method and access operation do not match",
				})
				return
			}
		}

		// If starts with $
		if strings.HasPrefix(apiPermission, "$") {
			if !contains(userPermissions, apiPermission) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"message": "User is not authorized to perform this operation. Fixed permission not provided",
				})
				return
			}
			c.Set("userType", "ADMIN")
			c.Next()
			return
		}

		// Match against user permissions
		matchPermissions := [][]string{}
		for _, p := range userPermissions {
			parts := strings.Split(p, "::")
			if len(parts) < 4 {
				continue
			}
			// Format: <+|->::<module>::<feature>::<operation>
			prefix, m, f, o := parts[0], parts[1], parts[2], parts[3]

			if (m == module || m == "*") &&
				(f == feature || f == "*") &&
				(o == operation || o == "*") {
				matchPermissions = append(matchPermissions, []string{prefix, m, f, o})
			}
		}

		if len(matchPermissions) == 0 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"message": "User is not authorized to perform this operation. Permission not provided",
			})
			return
		}

		// Check deny rules
		for _, mp := range matchPermissions {
			if mp[0] == "-" {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"message": "User is not authorized to perform this operation. Permission denied",
				})
				return
			}
		}

		// Passed authorization
		c.Set("userType", "ADMIN")
		c.Next()
	}
}

// Helper
func contains(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}
