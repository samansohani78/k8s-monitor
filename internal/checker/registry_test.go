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
	"testing"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetChecker tests the GetChecker convenience function
func TestGetChecker(t *testing.T) {
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTP,
		},
	}
	checker, err := GetChecker(target)
	assert.NoError(t, err)
	assert.NotNil(t, checker)
}

// TestGetSupportedTypes tests GetSupportedTypes
func TestGetSupportedTypes(t *testing.T) {
	types := GetSupportedTypes()
	assert.NotEmpty(t, types)

	// Verify common types exist
	expectedTypes := []string{
		"network", "dns", "http", "https", "kubernetes",
		"postgresql", "mysql", "mssql", "redis", "mongodb", "clickhouse",
		"elasticsearch", "opensearch", "minio",
		"kafka", "rabbitmq",
		"keycloak", "nginx",
		"internal-canary", "external-http", "node-egress", "node-to-node",
	}
	for _, expected := range expectedTypes {
		assert.Contains(t, types, expected, "Should contain type: %s", expected)
	}
}

// TestValidateTargetType tests ValidateTargetType
func TestValidateTargetType(t *testing.T) {
	// Test valid types
	validTypes := []string{
		"http", "https", "network", "dns", "kubernetes",
		"postgresql", "mysql", "redis", "mongodb",
		"kafka", "rabbitmq",
	}
	for _, tt := range validTypes {
		assert.True(t, ValidateTargetType(tt), "Should validate: %s", tt)
	}

	// Test invalid type
	assert.False(t, ValidateTargetType("non-existent"))
	assert.False(t, ValidateTargetType(""))
}

// TestGetCheckerInfo tests GetCheckerInfo
func TestGetCheckerInfo(t *testing.T) {
	info := GetCheckerInfo()
	assert.NotEmpty(t, info)
	assert.Contains(t, info, "http")
	assert.Contains(t, info, "postgresql")
}

// TestKubernetesCheckerFactory tests Kubernetes checker factory
func TestKubernetesCheckerFactory(t *testing.T) {
	factory := &KubernetesCheckerFactory{}

	// Test SupportedTypes
	types := factory.SupportedTypes()
	assert.Contains(t, types, "kubernetes")

	// Test Create
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeKubernetes,
		},
	}
	checker, err := factory.Create(target)
	require.NoError(t, err)
	assert.NotNil(t, checker)

	layers := checker.Layers()
	assert.NotEmpty(t, layers)
	assert.GreaterOrEqual(t, len(layers), 5) // L0, L1, L2, L3, L4+
}

// TestMySQLCheckerFactory tests MySQL checker factory
func TestMySQLCheckerFactory(t *testing.T) {
	factory := &MySQLCheckerFactory{}

	types := factory.SupportedTypes()
	assert.Contains(t, types, "mysql")

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeMySQL,
		},
	}
	checker, err := factory.Create(target)
	require.NoError(t, err)
	assert.NotNil(t, checker)
	assert.NotEmpty(t, checker.Layers())
}

// TestMSSQLCheckerFactory tests MSSQL checker factory
func TestMSSQLCheckerFactory(t *testing.T) {
	factory := &MSSQLCheckerFactory{}

	types := factory.SupportedTypes()
	assert.Contains(t, types, "mssql")

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeMSSQL,
		},
	}
	checker, err := factory.Create(target)
	require.NoError(t, err)
	assert.NotNil(t, checker)
	assert.NotEmpty(t, checker.Layers())
}

// TestMongoDBCheckerFactory tests MongoDB checker factory
func TestMongoDBCheckerFactory(t *testing.T) {
	factory := &MongoDBCheckerFactory{}

	types := factory.SupportedTypes()
	assert.Contains(t, types, "mongodb")

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeMongoDB,
		},
	}
	checker, err := factory.Create(target)
	require.NoError(t, err)
	assert.NotNil(t, checker)
	assert.NotEmpty(t, checker.Layers())
}

// TestClickHouseCheckerFactory tests ClickHouse checker factory
func TestClickHouseCheckerFactory(t *testing.T) {
	factory := &ClickHouseCheckerFactory{}

	types := factory.SupportedTypes()
	assert.Contains(t, types, "clickhouse")

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeClickHouse,
		},
	}
	checker, err := factory.Create(target)
	require.NoError(t, err)
	assert.NotNil(t, checker)
	assert.NotEmpty(t, checker.Layers())
}

// TestElasticsearchCheckerFactory tests Elasticsearch checker factory
func TestElasticsearchCheckerFactory(t *testing.T) {
	factory := &ElasticsearchCheckerFactory{}

	types := factory.SupportedTypes()
	assert.Contains(t, types, "elasticsearch")

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeElasticsearch,
		},
	}
	checker, err := factory.Create(target)
	require.NoError(t, err)
	assert.NotNil(t, checker)
	assert.NotEmpty(t, checker.Layers())
}

// TestOpenSearchCheckerFactory tests OpenSearch checker factory
func TestOpenSearchCheckerFactory(t *testing.T) {
	factory := &OpenSearchCheckerFactory{}

	types := factory.SupportedTypes()
	assert.Contains(t, types, "opensearch")

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeOpenSearch,
		},
	}
	checker, err := factory.Create(target)
	require.NoError(t, err)
	assert.NotNil(t, checker)
	assert.NotEmpty(t, checker.Layers())
}

// TestMinIOCheckerFactory tests MinIO checker factory
func TestMinIOCheckerFactory(t *testing.T) {
	factory := &MinIOCheckerFactory{}

	types := factory.SupportedTypes()
	assert.Contains(t, types, "minio")

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeMinIO,
		},
	}
	checker, err := factory.Create(target)
	require.NoError(t, err)
	assert.NotNil(t, checker)
	assert.NotEmpty(t, checker.Layers())
}

// TestKafkaCheckerFactory tests Kafka checker factory
func TestKafkaCheckerFactory(t *testing.T) {
	factory := &KafkaCheckerFactory{}

	types := factory.SupportedTypes()
	assert.Contains(t, types, "kafka")

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeKafka,
		},
	}
	checker, err := factory.Create(target)
	require.NoError(t, err)
	assert.NotNil(t, checker)
	assert.NotEmpty(t, checker.Layers())
}

// TestRabbitMQCheckerFactory tests RabbitMQ checker factory
func TestRabbitMQCheckerFactory(t *testing.T) {
	factory := &RabbitMQCheckerFactory{}

	types := factory.SupportedTypes()
	assert.Contains(t, types, "rabbitmq")

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeRabbitMQ,
		},
	}
	checker, err := factory.Create(target)
	require.NoError(t, err)
	assert.NotNil(t, checker)
	assert.NotEmpty(t, checker.Layers())
}

// TestKeycloakCheckerFactory tests Keycloak checker factory
func TestKeycloakCheckerFactory(t *testing.T) {
	factory := &KeycloakCheckerFactory{}

	types := factory.SupportedTypes()
	assert.Contains(t, types, "keycloak")

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeKeycloak,
		},
	}
	checker, err := factory.Create(target)
	require.NoError(t, err)
	assert.NotNil(t, checker)
	assert.NotEmpty(t, checker.Layers())
}

// TestNginxCheckerFactory tests Nginx checker factory
func TestNginxCheckerFactory(t *testing.T) {
	factory := &NginxCheckerFactory{}

	types := factory.SupportedTypes()
	assert.Contains(t, types, "nginx")

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNginx,
		},
	}
	checker, err := factory.Create(target)
	require.NoError(t, err)
	assert.NotNil(t, checker)
	assert.NotEmpty(t, checker.Layers())
}

// TestInternalCanaryCheckerFactory tests internal canary checker factory
func TestInternalCanaryCheckerFactory(t *testing.T) {
	factory := &InternalCanaryCheckerFactory{}

	types := factory.SupportedTypes()
	assert.Contains(t, types, "internal-canary")

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeInternalCanary,
		},
	}
	checker, err := factory.Create(target)
	require.NoError(t, err)
	assert.NotNil(t, checker)
	assert.NotEmpty(t, checker.Layers())
}

// TestExternalHTTPCheckerFactory tests external HTTP checker factory
func TestExternalHTTPCheckerFactory(t *testing.T) {
	factory := &ExternalHTTPCheckerFactory{}

	types := factory.SupportedTypes()
	assert.Contains(t, types, "external-http")

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeExternalHTTP,
		},
	}
	checker, err := factory.Create(target)
	require.NoError(t, err)
	assert.NotNil(t, checker)
	assert.NotEmpty(t, checker.Layers())
}

// TestNodeEgressCheckerFactory tests node egress checker factory
func TestNodeEgressCheckerFactory(t *testing.T) {
	factory := &NodeEgressCheckerFactory{}

	types := factory.SupportedTypes()
	assert.Contains(t, types, "node-egress")

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNodeEgress,
		},
	}
	checker, err := factory.Create(target)
	require.NoError(t, err)
	assert.NotNil(t, checker)
	assert.NotEmpty(t, checker.Layers())
}

// TestNodeToNodeCheckerFactory tests node-to-node checker factory
func TestNodeToNodeCheckerFactory(t *testing.T) {
	factory := &NodeToNodeCheckerFactory{}

	types := factory.SupportedTypes()
	assert.Contains(t, types, "node-to-node")

	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNodeToNode,
		},
	}
	checker, err := factory.Create(target)
	require.NoError(t, err)
	assert.NotNil(t, checker)
	assert.NotEmpty(t, checker.Layers())
}

// TestAllCheckerFactoriesExecute tests that all checker factories can execute
func TestAllCheckerFactoriesExecute(t *testing.T) {
	types := GetSupportedTypes()

	for _, targetType := range types {
		t.Run(targetType, func(t *testing.T) {
			target := &k8swatchv1.Target{
				Spec: k8swatchv1.TargetSpec{
					Type: k8swatchv1.TargetType(targetType),
				},
			}
			checker, err := GetChecker(target)
			require.NoError(t, err)
			assert.NotNil(t, checker)

			// Execute should return a result (may fail, but should return something)
			result, _ := checker.Execute(t.Context(), target)
			assert.NotNil(t, result)
		})
	}
}
