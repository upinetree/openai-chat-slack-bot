package log

import (
	"os"

	"go.uber.org/zap"
)

var sugar *zap.SugaredLogger

func init() {
	var logger *zap.Logger
	var err error

	switch m := os.Getenv("MODE"); m {
	case "", "local":
		logger, err = zap.NewDevelopment()
	default:
		logger, err = zap.NewProduction()
	}
	if err != nil {
		panic(err)
	}

	// Skip this wrapper function from callers for backtraces visibility
	sugar = logger.WithOptions(zap.AddCallerSkip(1)).Sugar()
}

func Infof(template string, args ...interface{}) {
	sugar.Infof(template, args...)
}

func Warnf(template string, args ...interface{}) {
	sugar.Warnf(template, args...)
}

func Errorf(template string, args ...interface{}) {
	sugar.Errorf(template, args...)
}
