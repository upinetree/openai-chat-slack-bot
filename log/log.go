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
	case "", "local": // TODO: debug mode => local mode
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

func Info(args ...interface{}) {
	sugar.Info(args...)
}

func Warn(args ...interface{}) {
	sugar.Warn(args...)
}

func Error(args ...interface{}) {
	sugar.Error(args...)
}
