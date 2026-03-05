package logs

import (
	"context"
	"fmt"

	"github.com/Faze-Technologies/go-utils/config"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var logger *zap.Logger

func NewLogger() *zap.Logger {
	environment := config.GetString("environment")
	var err error
	if environment == "prod" {
		logger, err = zap.NewProduction()
	} else {
		logger, err = zap.NewDevelopment()
	}
	if err != nil {
		panic(fmt.Errorf("unable to initialize utils\n %w", err))
	}
	defer logger.Sync()

	logger = logger.WithOptions(zap.AddCaller(), zap.AddCallerSkip(0))
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
