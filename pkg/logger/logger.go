// Copyright Jetstack Ltd. See LICENSE for details.

package logger

import (
	"sync"

	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/klog/v2"
)

var (
	Logger *zap.Logger
	once   sync.Once
)

func init() {
	once.Do(func() {
		config := zap.NewProductionConfig()
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		var err error
		Logger, err = config.Build()
		if err != nil {
			panic(err)
		}
		// Redirect klog to zap by default
		klog.SetLogger(zapr.NewLogger(Logger))
	})
}

// Init initializes the global logger with the specified log level.
// Supported levels: debug, info, warn, error, fatal
func Init(level string) {
	config := zap.NewProductionConfig()
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
		zapLevel = zapcore.InfoLevel
	}
	config.Level = zap.NewAtomicLevelAt(zapLevel)

	var err error
	Logger, err = config.Build(zap.AddCallerSkip(1))
	if err != nil {
		panic(err)
	}

	// Redirect klog to the newly initialized zap logger
	klog.SetLogger(zapr.NewLogger(Logger))
}
