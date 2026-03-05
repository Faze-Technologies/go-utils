package logs

import (
	"context"
	"fmt"
	"time"

	"github.com/Faze-Technologies/go-utils/config"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.Logger

var ist = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		return time.FixedZone("IST", 5*60*60+30*60)
	}
	return loc
}()

func NewLogger() *zap.Logger {
	environment := config.GetString("environment")

	var cfg zap.Config
	if environment == "prod" {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
	}

	cfg.EncoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.In(ist).Format("2006-01-02T15:04:05.000Z07:00"))
	}

	var err error
	logger, err = cfg.Build(zap.AddCaller())
	if err != nil {
		panic(fmt.Errorf("unable to initialize utils\n %w", err))
	}
	return logger
}

func GetLogger() *zap.Logger {
	return logger
}

// WithContext returns a logger with trace_id and span_id fields extracted
// from the OTel span in ctx. SigNoz uses these fields to correlate logs to traces.
func WithContext(ctx context.Context) *zap.Logger {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return logger
	}
	sc := span.SpanContext()
	return logger.With(
		zap.String("trace_id", sc.TraceID().String()),
		zap.String("span_id", sc.SpanID().String()),
		zap.String("trace_flags", sc.TraceFlags().String()),
	)
}
