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

func TestMySQLCheckerFactory_Create(t *testing.T) {
	factory := &MySQLCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeMySQL,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS:  strPtr("mysql.default.svc"),
				Port: int32Ptr(3306),
			},
		},
	}

	checker, err := factory.Create(target)
	assert.NoError(t, err)
	assert.NotNil(t, checker)
	assert.Len(t, checker.Layers(), 6)
}

func TestMySQLCheckerFactory_SupportedTypes(t *testing.T) {
	factory := &MySQLCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Equal(t, []string{"mysql"}, types)
}

func TestMySQLChecker_Execute(t *testing.T) {
	factory := &MySQLCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeMySQL,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS:  strPtr("mysql.invalid.svc"),
				Port: int32Ptr(3306),
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

func TestMySQLProtocolLayer_Name(t *testing.T) {
	layer := NewMySQLProtocolLayer()
	assert.Equal(t, "L4", layer.Name())
}

func TestMySQLProtocolLayer_Enabled(t *testing.T) {
	layer := NewMySQLProtocolLayer()
	
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeMySQL,
		},
	}
	assert.True(t, layer.Enabled(target))
}

func TestMySQLProtocolLayer_Check_NoEndpoint(t *testing.T) {
	layer := NewMySQLProtocolLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeMySQL,
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

func TestMySQLProtocolLayer_Check_ConnectionRefused(t *testing.T) {
	layer := NewMySQLProtocolLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeMySQL,
			Endpoint: k8swatchv1.EndpointConfig{
				IP:   strPtr("127.0.0.1"),
				Port: int32Ptr(3306),
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

func TestMySQLAuthLayer_Name(t *testing.T) {
	layer := NewMySQLAuthLayer()
	assert.Equal(t, "L5", layer.Name())
}

func TestMySQLAuthLayer_Enabled(t *testing.T) {
	layer := NewMySQLAuthLayer()
	
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

func TestMySQLSemanticLayer_Name(t *testing.T) {
	layer := NewMySQLSemanticLayer()
	assert.Equal(t, "L6", layer.Name())
}

func TestMySQLSemanticLayer_Enabled(t *testing.T) {
	layer := NewMySQLSemanticLayer()
	
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

func TestMSSQLCheckerFactory_Create(t *testing.T) {
	factory := &MSSQLCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeMSSQL,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS:  strPtr("mssql.default.svc"),
				Port: int32Ptr(1433),
			},
		},
	}

	checker, err := factory.Create(target)
	assert.NoError(t, err)
	assert.NotNil(t, checker)
	assert.Len(t, checker.Layers(), 4)
}

func TestMSSQLCheckerFactory_SupportedTypes(t *testing.T) {
	factory := &MSSQLCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Equal(t, []string{"mssql"}, types)
}

func TestMSSQLChecker_Execute(t *testing.T) {
	factory := &MSSQLCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeMSSQL,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS:  strPtr("mssql.invalid.svc"),
				Port: int32Ptr(1433),
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

func TestMSSQLProtocolLayer_Name(t *testing.T) {
	layer := NewMSSQLProtocolLayer()
	assert.Equal(t, "L4", layer.Name())
}

func TestMSSQLProtocolLayer_Enabled(t *testing.T) {
	layer := NewMSSQLProtocolLayer()
	
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeMSSQL,
		},
	}
	assert.True(t, layer.Enabled(target))
}

func TestMSSQLProtocolLayer_Check_NoEndpoint(t *testing.T) {
	layer := NewMSSQLProtocolLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeMSSQL,
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

func TestMSSQLProtocolLayer_Check_ConnectionRefused(t *testing.T) {
	layer := NewMSSQLProtocolLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeMSSQL,
			Endpoint: k8swatchv1.EndpointConfig{
				IP:   strPtr("127.0.0.1"),
				Port: int32Ptr(1433),
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

func TestClickHouseCheckerFactory_Create(t *testing.T) {
	factory := &ClickHouseCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeClickHouse,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS:  strPtr("clickhouse.default.svc"),
				Port: int32Ptr(9000),
			},
		},
	}

	checker, err := factory.Create(target)
	assert.NoError(t, err)
	assert.NotNil(t, checker)
	assert.Len(t, checker.Layers(), 4)
}

func TestClickHouseCheckerFactory_SupportedTypes(t *testing.T) {
	factory := &ClickHouseCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Equal(t, []string{"clickhouse"}, types)
}

func TestClickHouseChecker_Execute(t *testing.T) {
	factory := &ClickHouseCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeClickHouse,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS:  strPtr("clickhouse.invalid.svc"),
				Port: int32Ptr(9000),
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

func TestClickHouseProtocolLayer_Name(t *testing.T) {
	layer := NewClickHouseProtocolLayer()
	assert.Equal(t, "L4", layer.Name())
}

func TestClickHouseProtocolLayer_Enabled(t *testing.T) {
	layer := NewClickHouseProtocolLayer()
	
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeClickHouse,
		},
	}
	assert.True(t, layer.Enabled(target))
}

func TestClickHouseProtocolLayer_Check_NoEndpoint(t *testing.T) {
	layer := NewClickHouseProtocolLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeClickHouse,
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

func TestClickHouseProtocolLayer_Check_ConnectionRefused(t *testing.T) {
	layer := NewClickHouseProtocolLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeClickHouse,
			Endpoint: k8swatchv1.EndpointConfig{
				IP:   strPtr("127.0.0.1"),
				Port: int32Ptr(9000),
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

func TestMinIOCheckerFactory_Create(t *testing.T) {
	factory := &MinIOCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeMinIO,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS:  strPtr("minio.default.svc"),
				Port: int32Ptr(9000),
			},
		},
	}

	checker, err := factory.Create(target)
	assert.NoError(t, err)
	assert.NotNil(t, checker)
	assert.Len(t, checker.Layers(), 4)
}

func TestMinIOCheckerFactory_SupportedTypes(t *testing.T) {
	factory := &MinIOCheckerFactory{}
	types := factory.SupportedTypes()
	assert.Equal(t, []string{"minio"}, types)
}

func TestMinIOChecker_Execute(t *testing.T) {
	factory := &MinIOCheckerFactory{}
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeMinIO,
			Endpoint: k8swatchv1.EndpointConfig{
				DNS:  strPtr("minio.invalid.svc"),
				Port: int32Ptr(9000),
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

func TestMinIOProtocolLayer_Name(t *testing.T) {
	layer := NewMinIOProtocolLayer()
	assert.Equal(t, "L4", layer.Name())
}

func TestMinIOProtocolLayer_Enabled(t *testing.T) {
	layer := NewMinIOProtocolLayer()
	
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeMinIO,
		},
	}
	assert.True(t, layer.Enabled(target))
}

func TestMinIOProtocolLayer_Check_NoEndpoint(t *testing.T) {
	layer := NewMinIOProtocolLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeMinIO,
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

func TestMinIOProtocolLayer_Check_ConnectionRefused(t *testing.T) {
	layer := NewMinIOProtocolLayer()
	target := &k8swatchv1.Target{
		Spec: k8swatchv1.TargetSpec{
			Type: k8swatchv1.TargetTypeMinIO,
			Endpoint: k8swatchv1.EndpointConfig{
				IP:   strPtr("127.0.0.1"),
				Port: int32Ptr(9000),
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
