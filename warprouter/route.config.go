package warprouter

import "github.com/gin-gonic/gin"

// ------------------------ Supporting Structs ------------------------

type HTTPMethod string

const (
	GET    HTTPMethod = "GET"
	POST   HTTPMethod = "POST"
	PUT    HTTPMethod = "PUT"
	DELETE HTTPMethod = "DELETE"
	PATCH  HTTPMethod = "PATCH"
)

type ParamType string

const (
	ParamString  ParamType = "string"
	ParamInteger ParamType = "integer"
	ParamBoolean ParamType = "boolean"
)

type QueryParam struct {
	Required bool      `json:"required"`
	Type     ParamType `json:"type"`
}

type ServiceConfig struct {
	Method      HTTPMethod `json:"method" validate:"required"`
	ServiceName string     `json:"serviceName" validate:"required"`
	Endpoint    string     `json:"endpoint" validate:"required"`
}

type RouteConfig struct {
	Endpoint              string                `json:"endpoint" validate:"required"`   // route path
	HTTPMethod            HTTPMethod            `json:"httpMethod" validate:"required"` // GET, POST, etc.
	Permission            string                `json:"permission" validate:"required"` // optional permission string
	Handler               gin.HandlerFunc       `json:"-"`                              // main handler
	Service               *ServiceConfig        `json:"service,omitempty"`              // service call (if no Handler)
	QueryStringParameters map[string]QueryParam `json:"queryStringParameters,omitempty"`
	RequestBodyRequired   bool                  `json:"requestBodyRequired,omitempty"`   // is request body required
	Middleware            []gin.HandlerFunc     `json:"-"` // pre-handler middleware
	PostMiddleware        []gin.HandlerFunc     `json:"-"` // post-handler middleware
}
