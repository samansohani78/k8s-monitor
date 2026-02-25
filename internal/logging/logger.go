/*
Copyright 2026 K8sWatch.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package logging

import (
	"context"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config holds logging configuration
type Config struct {
	// Level is the log level (debug, info, warn, error)
	Level string
	// Format is the log format (json, console)
	Format string
	// Component is the component name (agent, aggregator, alertmanager)
	Component string
	// AddTimestamp adds timestamp to logs
	AddTimestamp bool
	// AddCaller adds caller info to logs
	AddCaller bool
}

// DefaultConfig returns default logging configuration
func DefaultConfig() *Config {
	return &Config{
		Level:        "info",
		Format:       "json",
		Component:    "",
		AddTimestamp: true,
		AddCaller:    false,
	}
}

// NewLogger creates a new structured logger with the given configuration
func NewLogger(cfg *Config) (logr.Logger, error) {
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		level = zapcore.InfoLevel
	}

	zapConfig := zap.Config{
		Level:             zap.NewAtomicLevelAt(level),
		DisableCaller:     !cfg.AddCaller,
		DisableStacktrace: true,
		Encoding:          cfg.Format,
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "timestamp",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "message",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	zapLogger, err := zapConfig.Build()
	if err != nil {
		return logr.Discard(), err
	}

	// Add component field if specified
	if cfg.Component != "" {
		zapLogger = zapLogger.With(zap.String("component", cfg.Component))
	}

	return zapr.NewLogger(zapLogger), nil
}

// NewProductionLogger creates a production-ready logger
func NewProductionLogger(component string) (logr.Logger, error) {
	cfg := DefaultConfig()
	cfg.Component = component
	cfg.Format = "json"
	return NewLogger(cfg)
}

// NewDevelopmentLogger creates a development-friendly logger
func NewDevelopmentLogger(component string, verbose bool) (logr.Logger, error) {
	cfg := DefaultConfig()
	cfg.Component = component
	cfg.Format = "console"
	if verbose {
		cfg.Level = "debug"
		cfg.AddCaller = true
	}
	return NewLogger(cfg)
}

// ContextLogger provides logging with correlation ID support
type ContextLogger struct {
	logger logr.Logger
	gen    *CorrelationIDGenerator
}

// NewContextLogger creates a new context logger
func NewContextLogger(logger logr.Logger) *ContextLogger {
	return &ContextLogger{
		logger: logger,
		gen:    NewCorrelationIDGenerator(),
	}
}

// StartOperation starts a new operation with a correlation ID
func (l *ContextLogger) StartOperation(ctx context.Context, operation string, fields ...interface{}) (context.Context, *OperationLogger) {
	ctx, correlationID := l.gen.GenerateOrExtract(ctx)
	opLogger := &OperationLogger{
		logger:        l.logger.WithValues(append([]interface{}{"correlationID", correlationID, "operation", operation}, fields...)...),
		correlationID: correlationID,
		operation:     operation,
		startTime:     time.Now(),
	}
	opLogger.logger.Info("operation started")
	return ctx, opLogger
}

// WithContext returns a logger with correlation ID from context
func (l *ContextLogger) WithContext(ctx context.Context) logr.Logger {
	return WithContext(l.logger, ctx)
}

// OperationLogger tracks a single operation
type OperationLogger struct {
	logger        logr.Logger
	correlationID string
	operation     string
	startTime     time.Time
}

// End ends the operation and logs duration
func (ol *OperationLogger) End(fields ...interface{}) {
	duration := time.Since(ol.startTime)
	args := append([]interface{}{"duration", duration}, fields...)
	ol.logger.Info("operation completed", args...)
}

// EndWithError ends the operation with an error
func (ol *OperationLogger) EndWithError(err error, fields ...interface{}) {
	duration := time.Since(ol.startTime)
	args := append([]interface{}{"duration", duration, "error", err}, fields...)
	ol.logger.Error(err, "operation failed", args...)
}

// StandardLogger wraps logr.Logger with standard logger methods
type StandardLogger struct {
	logger logr.Logger
}

// NewStandardLogger creates a new standard logger wrapper
func NewStandardLogger(logger logr.Logger) *StandardLogger {
	return &StandardLogger{logger: logger}
}

// Info logs an info message
func (l *StandardLogger) Info(msg string, keysAndValues ...interface{}) {
	l.logger.Info(msg, keysAndValues...)
}

// Error logs an error message
func (l *StandardLogger) Error(msg string, keysAndValues ...interface{}) {
	l.logger.Error(nil, msg, keysAndValues...)
}

// Debug logs a debug message
func (l *StandardLogger) Debug(msg string, keysAndValues ...interface{}) {
	l.logger.V(1).Info(msg, keysAndValues...)
}

// WithValues returns a new logger with additional key-value pairs
func (l *StandardLogger) WithValues(keysAndValues ...interface{}) *StandardLogger {
	return &StandardLogger{
		logger: l.logger.WithValues(keysAndValues...),
	}
}

// GetHostname returns the current hostname for logging
func GetHostname() string {
	if hostname, err := os.Hostname(); err == nil {
		return hostname
	}
	return "unknown"
}
