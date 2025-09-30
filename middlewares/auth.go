package middlewares

import (
	"context"
	"net/http"

	"github.com/Faze-Technologies/go-utils/config"
	"github.com/Faze-Technologies/go-utils/logs"
	"github.com/Faze-Technologies/go-utils/request"
	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
	"github.com/golang-jwt/jwt/v4"
	"go.uber.org/zap"
)

type UserDetails struct {
	Id            string                 `json:"id"`
	Email         string                 `json:"email"`
	EmailVerified bool                   `json:"emailVerified"`
	HasPassword   bool                   `json:"hasPassword"`
	Mobile        string                 `json:"mobile"`
	GoogleId      string                 `json:"googleId"`
	TwitterId     string                 `json:"twitterId"`
	ExternalId    string                 `json:"externalId"`
	MfaMethod     string                 `json:"mfaMethod"`
	Metadata      map[string]interface{} `json:"metadata"`
	CreatedAt     string                 `json:"createdAt"`
	UpdatedAt     string                 `json:"updatedAt"`
}

func GetAuthUser(c *gin.Context) (*UserDetails, *request.ServiceError) {
	user, ok := c.Request.Context().Value("user").(UserDetails)
	if !ok {
		return nil, request.CreateUnauthorizedError(nil, "User is not authenticated")
	}
	return &user, nil
}

func verifyTokenSignature(token string) (*jwt.Token, *request.ServiceError) {
	logger := logs.GetLogger()
	publicKey := config.GetString("auth_public_key")
	publicKeyBytes := []byte(publicKey)

	publicKeyParsed, err := jwt.ParseRSAPublicKeyFromPEM(publicKeyBytes)
	if err != nil {
		logger.Error("Error parsing public key", zap.Error(err))
		return nil, request.CreateInternalServerError(err)
	}

	parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return publicKeyParsed, nil
	})
	if err != nil {
		logger.Error("Error parsing token", zap.Error(err))
		return nil, request.CreateUnauthorizedError(err, "Invalid Access Token")
	}
	if !parsedToken.Valid {
		return nil, request.CreateUnauthorizedError(err, "Invalid Access Token")
	}
	return parsedToken, nil
}

func AuthenticateUser(c *gin.Context) {
	accessToken := c.Request.Header.Get("Authorization")
	if accessToken == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	token, sErr := verifyTokenSignature(accessToken)
	if sErr != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	jwtClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	claimsData, ok := jwtClaims["data"]
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	jsonBytes, err := json.Marshal(claimsData)
	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	var user UserDetails
	err = json.Unmarshal(jsonBytes, &user)
	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	ctx := context.WithValue(c.Request.Context(), "user", user)
	c.Request = c.Request.WithContext(ctx)
	c.Next()
}
