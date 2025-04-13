package logs

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Global logging state
var (
	logger  *zap.SugaredLogger
	Options = struct {
		Verbose     bool
		ProjectName string
		Version     string
		Environment string
	}{
		ProjectName: "project", // default
	}

	initOnce sync.Once
)

// Logger returns the global zap.SugaredLogger instance.
// If it's nil, InitLogger is called automatically.
func Logger() *zap.SugaredLogger {
	if logger == nil {
		InitLogger(os.Getenv("ENV"))
	}
	return logger
}

// InitLogger configures the global logger based on environment & LOG_FMT overrides.
// If environment is 'development' or 'dev', default to text console logs unless overridden.
// If environment is 'production' or ”, default to JSON unless overridden.
// LOG_FMT can be 'json', 'formatted', or 'text'.
func InitLogger(env string) {
	initOnce.Do(func() {
		if env == "" || strings.EqualFold(env, "production") {
			env = "production"
		} else if strings.EqualFold(env, "dev") {
			env = "development"
		}
		Options.Environment = env

		format := os.Getenv("LOG_FMT") // user override
		if format == "" {
			if env == "development" {
				format = "text"
			} else {
				format = "json"
			}
		}

		var cfg zap.Config
		if format == "text" {
			cfg = zap.NewDevelopmentConfig()
			cfg.Encoding = "console"
		} else {
			// 'json' or 'formatted' => base is ProductionConfig
			cfg = zap.NewProductionConfig()
			cfg.Encoding = "json"
			if format == "formatted" {
				// Example of a more pretty JSON
				cfg.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
				cfg.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
				cfg.EncoderConfig.EncodeDuration = zapcore.StringDurationEncoder
			}
		}

		// Common settings
		cfg.OutputPaths = []string{"stdout"}
		cfg.ErrorOutputPaths = []string{"stderr"}

		if Options.Verbose {
			// Make logs more verbose. For JSON, might do debug-level.
			// For console, we already have stacktraces on error, etc.
			cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
		}

		// Add app/version/env fields in each log line
		log, err := cfg.Build(zap.Fields(
			zap.String("app", Options.ProjectName),
			zap.String("version", Options.Version),
			zap.String("env", Options.Environment),
		))
		if err != nil {
			// Fallback to a no-op logger or panic
			fmt.Println("Failed to init logger:", err)
			return
		}

		logger = log.Sugar()
	})
}

// VLogf is a convenience for verbose console prints (not structured).
// Typically used for very chatty debugging messages that don't belong in structured logs.
func VLogf(format string, args ...interface{}) {
	if Options.Verbose {
		fmt.Printf("[verbose] "+format+"\n", args...)
	}
}
