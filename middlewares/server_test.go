package middlewares

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Faze-Technologies/go-utils/logs"
	"github.com/gin-gonic/gin"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/zap"
)

const (
	testEmail  = "jane.doe@example.com"
	testMobile = "+919999900000"
)

// runAuthenticatedRequest sends one authenticated request through GinLogger and
// returns the span it was recorded on.
func runAuthenticatedRequest(t *testing.T, target string) sdktrace.ReadOnlySpan {
	t.Helper()
	gin.SetMode(gin.TestMode)
	logs.NewLogger() // GinLogger logs through the package logger

	recorder := tracetest.NewSpanRecorder()
	tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)).Tracer("test")

	router := gin.New()
	router.Use(func(c *gin.Context) {
		ctx, span := tracer.Start(c.Request.Context(), "request")
		defer span.End()
		user := UserDetails{Id: "user-1", Email: testEmail, Mobile: testMobile, KycStatus: true, KycCountry: "IN"}
		c.Request = c.Request.WithContext(context.WithValue(ctx, userContextKey, user))
		c.Next()
	})
	router.Use(GinLogger(zap.NewNop()))
	router.POST("/orders", func(c *gin.Context) { c.Status(http.StatusOK) })

	body := strings.NewReader(`{"email":"` + testEmail + `","mobile":"` + testMobile + `","amount":5}`)
	req := httptest.NewRequest(http.MethodPost, target, body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(httptest.NewRecorder(), req)

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 ended span, got %d", len(spans))
	}
	return spans[0]
}

func TestGinLoggerKeepsPersonalDataOffSpan(t *testing.T) {
	span := runAuthenticatedRequest(t, "/orders?email="+testEmail+"&mobile="+testMobile+"&page=2")

	attrs := map[string]string{}
	for _, kv := range span.Attributes() {
		attrs[string(kv.Key)] = kv.Value.Emit()
	}
	if attrs["user.id"] != "user-1" {
		t.Errorf("user.id should stay on the span, got %q", attrs["user.id"])
	}
	for _, key := range []string{"user.email", "user.mobile"} {
		if _, found := attrs[key]; found {
			t.Errorf("span has %s attribute", key)
		}
	}

	// No attribute or event may carry the values either (request body, query, params).
	var everything []string
	for _, v := range attrs {
		everything = append(everything, v)
	}
	for _, event := range span.Events() {
		for _, kv := range event.Attributes {
			everything = append(everything, kv.Value.Emit())
		}
	}
	joined := strings.Join(everything, " ")
	for _, secret := range []string{testEmail, testMobile} {
		if strings.Contains(joined, secret) {
			t.Errorf("span carries %q", secret)
		}
	}
	if !strings.Contains(joined, "page=2") {
		t.Errorf("non-sensitive query values should be kept, got %q", joined)
	}
}

func TestRedactQuery(t *testing.T) {
	got := redactQuery("token=abc&Email=a%40b.com&page=2")
	for _, leaked := range []string{"abc", "a%40b.com"} {
		if strings.Contains(got, leaked) {
			t.Errorf("redactQuery leaked %q: %s", leaked, got)
		}
	}
	if !strings.Contains(got, "page=2") {
		t.Errorf("redactQuery dropped page=2: %s", got)
	}
}

func TestDumpRequestWithoutCredentials(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	req.Header.Set("Cookie", "session=secret-cookie")
	req.Header.Set("appid", "web")

	dump := string(dumpRequestWithoutCredentials(req))
	if strings.Contains(dump, "secret-token") || strings.Contains(dump, "secret-cookie") {
		t.Errorf("dump leaked credentials: %s", dump)
	}
	if !strings.Contains(dump, "Appid: web") {
		t.Errorf("dump dropped other headers: %s", dump)
	}
	if req.Header.Get("Authorization") != "Bearer secret-token" {
		t.Error("original request headers were modified")
	}
}

func TestRedactBodyMasksPersonalAndCamelCaseKeys(t *testing.T) {
	got := redactBody([]byte(`{"email":"` + testEmail + `","cardNumber":"4111","amount":5}`))
	if strings.Contains(got, testEmail) || strings.Contains(got, "4111") {
		t.Errorf("redactBody leaked values: %s", got)
	}
	if !strings.Contains(got, `"amount":5`) {
		t.Errorf("redactBody dropped amount: %s", got)
	}
}
