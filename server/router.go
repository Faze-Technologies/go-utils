package server

import (
	"github.com/Faze-Technologies/go-utils/config"
	"github.com/Faze-Technologies/go-utils/utils"
	"github.com/gin-gonic/gin"
	"net/http"
)

func GetGinRouter() (*gin.Engine, error) {
	env := config.GetString("environment")
	if env != "development" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}
	logger := utils.GetLogger()
	router := gin.New()
	router.UseH2C = true
	router.RemoveExtraSlash = true
	err := router.SetTrustedProxies([]string{"127.0.0.1"})
	if err != nil {
		return nil, err
	}
	router.Use(setResponseHeaders())
	router.Use(parseUserAgent())
	router.Use(ginLogger(logger), ginRecovery(logger))
	router.HandleMethodNotAllowed = true
	router.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"status": "error", "error": "methodNotAllowed"})
	})
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "error": "routeNotFound"})
	})
	return router, nil
}
