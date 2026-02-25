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

func TestMongoDBCheckerFactory_Create(t *testing.T) {
	factory := &MongoDBCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeMongoDB,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS:  strPtr("mongodb.default.svc"),
				Port: int32Ptr(27017),
			},
		},
	}

	checker, err := factory.Create(target)
	assert.NoError(t, err)
	assert.NotNil(t, checker)
	assert.Len(t, checker.Layers(), 6)
}

func TestMongoDBCheckerFactory_SupportedTypes(t *testing.T) {
	factory := &MongoDBCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Equal(t, []string{"mongodb"}, types)
}

func TestMongoDBChecker_Execute(t *testing.T) {
	factory := &MongoDBCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeMongoDB,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS:  strPtr("mongodb.invalid.svc"),
				Port: int32Ptr(27017),
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

func TestMongoDBProtocolLayer_Name(t *testing.T) {
	layer := NewMongoDBProtocolLayer()
	assert.Equal(t, "L4", layer.Name())
}

func TestMongoDBProtocolLayer_Enabled(t *testing.T) {
	layer := NewMongoDBProtocolLayer()
	
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeMongoDB,
		},
	}
	assert.True(t, layer.Enabled(target))
}

func TestMongoDBProtocolLayer_Check_NoEndpoint(t *testing.T) {
	layer := NewMongoDBProtocolLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeMongoDB,
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

func TestMongoDBProtocolLayer_Check_ConnectionRefused(t *testing.T) {
	layer := NewMongoDBProtocolLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeMongoDB,
			Endpoint: k8swatchv1.EndpointConfig{
				IP:   strPtr("127.0.0.1"),
				Port: int32Ptr(27017),
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

func TestMongoDBAuthLayer_Name(t *testing.T) {
	layer := NewMongoDBAuthLayer()
	assert.Equal(t, "L5", layer.Name())
}

func TestMongoDBAuthLayer_Enabled(t *testing.T) {
	layer := NewMongoDBAuthLayer()
	
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Layers: k8swatchv1.LayerConfig{
				L5Auth: &k8swatchv1.AuthConfig{
					LayerConfigBase: k8swatchv1.LayerConfigBase{
						Enabled: true,
					},
				},
			},
		},
	}
	assert.True(t, layer.Enabled(target))
}

func TestMongoDBSemanticLayer_Name(t *testing.T) {
	layer := NewMongoDBSemanticLayer()
	assert.Equal(t, "L6", layer.Name())
}

func TestMongoDBSemanticLayer_Enabled(t *testing.T) {
	layer := NewMongoDBSemanticLayer()
	
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
}

func TestElasticsearchCheckerFactory_Create(t *testing.T) {
	factory := &ElasticsearchCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeElasticsearch,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS:  strPtr("elasticsearch.default.svc"),
				Port: int32Ptr(9200),
			},
		},
	}

	checker, err := factory.Create(target)
	assert.NoError(t, err)
	assert.NotNil(t, checker)
	assert.Len(t, checker.Layers(), 7)
}

func TestElasticsearchCheckerFactory_SupportedTypes(t *testing.T) {
	factory := &ElasticsearchCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Equal(t, []string{"elasticsearch"}, types)
}

func TestElasticsearchProtocolLayer_Name(t *testing.T) {
	layer := NewElasticsearchProtocolLayer()
	assert.Equal(t, "L4", layer.Name())
}

func TestElasticsearchProtocolLayer_Enabled(t *testing.T) {
	layer := NewElasticsearchProtocolLayer()
	
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeElasticsearch,
		},
	}
	assert.True(t, layer.Enabled(target))
}

func TestElasticsearchProtocolLayer_Check_NoEndpoint(t *testing.T) {
	layer := NewElasticsearchProtocolLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeElasticsearch,
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

func TestElasticsearchProtocolLayer_Check_ConnectionRefused(t *testing.T) {
	layer := NewElasticsearchProtocolLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeElasticsearch,
			Endpoint: k8swatchv1.EndpointConfig{
				IP:   strPtr("127.0.0.1"),
				Port: int32Ptr(9200),
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

func TestElasticsearchAuthLayer_Name(t *testing.T) {
	layer := NewElasticsearchAuthLayer()
	assert.Equal(t, "L5", layer.Name())
}

func TestElasticsearchAuthLayer_Enabled(t *testing.T) {
	layer := NewElasticsearchAuthLayer()
	
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Layers: k8swatchv1.LayerConfig{
				L5Auth: &k8swatchv1.AuthConfig{
					LayerConfigBase: k8swatchv1.LayerConfigBase{
						Enabled: true,
					},
				},
			},
		},
	}
	assert.True(t, layer.Enabled(target))
}

func TestElasticsearchSemanticLayer_Name(t *testing.T) {
	layer := NewElasticsearchSemanticLayer()
	assert.Equal(t, "L6", layer.Name())
}

func TestElasticsearchSemanticLayer_Enabled(t *testing.T) {
	layer := NewElasticsearchSemanticLayer()
	
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
}

func TestOpenSearchCheckerFactory_Create(t *testing.T) {
	factory := &OpenSearchCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeOpenSearch,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS:  strPtr("opensearch.default.svc"),
				Port: int32Ptr(9200),
			},
		},
	}

	checker, err := factory.Create(target)
	assert.NoError(t, err)
	assert.NotNil(t, checker)
	assert.Len(t, checker.Layers(), 5)
}

func TestOpenSearchCheckerFactory_SupportedTypes(t *testing.T) {
	factory := &OpenSearchCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Equal(t, []string{"opensearch"}, types)
}

func TestOpenSearchProtocolLayer_Name(t *testing.T) {
	layer := NewOpenSearchProtocolLayer()
	assert.Equal(t, "L4", layer.Name())
}

func TestOpenSearchProtocolLayer_Enabled(t *testing.T) {
	layer := NewOpenSearchProtocolLayer()
	
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeOpenSearch,
		},
	}
	assert.True(t, layer.Enabled(target))
}

func TestOpenSearchProtocolLayer_Check_NoEndpoint(t *testing.T) {
	layer := NewOpenSearchProtocolLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeOpenSearch,
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

func TestOpenSearchProtocolLayer_Check_ConnectionRefused(t *testing.T) {
	layer := NewOpenSearchProtocolLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeOpenSearch,
			Endpoint: k8swatchv1.EndpointConfig{
				IP:   strPtr("127.0.0.1"),
				Port: int32Ptr(9200),
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
