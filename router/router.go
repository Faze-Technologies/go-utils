package router

import (
	"bytes"
	"io"
	"net/http"
	"strconv"

	"github.com/Faze-Technologies/go-utils/config"
	"github.com/Faze-Technologies/go-utils/request"
	"github.com/gin-gonic/gin"
)

// ------------------------ EnableRoutes ------------------------
func EnableRoutes(parent interface{}, routes []RouteConfig) {
	for _, route := range routes {
		// Wrap handler with query validation & post middleware
		handlerWrapper := func(rc RouteConfig) gin.HandlerFunc {
			return func(c *gin.Context) {
				// --- Inject permission into context ---
				if rc.Permission != "" {
					c.Set("permission", rc.Permission)
				}

				// --- Query param validation ---
				for name, param := range rc.QueryStringParameters {
					value := c.Query(name)
					if param.Required && value == "" {
						request.SendServiceError(c, &request.ServiceError{
							HttpStatus: http.StatusBadRequest,
							ErrorCode:  "MISSING_QUERY_PARAM",
							Message:    "missing query param: " + name,
						})
						return
					}
					if value != "" {
						switch param.Type {
						case ParamInteger:
							if _, err := strconv.Atoi(value); err != nil {
								request.SendServiceError(c, &request.ServiceError{
									HttpStatus: http.StatusBadRequest,
									ErrorCode:  "INVALID_QUERY_PARAM",
									Message:    "invalid integer query param: " + name,
								})
								return
							}
						case ParamBoolean:
							if value != "true" && value != "false" {
								request.SendServiceError(c, &request.ServiceError{
									HttpStatus: http.StatusBadRequest,
									ErrorCode:  "INVALID_QUERY_PARAM",
									Message:    "invalid boolean query param: " + name,
								})
								return
							}
						}
					}
				}

				// --- Request body required check (Handler or Service) ---
				if rc.RequestBodyRequired {
					var bodyBytes []byte
					if c.Request.Body != nil {
						bodyBytes, _ = io.ReadAll(c.Request.Body)
					}
					if len(bodyBytes) == 0 {
						request.SendServiceError(c, &request.ServiceError{
							HttpStatus: http.StatusBadRequest,
							ErrorCode:  "REQUEST_BODY_REQUIRED",
							Message:    "request body required",
						})
						return
					}
					// Re-attach body so handler/service can read it
					c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				}

				// --- Call handler or service ---
				if rc.Handler != nil {
					rc.Handler(c)
				} else if rc.Service != nil {
					forwardToService(c, rc.Service)
				} else {
					request.SendServiceError(c, &request.ServiceError{
						HttpStatus: http.StatusInternalServerError,
						ErrorCode:  "NO_HANDLER_SERVICE",
						Message:    "no handler or service defined",
					})
					return
				}

				// --- Post middleware ---
				for _, post := range rc.PostMiddleware {
					post(c)
				}
			}
		}(route)

		// --- Combine pre-middlewares + wrapper ---
		allHandlers := append(route.Middleware, handlerWrapper)

		// --- Register route ---
		switch t := parent.(type) {
		case *gin.Engine:
			registerRoute(t, route, allHandlers)
		case *gin.RouterGroup:
			registerRouteGroup(t, route, allHandlers)
		default:
			panic("unsupported parent type")
		}
	}
}

// ------------------------ Route Registration Helpers ------------------------
func registerRoute(r *gin.Engine, route RouteConfig, handlers []gin.HandlerFunc) {
	switch route.HTTPMethod {
	case GET:
		r.GET(route.Endpoint, handlers...)
	case POST:
		r.POST(route.Endpoint, handlers...)
	case PUT:
		r.PUT(route.Endpoint, handlers...)
	case DELETE:
		r.DELETE(route.Endpoint, handlers...)
	case PATCH:
		r.PATCH(route.Endpoint, handlers...)
	}
}

func registerRouteGroup(rg *gin.RouterGroup, route RouteConfig, handlers []gin.HandlerFunc) {
	switch route.HTTPMethod {
	case GET:
		rg.GET(route.Endpoint, handlers...)
	case POST:
		rg.POST(route.Endpoint, handlers...)
	case PUT:
		rg.PUT(route.Endpoint, handlers...)
	case DELETE:
		rg.DELETE(route.Endpoint, handlers...)
	case PATCH:
		rg.PATCH(route.Endpoint, handlers...)
	}
}

// ------------------------ Service Forwarder ------------------------
func forwardToService(c *gin.Context, svc *ServiceConfig) {
	// Get base URL dynamically from config package
	baseURL := config.GetServiceURL(svc.ServiceName)
	if baseURL == "" {
		request.SendServiceError(c, &request.ServiceError{
			HttpStatus: http.StatusInternalServerError,
			ErrorCode:  "UNKNOWN_SERVICE",
			Message:    "unknown service: " + svc.ServiceName,
		})
		return
	}

	fullURL := baseURL + svc.Endpoint

	var bodyBytes []byte
	if c.Request.Body != nil {
		bodyBytes, _ = io.ReadAll(c.Request.Body)
	}

	req, err := http.NewRequest(string(svc.Method), fullURL, bytes.NewReader(bodyBytes))
	if err != nil {
		request.SendServiceError(c, &request.ServiceError{
			HttpStatus: http.StatusInternalServerError,
			ErrorCode:  "SERVICE_REQUEST_ERROR",
			Message:    "failed to create service request",
		})
		return
	}

	// Copy headers
	for k, v := range c.Request.Header {
		for _, val := range v {
			req.Header.Add(k, val)
		}
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		request.SendServiceError(c, &request.ServiceError{
			HttpStatus: http.StatusBadGateway,
			ErrorCode:  "SERVICE_CALL_FAILED",
			Message:    err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
}
