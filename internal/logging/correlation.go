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

// Package logging provides structured logging utilities for K8sWatch components.
package logging

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	// CorrelationIDKey is the context key for correlation ID
	CorrelationIDKey contextKey = "correlationID"
)

// CorrelationIDGenerator generates unique correlation IDs
type CorrelationIDGenerator struct {
	pool sync.Pool
}

// NewCorrelationIDGenerator creates a new correlation ID generator
func NewCorrelationIDGenerator() *CorrelationIDGenerator {
	return &CorrelationIDGenerator{
		pool: sync.Pool{
			New: func() interface{} {
				return new(uuid.UUID)
			},
		},
	}
}

// Generate creates a new correlation ID
func (g *CorrelationIDGenerator) Generate() string {
	id := uuid.New()
	return id.String()
}

// GenerateOrExtract generates a new correlation ID or extracts from context
func (g *CorrelationIDGenerator) GenerateOrExtract(ctx context.Context) (context.Context, string) {
	if existing, ok := ctx.Value(CorrelationIDKey).(string); ok && existing != "" {
		return ctx, existing
	}

	id := g.Generate()
	ctx = context.WithValue(ctx, CorrelationIDKey, id)
	return ctx, id
}

// FromContext extracts correlation ID from context
func FromContext(ctx context.Context) string {
	if id, ok := ctx.Value(CorrelationIDKey).(string); ok {
		return id
	}
	return ""
}

// WithContext creates a new logger with correlation ID from context
func WithContext(logger logr.Logger, ctx context.Context) logr.Logger {
	if id := FromContext(ctx); id != "" {
		return logger.WithValues("correlationID", id)
	}
	return logger
}

// NewContextWithCorrelationID creates a new context with correlation ID
func NewContextWithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, CorrelationIDKey, correlationID)
}
