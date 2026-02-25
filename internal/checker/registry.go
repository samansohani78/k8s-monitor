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
	"fmt"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
)

// NewDefaultRegistry creates a registry with all built-in checkers registered
func NewDefaultRegistry() *Registry {
	reg := NewRegistry()

	// Register all checker factories
	// Core infrastructure
	reg.Register(&NetworkCheckerFactory{}, string(k8swatchv1.TargetTypeNetwork))
	reg.Register(&NetworkCheckerFactory{}, string(k8swatchv1.TargetTypeDNS))
	reg.Register(&HTTPCheckerFactory{}, string(k8swatchv1.TargetTypeHTTP))
	reg.Register(&HTTPCheckerFactory{}, string(k8swatchv1.TargetTypeHTTPS))
	reg.Register(&KubernetesCheckerFactory{}, string(k8swatchv1.TargetTypeKubernetes))

	// Databases
	reg.Register(&PostgreSQLCheckerFactory{}, string(k8swatchv1.TargetTypePostgreSQL))
	reg.Register(&MySQLCheckerFactory{}, string(k8swatchv1.TargetTypeMySQL))
	reg.Register(&MSSQLCheckerFactory{}, string(k8swatchv1.TargetTypeMSSQL))
	reg.Register(&RedisCheckerFactory{}, string(k8swatchv1.TargetTypeRedis))
	reg.Register(&MongoDBCheckerFactory{}, string(k8swatchv1.TargetTypeMongoDB))
	reg.Register(&ClickHouseCheckerFactory{}, string(k8swatchv1.TargetTypeClickHouse))

	// Search & Storage
	reg.Register(&ElasticsearchCheckerFactory{}, string(k8swatchv1.TargetTypeElasticsearch))
	reg.Register(&OpenSearchCheckerFactory{}, string(k8swatchv1.TargetTypeOpenSearch))
	reg.Register(&MinIOCheckerFactory{}, string(k8swatchv1.TargetTypeMinIO))

	// Messaging
	reg.Register(&KafkaCheckerFactory{}, string(k8swatchv1.TargetTypeKafka))
	reg.Register(&RabbitMQCheckerFactory{}, string(k8swatchv1.TargetTypeRabbitMQ))

	// Identity & Proxy
	reg.Register(&KeycloakCheckerFactory{}, string(k8swatchv1.TargetTypeKeycloak))
	reg.Register(&NginxCheckerFactory{}, string(k8swatchv1.TargetTypeNginx))

	// Synthetic / Meta
	reg.Register(&InternalCanaryCheckerFactory{}, string(k8swatchv1.TargetTypeInternalCanary))
	reg.Register(&ExternalHTTPCheckerFactory{}, string(k8swatchv1.TargetTypeExternalHTTP))
	reg.Register(&NodeEgressCheckerFactory{}, string(k8swatchv1.TargetTypeNodeEgress))
	reg.Register(&NodeToNodeCheckerFactory{}, string(k8swatchv1.TargetTypeNodeToNode))

	return reg
}

// KubernetesCheckerFactory creates Kubernetes checkers
type KubernetesCheckerFactory struct{}

func (f *KubernetesCheckerFactory) Create(target *k8swatchv1.Target) (Checker, error) {
	layers := []Layer{
		NewNodeSanityChecker(DefaultNodeSanityConfig()),
		NewDNSLayer(),
		NewTCPLayer(),
		NewTLSLayer(),
		NewHTTPProtocolLayer(),
	}
	return NewBaseChecker(string(k8swatchv1.TargetTypeKubernetes), layers), nil
}

func (f *KubernetesCheckerFactory) SupportedTypes() []string { return []string{"kubernetes"} }

// GetChecker is a convenience function to get a checker for a target
func GetChecker(target *k8swatchv1.Target) (Checker, error) {
	reg := NewDefaultRegistry()
	return reg.Create(target)
}

// GetSupportedTypes returns all supported target types
func GetSupportedTypes() []string {
	reg := NewDefaultRegistry()
	return reg.SupportedTypes()
}

// ValidateTargetType checks if a target type is supported
func ValidateTargetType(targetType string) bool {
	supported := GetSupportedTypes()
	for _, t := range supported {
		if t == targetType {
			return true
		}
	}
	return false
}

// GetCheckerInfo returns information about available checkers
func GetCheckerInfo() string {
	reg := NewDefaultRegistry()
	result := ""
	for _, t := range reg.SupportedTypes() {
		factory, err := reg.Get(t)
		if err == nil {
			checker, _ := factory.Create(&k8swatchv1.Target{
				Spec: k8swatchv1.TargetSpec{Type: k8swatchv1.TargetType(t)},
			})
			if checker != nil {
				layers := make([]string, len(checker.Layers()))
				for i, layer := range checker.Layers() {
					layers[i] = layer.Name()
				}
				result += fmt.Sprintf("%s: %v\n", t, layers)
			}
		}
	}
	return result
}
