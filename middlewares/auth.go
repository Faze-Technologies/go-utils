package middlewares

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Faze-Technologies/go-utils/cache"
	"github.com/Faze-Technologies/go-utils/config"
	"github.com/Faze-Technologies/go-utils/logs"
	"github.com/Faze-Technologies/go-utils/request"
	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
	"github.com/goccy/go-json"
	"github.com/golang-jwt/jwt/v4"
	"go.uber.org/zap"
)

type Middlewares struct {
	Cache  *cache.Cache
	Logger *zap.Logger
}

func InitializeMiddlewares(cache *cache.Cache, logger *zap.Logger) *Middlewares {
	return &Middlewares{
		Cache:  cache,
		Logger: logger,
	}
}

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const userContextKey contextKey = "user"

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
	KycStatus     bool                   `json:"kycStatus"`
	CreatedAt     string                 `json:"createdAt"`
	UpdatedAt     string                 `json:"updatedAt"`
	Segments      []string               `json:"segments"`
}

func GetAuthUser(c *gin.Context) (*UserDetails, *request.ServiceError) {
	user, ok := c.Request.Context().Value(userContextKey).(UserDetails)
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

type KYCAPIResponse struct {
	Success   bool                   `json:"success"`
	ErrorCode int                    `json:"error_code"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data"`
}

func (m *Middlewares) verifyKYCStatus(ctx context.Context, userId string, ip string, tokenKycStatus bool) (bool, error) {
	logger := m.Logger

	// If KYC is true in token, allow immediately
	if tokenKycStatus {
		return true, nil
	}

	// Check Redis cache
	cacheKey := fmt.Sprintf("kyc:status:%s", userId)
	cachedStatus, err := m.Cache.Get(ctx, cacheKey)

	if err == nil && cachedStatus != "" {
		// Cache hit - return cached value
		logger.Debug("KYC status found in cache", zap.String("userId", userId), zap.String("status", cachedStatus))
		return cachedStatus == "true", nil
	}

	// Cache miss - call KYC API
	logger.Debug("KYC status not in cache, calling API", zap.String("userId", userId))
	baseURL := config.GetServiceURL("kycService")
	url := fmt.Sprintf("%s/kyc/getKycStatusAndCountry", baseURL)

	client := resty.New().
		SetTimeout(10*time.Second).
		SetHeader("Content-Type", "application/json")

	var kycResponse KYCAPIResponse

	resp, err := client.R().
		SetContext(ctx).
		SetQueryParam("id", userId).
		SetQueryParam("ip", ip).
		SetResult(&kycResponse).
		Get(url)

	if err != nil {
		logger.Error("Request to KYC service failed", zap.String("url", url), zap.String("userId", userId), zap.Error(err))
		return false, fmt.Errorf("request failed: %w", err)
	}

	if !resp.IsSuccess() {
		logger.Error("HTTP error from KYC service", zap.Int("statusCode", resp.StatusCode()), zap.String("body", resp.String()))
		return false, fmt.Errorf("HTTP %d: %s", resp.StatusCode(), resp.String())
	}

	// Check if status or verified field is "completed"
	kycData := kycResponse.Data
	status, statusOk := kycData["status"].(string)
	verified, verifiedOk := kycData["verified"].(string)

	isVerified := false
	if (statusOk && status == "completed") || (verifiedOk && verified == "completed") {
		isVerified = true
		logger.Info("KYC verified user", zap.String("userId", userId))
	} else {
		logger.Info("KYC not verified", zap.String("userId", userId), zap.String("status", status), zap.String("verified", verified))
	}

	// Cache the result
	var cacheDuration time.Duration
	if isVerified {
		cacheDuration = 2 * time.Hour
	} else {
		cacheDuration = 5 * time.Minute
	}

	statusStr := "false"
	if isVerified {
		statusStr = "true"
	}

	err = m.Cache.Set(ctx, cacheKey, statusStr, cacheDuration)
	if err != nil {
		logger.Error("Error caching KYC status", zap.Error(err))
		// Don't fail the request if caching fails
	}

	logger.Info("KYC status fetched and cached",
		zap.String("userId", userId),
		zap.Bool("kycStatus", isVerified),
		zap.Duration("cacheDuration", cacheDuration))

	return isVerified, nil
}

type SegmentsAPIResponse struct {
	Success   bool                   `json:"success"`
	ErrorCode int                    `json:"error_code"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data"`
}

func (m *Middlewares) fetchUserSegments(ctx context.Context, userId string) []string {
	base := config.GetServiceURL("superteamSegmentationService")
	url := fmt.Sprintf("%s/api/v1/segments/users/segmentsOfUser", base)
	client := resty.New().
		SetTimeout(2*time.Second).
		SetHeader("Content-Type", "application/json")
	var resp SegmentsAPIResponse
	r, err := client.R().
		SetContext(ctx).
		SetResult(&resp).
		SetQueryParam("userId", userId).
		Get(url)
	if err != nil || !r.IsSuccess() {
		if err != nil {
			m.Logger.Error("Segments fetch failed", zap.String("userId", userId), zap.Error(err))
		} else {
			m.Logger.Warn("Segments API non-success", zap.String("userId", userId), zap.Int("statusCode", r.StatusCode()))
		}
		return []string{}
	}
	data := resp.Data
	segments := make([]string, 0)
	if arr, ok := data["segments"].([]interface{}); ok {
		for _, v := range arr {
			if s, ok := v.(string); ok {
				segments = append(segments, s)
			}
		}
	}
	return segments
}

func (m *Middlewares) AuthenticateUser(c *gin.Context) {
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

	// Extract client IP address
	clientIP := c.ClientIP()

	// Verify KYC status with caching
	verifiedKycStatus, err := m.verifyKYCStatus(c.Request.Context(), user.Id, clientIP, user.KycStatus)
	if err != nil {
		m.Logger.Error("Error verifying KYC status", zap.String("userId", user.Id), zap.Error(err))
		// Continue with token KYC status on error
		verifiedKycStatus = user.KycStatus
	}

	// Update user KYC status with verified value
	user.KycStatus = verifiedKycStatus
	user.Segments = m.fetchUserSegments(c.Request.Context(), user.Id)

	ctx := context.WithValue(c.Request.Context(), userContextKey, user)
	c.Request = c.Request.WithContext(ctx)
	c.Next()
}
