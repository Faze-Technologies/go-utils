package logs

import (
	"fmt"
	"github.com/Faze-Technologies/go-utils/config"

	"go.uber.org/zap"
)

var logger *zap.Logger

func NewLogger() *zap.Logger {
	environment := config.GetString("environment")
	var err error
	if environment != "development" {
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
