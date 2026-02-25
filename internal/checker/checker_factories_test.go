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
	"testing"
	"time"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMySQLCheckerFactoryCreate tests MySQL checker factory
func TestMySQLCheckerFactoryCreate(t *testing.T) {
	factory := &MySQLCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type:     k8swatchv1.TargetTypeMySQL,
			Endpoint: k8swatchv1.EndpointConfig{Port: ptrInt32(3306)},
		},
	}

	checker, err := factory.Create(target)
	require.NoError(t, err)
	assert.NotNil(t, checker)
	assert.NotEmpty(t, checker.Layers())
}

// TestMySQLCheckerFactorySupportedTypes tests supported types
func TestMySQLCheckerFactorySupportedTypes(t *testing.T) {
	factory := &MySQLCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Contains(t, types, "mysql")
}

// TestMongoDBCheckerFactoryCreate tests MongoDB checker factory
func TestMongoDBCheckerFactoryCreate(t *testing.T) {
	factory := &MongoDBCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type:     k8swatchv1.TargetTypeMongoDB,
			Endpoint: k8swatchv1.EndpointConfig{Port: ptrInt32(27017)},
		},
	}

	checker, err := factory.Create(target)
	require.NoError(t, err)
	assert.NotNil(t, checker)
	assert.NotEmpty(t, checker.Layers())
}

// TestMongoDBCheckerFactorySupportedTypes tests supported types
func TestMongoDBCheckerFactorySupportedTypes(t *testing.T) {
	factory := &MongoDBCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Contains(t, types, "mongodb")
}

// TestKafkaCheckerFactoryCreate tests Kafka checker factory
func TestKafkaCheckerFactoryCreate(t *testing.T) {
	factory := &KafkaCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type:     k8swatchv1.TargetTypeKafka,
			Endpoint: k8swatchv1.EndpointConfig{Port: ptrInt32(9092)},
		},
	}

	checker, err := factory.Create(target)
	require.NoError(t, err)
	assert.NotNil(t, checker)
	assert.NotEmpty(t, checker.Layers())
}

// TestKafkaCheckerFactorySupportedTypes tests supported types
func TestKafkaCheckerFactorySupportedTypes(t *testing.T) {
	factory := &KafkaCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Contains(t, types, "kafka")
}

// TestRabbitMQCheckerFactoryCreate tests RabbitMQ checker factory
func TestRabbitMQCheckerFactoryCreate(t *testing.T) {
	factory := &RabbitMQCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type:     k8swatchv1.TargetTypeRabbitMQ,
			Endpoint: k8swatchv1.EndpointConfig{Port: ptrInt32(5672)},
		},
	}

	checker, err := factory.Create(target)
	require.NoError(t, err)
	assert.NotNil(t, checker)
	assert.NotEmpty(t, checker.Layers())
}

// TestRabbitMQCheckerFactorySupportedTypes tests supported types
func TestRabbitMQCheckerFactorySupportedTypes(t *testing.T) {
	factory := &RabbitMQCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Contains(t, types, "rabbitmq")
}

// TestElasticsearchCheckerFactoryCreate tests Elasticsearch checker factory
func TestElasticsearchCheckerFactoryCreate(t *testing.T) {
	factory := &ElasticsearchCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type:     k8swatchv1.TargetTypeElasticsearch,
			Endpoint: k8swatchv1.EndpointConfig{Port: ptrInt32(9200)},
		},
	}

	checker, err := factory.Create(target)
	require.NoError(t, err)
	assert.NotNil(t, checker)
	assert.NotEmpty(t, checker.Layers())
}

// TestElasticsearchCheckerFactorySupportedTypes tests supported types
func TestElasticsearchCheckerFactorySupportedTypes(t *testing.T) {
	factory := &ElasticsearchCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Contains(t, types, "elasticsearch")
}

// TestOpenSearchCheckerFactoryCreate tests OpenSearch checker factory
func TestOpenSearchCheckerFactoryCreate(t *testing.T) {
	factory := &OpenSearchCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type:     k8swatchv1.TargetTypeOpenSearch,
			Endpoint: k8swatchv1.EndpointConfig{Port: ptrInt32(9200)},
		},
	}

	checker, err := factory.Create(target)
	require.NoError(t, err)
	assert.NotNil(t, checker)
	assert.NotEmpty(t, checker.Layers())
}

// TestOpenSearchCheckerFactorySupportedTypes tests supported types
func TestOpenSearchCheckerFactorySupportedTypes(t *testing.T) {
	factory := &OpenSearchCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Contains(t, types, "opensearch")
}

// TestMinIOCheckerFactoryCreate tests MinIO checker factory
func TestMinIOCheckerFactoryCreate(t *testing.T) {
	factory := &MinIOCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type:     k8swatchv1.TargetTypeMinIO,
			Endpoint: k8swatchv1.EndpointConfig{Port: ptrInt32(9000)},
		},
	}

	checker, err := factory.Create(target)
	require.NoError(t, err)
	assert.NotNil(t, checker)
	assert.NotEmpty(t, checker.Layers())
}

// TestMinIOCheckerFactorySupportedTypes tests supported types
func TestMinIOCheckerFactorySupportedTypes(t *testing.T) {
	factory := &MinIOCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Contains(t, types, "minio")
}

// TestClickHouseCheckerFactoryCreate tests ClickHouse checker factory
func TestClickHouseCheckerFactoryCreate(t *testing.T) {
	factory := &ClickHouseCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type:     k8swatchv1.TargetTypeClickHouse,
			Endpoint: k8swatchv1.EndpointConfig{Port: ptrInt32(9000)},
		},
	}

	checker, err := factory.Create(target)
	require.NoError(t, err)
	assert.NotNil(t, checker)
	assert.NotEmpty(t, checker.Layers())
}

// TestClickHouseCheckerFactorySupportedTypes tests supported types
func TestClickHouseCheckerFactorySupportedTypes(t *testing.T) {
	factory := &ClickHouseCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Contains(t, types, "clickhouse")
}

// TestMSSQLCheckerFactoryCreate tests MSSQL checker factory
func TestMSSQLCheckerFactoryCreate(t *testing.T) {
	factory := &MSSQLCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type:     k8swatchv1.TargetTypeMSSQL,
			Endpoint: k8swatchv1.EndpointConfig{Port: ptrInt32(1433)},
		},
	}

	checker, err := factory.Create(target)
	require.NoError(t, err)
	assert.NotNil(t, checker)
	assert.NotEmpty(t, checker.Layers())
}

// TestMSSQLCheckerFactorySupportedTypes tests supported types
func TestMSSQLCheckerFactorySupportedTypes(t *testing.T) {
	factory := &MSSQLCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Contains(t, types, "mssql")
}

// TestKeycloakCheckerFactoryCreate tests Keycloak checker factory
func TestKeycloakCheckerFactoryCreate(t *testing.T) {
	factory := &KeycloakCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type:     k8swatchv1.TargetTypeKeycloak,
			Endpoint: k8swatchv1.EndpointConfig{Port: ptrInt32(443)},
		},
	}

	checker, err := factory.Create(target)
	require.NoError(t, err)
	assert.NotNil(t, checker)
	assert.NotEmpty(t, checker.Layers())
}

// TestKeycloakCheckerFactorySupportedTypes tests supported types
func TestKeycloakCheckerFactorySupportedTypes(t *testing.T) {
	factory := &KeycloakCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Contains(t, types, "keycloak")
}

// TestNginxCheckerFactoryCreate tests Nginx checker factory
func TestNginxCheckerFactoryCreate(t *testing.T) {
	factory := &NginxCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type:     k8swatchv1.TargetTypeNginx,
			Endpoint: k8swatchv1.EndpointConfig{Port: ptrInt32(80)},
		},
	}

	checker, err := factory.Create(target)
	require.NoError(t, err)
	assert.NotNil(t, checker)
	assert.NotEmpty(t, checker.Layers())
}

// TestNginxCheckerFactorySupportedTypes tests supported types
func TestNginxCheckerFactorySupportedTypes(t *testing.T) {
	factory := &NginxCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Contains(t, types, "nginx")
}

// TestInternalCanaryCheckerFactoryCreate tests internal canary checker factory
func TestInternalCanaryCheckerFactoryCreate(t *testing.T) {
	factory := &InternalCanaryCheckerFactory{}
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

// TestInternalCanaryCheckerFactorySupportedTypes tests supported types
func TestInternalCanaryCheckerFactorySupportedTypes(t *testing.T) {
	factory := &InternalCanaryCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Contains(t, types, "internal-canary")
}

// TestExternalHTTPCheckerFactoryCreate tests external HTTP checker factory
func TestExternalHTTPCheckerFactoryCreate(t *testing.T) {
	factory := &ExternalHTTPCheckerFactory{}
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

// TestExternalHTTPCheckerFactorySupportedTypes tests supported types
func TestExternalHTTPCheckerFactorySupportedTypes(t *testing.T) {
	factory := &ExternalHTTPCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Contains(t, types, "external-http")
}

// TestNodeEgressCheckerFactoryCreate tests node egress checker factory
func TestNodeEgressCheckerFactoryCreate(t *testing.T) {
	factory := &NodeEgressCheckerFactory{}
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

// TestNodeEgressCheckerFactorySupportedTypes tests supported types
func TestNodeEgressCheckerFactorySupportedTypes(t *testing.T) {
	factory := &NodeEgressCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Contains(t, types, "node-egress")
}

// TestNodeToNodeCheckerFactoryCreate tests node-to-node checker factory
func TestNodeToNodeCheckerFactoryCreate(t *testing.T) {
	factory := &NodeToNodeCheckerFactory{}
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

// TestNodeToNodeCheckerFactorySupportedTypes tests supported types
func TestNodeToNodeCheckerFactorySupportedTypes(t *testing.T) {
	factory := &NodeToNodeCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Contains(t, types, "node-to-node")
}

// TestAllCheckerExecute tests that all checkers can execute
func TestAllCheckerExecute(t *testing.T) {
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

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			result, _ := checker.Execute(ctx, target)
			assert.NotNil(t, result)
		})
	}
}
func ptrInt32(i int32) *int32 { return &i }
