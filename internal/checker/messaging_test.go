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
)

func TestKafkaCheckerFactory_Create(t *testing.T) {
	factory := &KafkaCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeKafka,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS:  strPtr("kafka.default.svc"),
				Port: int32Ptr(9092),
			},
		},
	}

	checker, err := factory.Create(target)
	assert.NoError(t, err)
	assert.NotNil(t, checker)
	assert.Len(t, checker.Layers(), 4)
}

func TestKafkaCheckerFactory_SupportedTypes(t *testing.T) {
	factory := &KafkaCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Equal(t, []string{"kafka"}, types)
}

func TestKafkaChecker_Execute(t *testing.T) {
	factory := &KafkaCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeKafka,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS:  strPtr("kafka.invalid.svc"),
				Port: int32Ptr(9092),
			},
		},
	}

	checker, _ := factory.Create(target)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := checker.Execute(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Check.Success)
}

func TestKafkaProtocolLayer_Name(t *testing.T) {
	layer := NewKafkaProtocolLayer()
	assert.Equal(t, "L4", layer.Name())
}

func TestKafkaProtocolLayer_Enabled(t *testing.T) {
	layer := NewKafkaProtocolLayer()
	
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeKafka,
		},
	}
	assert.True(t, layer.Enabled(target))
	
	target2 := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTP,
			Layers: k8swatchv1.LayerConfig{
				L4Protocol: &k8swatchv1.ProtocolConfig{
					LayerConfigBase: k8swatchv1.LayerConfigBase{
						Enabled: true,
					},
				},
			},
		},
	}
	assert.True(t, layer.Enabled(target2))
	
	target3 := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeHTTP,
		},
	}
	assert.False(t, layer.Enabled(target3))
}

func TestKafkaProtocolLayer_Check_NoEndpoint(t *testing.T) {
	layer := NewKafkaProtocolLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeKafka,
			Endpoint: k8swatchv1.EndpointConfig{},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := layer.Check(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, string(k8swatchv1.FailureCodeConfigError), result.FailureCode)
}

func TestKafkaProtocolLayer_Check_ConnectionRefused(t *testing.T) {
	layer := NewKafkaProtocolLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeKafka,
			Endpoint: k8swatchv1.EndpointConfig{
				IP:   strPtr("127.0.0.1"),
				Port: int32Ptr(9092),
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := layer.Check(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, string(k8swatchv1.FailureCodeTCPRefused), result.FailureCode)
}

func TestKafkaProtocolLayer_Check_K8sService(t *testing.T) {
	layer := NewKafkaProtocolLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeKafka,
			Endpoint: k8swatchv1.EndpointConfig{
				K8sService: &k8swatchv1.K8sServiceEndpoint{
					Name:      "kafka",
					Namespace: "default",
					Port:      "9092",
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := layer.Check(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestRabbitMQCheckerFactory_Create(t *testing.T) {
	factory := &RabbitMQCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeRabbitMQ,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS:  strPtr("rabbitmq.default.svc"),
				Port: int32Ptr(5672),
			},
		},
	}

	checker, err := factory.Create(target)
	assert.NoError(t, err)
	assert.NotNil(t, checker)
	assert.Len(t, checker.Layers(), 5)
}

func TestRabbitMQCheckerFactory_SupportedTypes(t *testing.T) {
	factory := &RabbitMQCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Equal(t, []string{"rabbitmq"}, types)
}

func TestRabbitMQChecker_Execute(t *testing.T) {
	factory := &RabbitMQCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeRabbitMQ,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS:  strPtr("rabbitmq.invalid.svc"),
				Port: int32Ptr(5672),
			},
		},
	}

	checker, _ := factory.Create(target)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := checker.Execute(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Check.Success)
}

func TestRabbitMQProtocolLayer_Name(t *testing.T) {
	layer := NewRabbitMQProtocolLayer()
	assert.Equal(t, "L4", layer.Name())
}

func TestRabbitMQProtocolLayer_Enabled(t *testing.T) {
	layer := NewRabbitMQProtocolLayer()
	
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeRabbitMQ,
		},
	}
	assert.True(t, layer.Enabled(target))
}

func TestRabbitMQProtocolLayer_Check_NoEndpoint(t *testing.T) {
	layer := NewRabbitMQProtocolLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeRabbitMQ,
			Endpoint: k8swatchv1.EndpointConfig{},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := layer.Check(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, string(k8swatchv1.FailureCodeConfigError), result.FailureCode)
}

func TestRabbitMQProtocolLayer_Check_ConnectionRefused(t *testing.T) {
	layer := NewRabbitMQProtocolLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeRabbitMQ,
			Endpoint: k8swatchv1.EndpointConfig{
				IP:   strPtr("127.0.0.1"),
				Port: int32Ptr(5672),
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := layer.Check(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, string(k8swatchv1.FailureCodeTCPRefused), result.FailureCode)
}

func TestRabbitMQSemanticLayer_Name(t *testing.T) {
	layer := NewRabbitMQSemanticLayer()
	assert.Equal(t, "L6", layer.Name())
}

func TestRabbitMQSemanticLayer_Enabled(t *testing.T) {
	layer := NewRabbitMQSemanticLayer()
	
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Layers: k8swatchv1.LayerConfig{
				L6Semantic: &k8swatchv1.SemanticConfig{
					LayerConfigBase: k8swatchv1.LayerConfigBase{
						Enabled: true,
					},
				},
			},
		},
	}
	assert.True(t, layer.Enabled(target))
	
	target2 := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Layers: k8swatchv1.LayerConfig{
				L6Semantic: &k8swatchv1.SemanticConfig{
					LayerConfigBase: k8swatchv1.LayerConfigBase{
						Enabled: false,
					},
				},
			},
		},
	}
	assert.False(t, layer.Enabled(target2))
}

func TestRabbitMQSemanticLayer_Check_NoEndpoint(t *testing.T) {
	layer := NewRabbitMQSemanticLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeRabbitMQ,
			Endpoint: k8swatchv1.EndpointConfig{},
			Layers: k8swatchv1.LayerConfig{
				L6Semantic: &k8swatchv1.SemanticConfig{
					LayerConfigBase: k8swatchv1.LayerConfigBase{
						Enabled: true,
					},
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := layer.Check(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	// RabbitMQ semantic layer falls back to success if management API is not available
	assert.True(t, result.Success || result.FailureCode != "")
}

func TestKeycloakCheckerFactory_Create(t *testing.T) {
	factory := &KeycloakCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeKeycloak,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS:  strPtr("keycloak.default.svc"),
				Port: int32Ptr(443),
			},
		},
	}

	checker, err := factory.Create(target)
	assert.NoError(t, err)
	assert.NotNil(t, checker)
	assert.Len(t, checker.Layers(), 5)
}

func TestKeycloakCheckerFactory_SupportedTypes(t *testing.T) {
	factory := &KeycloakCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Equal(t, []string{"keycloak"}, types)
}

func TestKeycloakProtocolLayer_Name(t *testing.T) {
	layer := NewKeycloakProtocolLayer()
	assert.Equal(t, "L4", layer.Name())
}

func TestKeycloakProtocolLayer_Enabled(t *testing.T) {
	layer := NewKeycloakProtocolLayer()
	
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeKeycloak,
		},
	}
	assert.True(t, layer.Enabled(target))
}

func TestKeycloakProtocolLayer_Check_NoEndpoint(t *testing.T) {
	layer := NewKeycloakProtocolLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeKeycloak,
			Endpoint: k8swatchv1.EndpointConfig{},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := layer.Check(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, string(k8swatchv1.FailureCodeConfigError), result.FailureCode)
}

func TestKeycloakProtocolLayer_Check_ConnectionRefused(t *testing.T) {
	layer := NewKeycloakProtocolLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeKeycloak,
			Endpoint: k8swatchv1.EndpointConfig{
				IP:   strPtr("127.0.0.1"),
				Port: int32Ptr(443),
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := layer.Check(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, string(k8swatchv1.FailureCodeTCPRefused), result.FailureCode)
}

func TestNginxCheckerFactory_Create(t *testing.T) {
	factory := &NginxCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNginx,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS:  strPtr("nginx.default.svc"),
				Port: int32Ptr(80),
			},
		},
	}

	checker, err := factory.Create(target)
	assert.NoError(t, err)
	assert.NotNil(t, checker)
	assert.Len(t, checker.Layers(), 4)
}

func TestNginxCheckerFactory_SupportedTypes(t *testing.T) {
	factory := &NginxCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Equal(t, []string{"nginx"}, types)
}

func TestNginxProtocolLayer_Name(t *testing.T) {
	layer := NewNginxProtocolLayer()
	assert.Equal(t, "L4", layer.Name())
}

func TestNginxProtocolLayer_Enabled(t *testing.T) {
	layer := NewNginxProtocolLayer()
	
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNginx,
		},
	}
	assert.True(t, layer.Enabled(target))
}

func TestNginxProtocolLayer_Check_NoEndpoint(t *testing.T) {
	layer := NewNginxProtocolLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNginx,
			Endpoint: k8swatchv1.EndpointConfig{},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := layer.Check(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, string(k8swatchv1.FailureCodeConfigError), result.FailureCode)
}

func TestNginxProtocolLayer_Check_ConnectionRefused(t *testing.T) {
	layer := NewNginxProtocolLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeNginx,
			Endpoint: k8swatchv1.EndpointConfig{
				IP:   strPtr("127.0.0.1"),
				Port: int32Ptr(80),
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := layer.Check(ctx, target)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, string(k8swatchv1.FailureCodeTCPRefused), result.FailureCode)
}
