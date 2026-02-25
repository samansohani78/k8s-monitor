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

package alertmanager

import (
	"context"

	"github.com/stretchr/testify/assert"
)

// MockNotificationChannel for testing
type MockNotificationChannel struct {
	name      string
	failCount int
	sendCount int
	closed    bool
}

func (m *MockNotificationChannel) Name() string {
	return m.name
}

func (m *MockNotificationChannel) Send(ctx context.Context, alert *Alert) error {
	m.sendCount++
	if m.failCount > 0 {
		m.failCount--
		return assert.AnError
	}
	return nil
}

func (m *MockNotificationChannel) Close() error {
	m.closed = true
	return nil
}
