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

package v1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test GetScheme function
func TestGetScheme(t *testing.T) {
	scheme := GetScheme()
	
	assert.NotNil(t, scheme)
	
	// Verify that our types are registered
	knownTypes := scheme.KnownTypes(GroupVersion)
	assert.NotEmpty(t, knownTypes)
	
	// Check that Target is registered
	_, exists := knownTypes["Target"]
	assert.True(t, exists, "Target should be registered in scheme")
	
	// Check that AlertEvent is registered
	_, exists = knownTypes["AlertEvent"]
	assert.True(t, exists, "AlertEvent should be registered in scheme")
}

// Test AddToScheme
func TestAddToScheme(t *testing.T) {
	scheme := GetScheme()
	
	// AddToScheme should not fail
	err := AddToScheme(scheme)
	assert.NoError(t, err)
}

// Test GroupVersion
func TestGroupVersion(t *testing.T) {
	assert.Equal(t, "k8swatch.io", GroupVersion.Group)
	assert.Equal(t, "v1", GroupVersion.Version)
}

// Test SchemeBuilder
func TestSchemeBuilder(t *testing.T) {
	assert.NotNil(t, SchemeBuilder)
	assert.Equal(t, GroupVersion, SchemeBuilder.GroupVersion)
}
