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
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigDefaults(t *testing.T) {
	cfg := DefaultConfig()
	assert.NotNil(t, cfg)
	assert.Equal(t, "info", cfg.Level)
	assert.Equal(t, "json", cfg.Format)
	assert.Empty(t, cfg.Component)
	assert.True(t, cfg.AddTimestamp)
	assert.False(t, cfg.AddCaller)
}

func TestNewLoggerInvalidLevel(t *testing.T) {
	cfg := &Config{
		Level:  "invalid-level",
		Format: "json",
	}

	logger, err := NewLogger(cfg)
	assert.NoError(t, err)
	assert.True(t, logger.Enabled()) // Default to info level
}

func TestNewLoggerJSONFormat(t *testing.T) {
	cfg := &Config{
		Level:     "info",
		Format:    "json",
		Component: "test-component",
	}

	logger, err := NewLogger(cfg)
	require.NoError(t, err)
	assert.True(t, logger.Enabled())
}

func TestNewLoggerConsoleFormat(t *testing.T) {
	cfg := &Config{
		Level:     "debug",
		Format:    "console",
		Component: "test-component",
		AddCaller: true,
	}

	logger, err := NewLogger(cfg)
	require.NoError(t, err)
	assert.True(t, logger.Enabled())
}

func TestNewProductionLogger(t *testing.T) {
	logger, err := NewProductionLogger("test-agent")
	require.NoError(t, err)
	assert.True(t, logger.Enabled())
}

func TestNewDevelopmentLogger(t *testing.T) {
	logger, err := NewDevelopmentLogger("test-agent", true)
	require.NoError(t, err)
	assert.True(t, logger.Enabled())
}

func TestNewDevelopmentLoggerVerbose(t *testing.T) {
	cfg := &Config{
		Level:     "debug",
		Format:    "console",
		Component: "test-agent",
		AddCaller: true,
	}

	logger, err := NewLogger(cfg)
	require.NoError(t, err)
	assert.True(t, logger.Enabled())
}

func TestCorrelationIDGenerator(t *testing.T) {
	gen := NewCorrelationIDGenerator()
	assert.NotNil(t, gen)

	id1 := gen.Generate()
	id2 := gen.Generate()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
}

func TestCorrelationIDGeneratorGenerateOrExtract(t *testing.T) {
	gen := NewCorrelationIDGenerator()
	ctx := context.Background()

	// Generate new correlation ID
	newCtx, id1 := gen.GenerateOrExtract(ctx)
	assert.NotEmpty(t, id1)

	// Extract existing correlation ID
	newCtx2, id2 := gen.GenerateOrExtract(newCtx)
	assert.Equal(t, id1, id2)
	assert.Equal(t, newCtx, newCtx2)
}

func TestCorrelationIDGeneratorGenerateOrExtractWithExisting(t *testing.T) {
	gen := NewCorrelationIDGenerator()
	ctx := context.WithValue(context.Background(), CorrelationIDKey, "existing-id")

	newCtx, id := gen.GenerateOrExtract(ctx)
	assert.Equal(t, "existing-id", id)
	assert.Equal(t, ctx, newCtx)
}

func TestFromContext(t *testing.T) {
	ctx := context.Background()

	// No correlation ID
	id := FromContext(ctx)
	assert.Empty(t, id)

	// With correlation ID
	ctx = context.WithValue(ctx, CorrelationIDKey, "test-correlation-id")
	id = FromContext(ctx)
	assert.Equal(t, "test-correlation-id", id)
}

func TestFromContextInvalidType(t *testing.T) {
	// Wrong type for correlation ID
	ctx := context.WithValue(context.Background(), CorrelationIDKey, 12345)
	id := FromContext(ctx)
	assert.Empty(t, id)
}

func TestWithContext(t *testing.T) {
	baseLogger := logr.Discard()

	ctx := context.WithValue(context.Background(), CorrelationIDKey, "test-correlation-id")
	logger := WithContext(baseLogger, ctx)

	assert.False(t, logger.Enabled())
}

func TestWithContextNoCorrelationID(t *testing.T) {
	baseLogger := logr.Discard()

	ctx := context.Background()
	logger := WithContext(baseLogger, ctx)

	assert.False(t, logger.Enabled())
}

func TestNewContextWithCorrelationID(t *testing.T) {
	ctx := context.Background()
	newCtx := NewContextWithCorrelationID(ctx, "test-id")

	id := FromContext(newCtx)
	assert.Equal(t, "test-id", id)
}

func TestContextLogger(t *testing.T) {
	baseLogger := logr.Discard()

	ctxLogger := NewContextLogger(baseLogger)
	assert.NotNil(t, ctxLogger)

	ctx := context.Background()
	opCtx, opLogger := ctxLogger.StartOperation(ctx, "test-operation", "key1", "value1")

	assert.NotNil(t, opCtx)
	assert.NotNil(t, opLogger)
}

func TestContextLoggerWithExistingCorrelationID(t *testing.T) {
	baseLogger := logr.Discard()

	ctxLogger := NewContextLogger(baseLogger)

	ctx := context.WithValue(context.Background(), CorrelationIDKey, "existing-id")
	opCtx, opLogger := ctxLogger.StartOperation(ctx, "test-operation")

	assert.NotNil(t, opCtx)
	assert.NotNil(t, opLogger)

	// Should use existing correlation ID
	id := FromContext(opCtx)
	assert.Equal(t, "existing-id", id)
}

func TestOperationLoggerEnd(t *testing.T) {
	baseLogger := logr.Discard()

	ctxLogger := NewContextLogger(baseLogger)
	ctx := context.Background()
	_, opLogger := ctxLogger.StartOperation(ctx, "test-operation")

	opLogger.End("result", "success")
}

func TestOperationLoggerEndWithError(t *testing.T) {
	baseLogger := logr.Discard()

	ctxLogger := NewContextLogger(baseLogger)
	ctx := context.Background()
	_, opLogger := ctxLogger.StartOperation(ctx, "test-operation")

	testErr := assert.AnError
	opLogger.EndWithError(testErr, "result", "failure")
}

func TestContextLoggerWithContext(t *testing.T) {
	baseLogger := logr.Discard()

	ctxLogger := NewContextLogger(baseLogger)
	ctx := context.WithValue(context.Background(), CorrelationIDKey, "test-id")

	logger := ctxLogger.WithContext(ctx)
	assert.False(t, logger.Enabled())
}

func TestStandardLogger(t *testing.T) {
	baseLogger := logr.Discard()

	stdLogger := NewStandardLogger(baseLogger)
	assert.NotNil(t, stdLogger)

	stdLogger.Info("test info message", "key", "value")
}

func TestStandardLoggerError(t *testing.T) {
	baseLogger := logr.Discard()

	stdLogger := NewStandardLogger(baseLogger)

	stdLogger.Error("test error message", "key", "value")
}

func TestStandardLoggerDebug(t *testing.T) {
	baseLogger := logr.Discard()

	stdLogger := NewStandardLogger(baseLogger)

	stdLogger.Debug("test debug message", "key", "value")
}

func TestStandardLoggerWithValues(t *testing.T) {
	baseLogger := logr.Discard()

	stdLogger := NewStandardLogger(baseLogger)
	stdLoggerWithValues := stdLogger.WithValues("common", "value")

	stdLoggerWithValues.Info("test message")
}

func TestGetHostname(t *testing.T) {
	hostname := GetHostname()
	assert.NotEmpty(t, hostname)
}

func TestCorrelationIDKey(t *testing.T) {
	// Verify the context key type
	key := CorrelationIDKey
	assert.Equal(t, contextKey("correlationID"), key)
}

func TestLoggerConcurrentAccess(t *testing.T) {
	baseLogger := logr.Discard()

	ctxLogger := NewContextLogger(baseLogger)
	ctx := context.Background()

	done := make(chan bool, 10)

	// Start multiple goroutines accessing the logger concurrently
	for i := 0; i < 10; i++ {
		go func(id int) {
			opCtx, opLogger := ctxLogger.StartOperation(ctx, "concurrent-operation")
			opLogger.End("goroutine", id)
			_ = opCtx
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name   string
		cfg    *Config
		valid  bool
		reason string
	}{
		{
			name: "Valid JSON config",
			cfg: &Config{
				Level:  "info",
				Format: "json",
			},
			valid: true,
		},
		{
			name: "Valid console config",
			cfg: &Config{
				Level:  "debug",
				Format: "console",
			},
			valid: true,
		},
		{
			name:  "Empty config fails validation",
			cfg:   &Config{},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := NewLogger(tt.cfg)
			if tt.valid {
				assert.NoError(t, err)
				assert.True(t, logger.Enabled())
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestLoggerLevels(t *testing.T) {
	tests := []struct {
		level         string
		expectEnabled bool
	}{
		{"debug", true},
		{"info", true},
		{"warn", false}, // logger.Enabled() checks if info level is enabled
		{"error", false},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			cfg := &Config{
				Level:  tt.level,
				Format: "json",
			}

			logger, err := NewLogger(cfg)
			require.NoError(t, err)
			assert.Equal(t, tt.expectEnabled, logger.Enabled())
		})
	}
}

func TestLoggerInvalidFormat(t *testing.T) {
	cfg := &Config{
		Level:  "info",
		Format: "invalid-format",
	}

	// Should fail with invalid format
	_, err := NewLogger(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no encoder registered")
}

func TestOperationLoggerEndConcurrent(t *testing.T) {
	baseLogger := logr.Discard()

	ctxLogger := NewContextLogger(baseLogger)
	ctx := context.Background()

	done := make(chan bool, 5)

	for i := 0; i < 5; i++ {
		go func(id int) {
			_, opLogger := ctxLogger.StartOperation(ctx, "test-operation")
			opLogger.End("id", id)
			done <- true
		}(i)
	}

	for i := 0; i < 5; i++ {
		<-done
	}
}

func TestStandardLoggerChain(t *testing.T) {
	baseLogger := logr.Discard()

	stdLogger := NewStandardLogger(baseLogger)
	stdLogger = stdLogger.WithValues("key1", "value1")
	stdLogger = stdLogger.WithValues("key2", "value2")

	stdLogger.Info("chained message")
}
