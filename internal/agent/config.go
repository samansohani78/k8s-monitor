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

package agent

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
)

// ConfigLoader loads target configurations from Kubernetes API
// Stateless design: fetches fresh config on each check interval
type ConfigLoader struct {
	client        client.Client
	namespace     string
	labelSelector labels.Selector
}

// ConfigLoaderConfig holds configuration for the config loader
type ConfigLoaderConfig struct {
	// Namespace is the namespace to watch for targets
	Namespace string

	// LabelSelector is the label selector for targets
	LabelSelector string
}

// DefaultConfigLoaderConfig returns the default config loader configuration
func DefaultConfigLoaderConfig() *ConfigLoaderConfig {
	return &ConfigLoaderConfig{
		Namespace:     "k8swatch",
		LabelSelector: "",
	}
}

// NewConfigLoader creates a new config loader
func NewConfigLoader(client client.Client, cfg *ConfigLoaderConfig) (*ConfigLoader, error) {
	if cfg == nil {
		cfg = DefaultConfigLoaderConfig()
	}

	var selector labels.Selector
	if cfg.LabelSelector != "" {
		var err error
		selector, err = labels.Parse(cfg.LabelSelector)
		if err != nil {
			return nil, fmt.Errorf("failed to parse label selector: %w", err)
		}
	}

	return &ConfigLoader{
		client:        client,
		namespace:     cfg.Namespace,
		labelSelector: selector,
	}, nil
}

// LoadTargets fetches all targets from the Kubernetes API
// This is a stateless operation - no caching between calls
func (l *ConfigLoader) LoadTargets(ctx context.Context) ([]k8swatchv1.Target, string, error) {
	startTime := time.Now()

	// Build list options
	listOpts := []client.ListOption{
		client.InNamespace(l.namespace),
	}

	if l.labelSelector != nil {
		listOpts = append(listOpts, client.MatchingLabelsSelector{
			Selector: l.labelSelector,
		})
	}

	// Fetch targets from API
	var targetList k8swatchv1.TargetList
	if err := l.client.List(ctx, &targetList, listOpts...); err != nil {
		return nil, "", fmt.Errorf("failed to list targets: %w", err)
	}

	// Generate config version based on resource versions
	configVersion := l.generateConfigVersion(targetList.Items)

	log.Info("Config loaded",
		"targetCount", len(targetList.Items),
		"configVersion", configVersion,
		"durationMs", time.Since(startTime).Milliseconds(),
	)

	return targetList.Items, configVersion, nil
}

// LoadTarget fetches a single target by name
func (l *ConfigLoader) LoadTarget(ctx context.Context, name string) (*k8swatchv1.Target, error) {
	var target k8swatchv1.Target
	key := client.ObjectKey{
		Namespace: l.namespace,
		Name:      name,
	}

	if err := l.client.Get(ctx, key, &target); err != nil {
		return nil, fmt.Errorf("failed to get target %s: %w", name, err)
	}

	return &target, nil
}

// generateConfigVersion generates a config version from target resource versions
func (l *ConfigLoader) generateConfigVersion(targets []k8swatchv1.Target) string {
	if len(targets) == 0 {
		return "empty"
	}

	// Simple version: count + latest resource version
	maxRV := ""
	for _, t := range targets {
		rv := t.GetResourceVersion()
		if rv > maxRV {
			maxRV = rv
		}
	}

	return fmt.Sprintf("v%d-%s", len(targets), maxRV)
}

// ValidateTarget validates a target configuration
func ValidateTarget(target *k8swatchv1.Target) error {
	// Validate target type is supported
	supportedTypes := map[k8swatchv1.TargetType]bool{
		k8swatchv1.TargetTypeNetwork:        true,
		k8swatchv1.TargetTypeDNS:            true,
		k8swatchv1.TargetTypeHTTP:           true,
		k8swatchv1.TargetTypeHTTPS:          true,
		k8swatchv1.TargetTypeKubernetes:     true,
		k8swatchv1.TargetTypeRedis:          true,
		k8swatchv1.TargetTypePostgreSQL:     true,
		k8swatchv1.TargetTypeMySQL:          true,
		k8swatchv1.TargetTypeMSSQL:          true,
		k8swatchv1.TargetTypeMongoDB:        true,
		k8swatchv1.TargetTypeClickHouse:     true,
		k8swatchv1.TargetTypeElasticsearch:  true,
		k8swatchv1.TargetTypeOpenSearch:     true,
		k8swatchv1.TargetTypeMinIO:          true,
		k8swatchv1.TargetTypeKafka:          true,
		k8swatchv1.TargetTypeRabbitMQ:       true,
		k8swatchv1.TargetTypeKeycloak:       true,
		k8swatchv1.TargetTypeNginx:          true,
		k8swatchv1.TargetTypeInternalCanary: true,
		k8swatchv1.TargetTypeExternalHTTP:   true,
		k8swatchv1.TargetTypeNodeEgress:     true,
		k8swatchv1.TargetTypeNodeToNode:     true,
	}

	if !supportedTypes[target.Spec.Type] {
		return fmt.Errorf("unsupported target type: %s", target.Spec.Type)
	}

	// Validate endpoint configuration
	if err := validateEndpoint(&target.Spec.Endpoint); err != nil {
		return fmt.Errorf("invalid endpoint: %w", err)
	}

	// Validate schedule
	if target.Spec.Schedule.Interval == "" {
		return fmt.Errorf("schedule.interval is required")
	}

	if _, err := time.ParseDuration(target.Spec.Schedule.Interval); err != nil {
		return fmt.Errorf("invalid schedule.interval: %w", err)
	}

	if target.Spec.Schedule.Timeout != "" {
		if _, err := time.ParseDuration(target.Spec.Schedule.Timeout); err != nil {
			return fmt.Errorf("invalid schedule.timeout: %w", err)
		}
	}

	// Validate network modes
	if len(target.Spec.NetworkModes) == 0 {
		// Default to pod network mode
		target.Spec.NetworkModes = []k8swatchv1.NetworkMode{k8swatchv1.NetworkModePod}
	}

	for _, mode := range target.Spec.NetworkModes {
		if mode != k8swatchv1.NetworkModePod && mode != k8swatchv1.NetworkModeHost {
			return fmt.Errorf("invalid network mode: %s", mode)
		}
	}

	return nil
}

// validateEndpoint validates endpoint configuration
func validateEndpoint(endpoint *k8swatchv1.EndpointConfig) error {
	// Count configured endpoint types
	configured := 0
	if endpoint.K8sService != nil {
		configured++
		if endpoint.K8sService.Name == "" {
			return fmt.Errorf("k8sService.name is required")
		}
		if endpoint.K8sService.Port == "" {
			return fmt.Errorf("k8sService.port is required")
		}
	}
	if endpoint.DNS != nil {
		configured++
		if *endpoint.DNS == "" {
			return fmt.Errorf("dns hostname is required")
		}
	}
	if endpoint.IP != nil {
		configured++
		if *endpoint.IP == "" {
			return fmt.Errorf("ip address is required")
		}
	}

	if configured == 0 {
		return fmt.Errorf("one of k8sService, dns, or ip must be specified")
	}

	if configured > 1 {
		return fmt.Errorf("only one of k8sService, dns, or ip can be specified")
	}

	return nil
}

// ConfigLoaderWithClient creates a config loader with an existing client
func ConfigLoaderWithClient(c client.Client, namespace string) *ConfigLoader {
	return &ConfigLoader{
		client:    c,
		namespace: namespace,
	}
}
