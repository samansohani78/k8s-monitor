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

package checker

import (
	"context"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
)

// Checker is the interface implemented by all target checkers
type Checker interface {
	// Execute executes all enabled layers for a target and returns the result
	Execute(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error)

	// Layers returns the list of layers this checker supports
	Layers() []Layer
}

// CheckerFactory creates checkers for specific target types
type CheckerFactory interface {
	// Create creates a new checker for the given target
	Create(target *k8swatchv1.Target) (Checker, error)

	// SupportedTypes returns the list of target types this factory supports
	SupportedTypes() []string
}

// Registry is a registry of checker factories
type Registry struct {
	factories map[string]CheckerFactory
}

// NewRegistry creates a new checker registry
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]CheckerFactory),
	}
}

// Register registers a checker factory for the given target types
func (r *Registry) Register(factory CheckerFactory, targetTypes ...string) {
	for _, t := range targetTypes {
		r.factories[t] = factory
		log.Info("Registered checker factory", "targetType", t)
	}
}

// Get returns a checker factory for the given target type
func (r *Registry) Get(targetType string) (CheckerFactory, error) {
	factory, ok := r.factories[targetType]
	if !ok {
		return nil, ErrUnsupportedTargetType{TargetType: targetType}
	}
	return factory, nil
}

// Create creates a checker for the given target
func (r *Registry) Create(target *k8swatchv1.Target) (Checker, error) {
	factory, err := r.Get(string(target.Spec.Type))
	if err != nil {
		return nil, err
	}
	return factory.Create(target)
}

// SupportedTypes returns all supported target types
func (r *Registry) SupportedTypes() []string {
	types := make([]string, 0, len(r.factories))
	for t := range r.factories {
		types = append(types, t)
	}
	return types
}

// ErrUnsupportedTargetType is returned when a target type is not supported
type ErrUnsupportedTargetType struct {
	TargetType string
}

func (e ErrUnsupportedTargetType) Error() string {
	return "unsupported target type: " + e.TargetType
}

// BaseChecker provides common functionality for all checkers
type BaseChecker struct {
	targetType string
	layers     []Layer
}

// NewBaseChecker creates a new base checker
func NewBaseChecker(targetType string, layers []Layer) *BaseChecker {
	return &BaseChecker{
		targetType: targetType,
		layers:     layers,
	}
}

// Layers returns the layers for this checker
func (c *BaseChecker) Layers() []Layer {
	return c.layers
}

// Execute executes all enabled layers for a target
func (c *BaseChecker) Execute(ctx context.Context, target *k8swatchv1.Target) (*k8swatchv1.CheckResult, error) {
	executor := NewExecutor(c.layers)
	return executor.Execute(ctx, target)
}
