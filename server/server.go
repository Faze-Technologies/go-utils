package server

import (
	"github.com/Faze-Technologies/go-utils/config"
	"github.com/gin-gonic/gin"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"net/http"
	"time"
)

func GetGinServer(router *gin.Engine) *http.Server {
	port := config.GetString("server.port")
	readTimeout := config.GetInt("server.readTimeout")
	writeTimeout := config.GetInt("server.writeTimeout")
	h2s := &http2.Server{}
	return &http.Server{
		Addr:         port,
		Handler:      h2c.NewHandler(router, h2s),
		ReadTimeout:  time.Duration(readTimeout) * time.Second,
		WriteTimeout: time.Duration(writeTimeout) * time.Second,
	}
}
