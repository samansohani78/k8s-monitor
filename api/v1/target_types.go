// Package v1 contains API Schema definitions for the k8swatch.io v1 API group
// +kubebuilder:object:generate=true
// +groupName=k8swatch.io
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="Endpoint",type=string,JSONPath=`.spec.endpoint.dns`
// +kubebuilder:printcolumn:name="Interval",type=string,JSONPath=`.spec.schedule.interval`
// +kubebuilder:printcolumn:name="Criticality",type=string,JSONPath=`.spec.alerting.criticalityOverride`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Target represents a monitoring target configuration for health checks
type Target struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TargetSpec   `json:"spec,omitempty"`
	Status TargetStatus `json:"status,omitempty"`
}

// TargetSpec defines the desired state of a Target
type TargetSpec struct {
	// Type is the target type which determines the checker implementation
	// +kubebuilder:validation:Enum=network;dns;http;https;kubernetes;redis;postgresql;mysql;mssql;mongodb;clickhouse;elasticsearch;opensearch;minio;kafka;rabbitmq;keycloak;nginx;internal-canary;external-http;node-egress;node-to-node
	Type TargetType `json:"type"`

	// Endpoint defines how to reach the target
	Endpoint EndpointConfig `json:"endpoint"`

	// NetworkModes specifies which network perspectives to check from
	// +kubebuilder:validation:MinItems=1
	NetworkModes []NetworkMode `json:"networkModes,omitempty"`

	// Layers configures which health check layers to execute
	Layers LayerConfig `json:"layers,omitempty"`

	// Schedule defines the check execution schedule
	Schedule ScheduleConfig `json:"schedule,omitempty"`

	// Alerting overrides for this target
	Alerting AlertingConfig `json:"alerting,omitempty"`

	// Tags for categorization and filtering
	Tags map[string]string `json:"tags,omitempty"`
}

// TargetType represents the type of target being monitored
type TargetType string

const (
	TargetTypeNetwork        TargetType = "network"
	TargetTypeDNS            TargetType = "dns"
	TargetTypeHTTP           TargetType = "http"
	TargetTypeHTTPS          TargetType = "https"
	TargetTypeKubernetes     TargetType = "kubernetes"
	TargetTypeRedis          TargetType = "redis"
	TargetTypePostgreSQL     TargetType = "postgresql"
	TargetTypeMySQL          TargetType = "mysql"
	TargetTypeMSSQL          TargetType = "mssql"
	TargetTypeMongoDB        TargetType = "mongodb"
	TargetTypeClickHouse     TargetType = "clickhouse"
	TargetTypeElasticsearch  TargetType = "elasticsearch"
	TargetTypeOpenSearch     TargetType = "opensearch"
	TargetTypeMinIO          TargetType = "minio"
	TargetTypeKafka          TargetType = "kafka"
	TargetTypeRabbitMQ       TargetType = "rabbitmq"
	TargetTypeKeycloak       TargetType = "keycloak"
	TargetTypeNginx          TargetType = "nginx"
	TargetTypeInternalCanary TargetType = "internal-canary"
	TargetTypeExternalHTTP   TargetType = "external-http"
	TargetTypeNodeEgress     TargetType = "node-egress"
	TargetTypeNodeToNode     TargetType = "node-to-node"
)

// EndpointConfig defines how to reach a target
type EndpointConfig struct {
	// K8sService references a Kubernetes Service
	K8sService *K8sServiceEndpoint `json:"k8sService,omitempty"`

	// DNS is a DNS hostname to resolve
	DNS *string `json:"dns,omitempty"`

	// IP is a direct IP address to connect to
	IP *string `json:"ip,omitempty"`

	// Port is the target port (required for IP endpoints, optional for others)
	Port *int32 `json:"port,omitempty"`

	// Path is the URL path for HTTP/HTTPS targets (default: "/")
	Path *string `json:"path,omitempty"`
}

// K8sServiceEndpoint references a Kubernetes Service
type K8sServiceEndpoint struct {
	// Name is the service name
	Name string `json:"name"`

	// Namespace is the service namespace (default: "default")
	Namespace string `json:"namespace,omitempty"`

	// Port is the service port number or name
	Port string `json:"port"`
}

// NetworkMode specifies the network perspective for checks
type NetworkMode string

const (
	// NetworkModePod uses the pod network (CNI overlay)
	NetworkModePod NetworkMode = "pod"
	// NetworkModeHost uses the host network (node routing)
	NetworkModeHost NetworkMode = "host"
)

// LayerConfig configures which health check layers to execute
type LayerConfig struct {
	// L0NodeSanity enables node sanity checks
	L0NodeSanity *NodeSanityConfig `json:"L0_nodeSanity,omitempty"`

	// L1DNS enables DNS resolution checks
	L1DNS *LayerConfigBase `json:"L1_dns,omitempty"`

	// L2TCP enables TCP connectivity checks
	L2TCP *LayerConfigBase `json:"L2_tcp,omitempty"`

	// L3TLS enables TLS handshake and certificate validation
	L3TLS *TLSConfig `json:"L3_tls,omitempty"`

	// L4Protocol enables protocol-level health checks
	L4Protocol *ProtocolConfig `json:"L4_protocol,omitempty"`

	// L5Auth enables authentication/authorization checks
	L5Auth *AuthConfig `json:"L5_auth,omitempty"`

	// L6Semantic enables semantic/functional checks
	L6Semantic *SemanticConfig `json:"L6_semantic,omitempty"`
}

// LayerConfigBase is the base configuration for a layer
type LayerConfigBase struct {
	// Enabled specifies if this layer should be executed
	Enabled bool `json:"enabled"`

	// Timeout is the maximum time to wait for this layer
	Timeout string `json:"timeout,omitempty"`
}

// NodeSanityConfig configures L0 node sanity checks
type NodeSanityConfig struct {
	LayerConfigBase `json:",inline"`

	// ClockSkew configures clock skew checking
	ClockSkew *ClockSkewConfig `json:"clockSkew,omitempty"`

	// FileDescriptors configures FD exhaustion checking
	FileDescriptors *ThresholdConfig `json:"fileDescriptors,omitempty"`

	// EphemeralPorts configures ephemeral port checking
	EphemeralPorts *ThresholdConfig `json:"ephemeralPorts,omitempty"`

	// Conntrack configures conntrack table checking
	Conntrack *ThresholdConfig `json:"conntrack,omitempty"`
}

// ClockSkewConfig configures clock skew checking
type ClockSkewConfig struct {
	// Enabled specifies if clock skew checking is enabled
	Enabled bool `json:"enabled"`

	// Threshold is the maximum allowed clock skew
	Threshold string `json:"threshold,omitempty"`

	// NTPServer is the NTP server to compare against (optional)
	NTPServer string `json:"ntpServer,omitempty"`
}

// ThresholdConfig configures threshold-based checks
type ThresholdConfig struct {
	// Enabled specifies if this check is enabled
	Enabled bool `json:"enabled"`

	// WarningThreshold is the warning threshold percentage (0-100)
	WarningThreshold int32 `json:"warningThreshold,omitempty"`

	// CriticalThreshold is the critical threshold percentage (0-100)
	CriticalThreshold int32 `json:"criticalThreshold,omitempty"`
}

// TLSConfig configures TLS handshake and certificate validation
type TLSConfig struct {
	LayerConfigBase `json:",inline"`

	// ValidationMode is the certificate validation mode
	// +kubebuilder:validation:Enum=strict;permissive
	ValidationMode string `json:"validationMode,omitempty"`

	// CABundleRef references a Secret containing CA certificates
	CABundleRef *SecretKeyRef `json:"caBundleRef,omitempty"`

	// ClientCertRef references a Secret containing client certificate for mTLS
	ClientCertRef *TLSCertRef `json:"clientCertRef,omitempty"`

	// InsecureSkipVerify disables certificate verification (not recommended)
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

// TLSCertRef references a TLS certificate in a Secret
type TLSCertRef struct {
	// SecretName is the name of the Secret
	SecretName string `json:"secretName"`

	// CertKey is the key containing the certificate
	CertKey string `json:"certKey"`

	// KeyKey is the key containing the private key
	KeyKey string `json:"keyKey"`
}

// ProtocolConfig configures protocol-level health checks
type ProtocolConfig struct {
	LayerConfigBase `json:",inline"`

	// HealthQuery is a protocol-specific health check query
	HealthQuery string `json:"healthQuery,omitempty"`

	// ExpectedResponse is the expected response pattern
	ExpectedResponse string `json:"expectedResponse,omitempty"`

	// Method is the HTTP method for HTTP targets (default: "GET")
	Method string `json:"method,omitempty"`

	// StatusCode is the expected HTTP status code (default: 200-299)
	StatusCode *int32 `json:"statusCode,omitempty"`

	// Headers are custom HTTP headers
	Headers map[string]string `json:"headers,omitempty"`

	// Body is the request body for POST/PUT requests
	Body string `json:"body,omitempty"`
}

// AuthConfig configures authentication/authorization checks
type AuthConfig struct {
	LayerConfigBase `json:",inline"`

	// CredentialsRef references a Secret containing credentials
	CredentialsRef *CredentialsRef `json:"credentialsRef,omitempty"`

	// AuthType is the authentication type
	// +kubebuilder:validation:Enum=basic;bearer;mtls;sasl;scram;apikey
	AuthType string `json:"authType,omitempty"`

	// Token is a bearer token (alternative to CredentialsRef)
	Token string `json:"token,omitempty"`
}

// CredentialsRef references credentials stored in a Secret
type CredentialsRef struct {
	// SecretName is the name of the Secret
	SecretName string `json:"secretName"`

	// SecretNamespace is the namespace of the Secret
	SecretNamespace string `json:"secretNamespace,omitempty"`

	// UsernameKey is the key containing the username
	UsernameKey string `json:"usernameKey,omitempty"`

	// PasswordKey is the key containing the password
	PasswordKey string `json:"passwordKey,omitempty"`

	// TokenKey is the key containing an API token
	TokenKey string `json:"tokenKey,omitempty"`
}

// SecretKeyRef references a value in a Secret
type SecretKeyRef struct {
	// SecretName is the name of the Secret
	SecretName string `json:"secretName"`

	// SecretNamespace is the namespace of the Secret
	SecretNamespace string `json:"secretNamespace,omitempty"`

	// Key is the key in the Secret
	Key string `json:"key"`
}

// SemanticConfig configures semantic/functional health checks
type SemanticConfig struct {
	LayerConfigBase `json:",inline"`

	// ExpectedContent is the expected content pattern in response
	ExpectedContent string `json:"expectedContent,omitempty"`

	// JSONPath is a JSONPath expression to evaluate
	JSONPath string `json:"jsonPath,omitempty"`

	// ExpectedValue is the expected value after JSONPath evaluation
	ExpectedValue string `json:"expectedValue,omitempty"`

	// Regex is a regular expression to match against response
	Regex string `json:"regex,omitempty"`
}

// ScheduleConfig defines the check execution schedule
type ScheduleConfig struct {
	// Interval is the time between checks (e.g., "30s", "1m")
	Interval string `json:"interval"`

	// Timeout is the total timeout for all layers (e.g., "15s")
	Timeout string `json:"timeout,omitempty"`

	// Retries is the number of retries on failure
	Retries int32 `json:"retries,omitempty"`

	// RetryBackoff is the backoff between retries (e.g., "1s")
	RetryBackoff string `json:"retryBackoff,omitempty"`
}

// AlertingConfig defines alerting overrides for a target
type AlertingConfig struct {
	// CriticalityOverride overrides the target criticality
	// +kubebuilder:validation:Enum=P0;P1;P2;P3
	CriticalityOverride string `json:"criticalityOverride,omitempty"`

	// CustomThresholds overrides default alerting thresholds
	CustomThresholds *CustomThresholds `json:"customThresholds,omitempty"`

	// NotificationChannels specifies which channels to use
	NotificationChannels []string `json:"notificationChannels,omitempty"`

	// SuppressAlerts disables alerting for this target
	SuppressAlerts bool `json:"suppressAlerts,omitempty"`
}

// CustomThresholds defines custom alerting thresholds
type CustomThresholds struct {
	// ConsecutiveFailures is the number of consecutive failures before alerting
	ConsecutiveFailures int32 `json:"consecutiveFailures,omitempty"`

	// RecoverySuccesses is the number of consecutive successes before resolving
	RecoverySuccesses int32 `json:"recoverySuccesses,omitempty"`
}

// TargetStatus defines the observed state of a Target
type TargetStatus struct {
	// ObservedGeneration is the last observed generation
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// LastCheckTime is the time of the last health check
	LastCheckTime *metav1.Time `json:"lastCheckTime,omitempty"`

	// LastCheckStatus is the status of the last check
	LastCheckStatus *CheckStatus `json:"lastCheckStatus,omitempty"`

	// ConsecutiveFailures is the current consecutive failure count
	ConsecutiveFailures int32 `json:"consecutiveFailures,omitempty"`

	// Conditions represent the current conditions of the target
	Conditions []TargetCondition `json:"conditions,omitempty"`
}

// CheckStatus represents the status of a health check
type CheckStatus struct {
	// Success indicates if the check was successful
	Success bool `json:"success"`

	// FailureLayer is the layer where failure occurred (if any)
	FailureLayer string `json:"failureLayer,omitempty"`

	// FailureCode is the specific failure code
	FailureCode string `json:"failureCode,omitempty"`

	// FailureMessage is a human-readable failure message
	FailureMessage string `json:"failureMessage,omitempty"`

	// LatencyMs is the total check latency in milliseconds
	LatencyMs int64 `json:"latencyMs,omitempty"`
}

// TargetCondition represents a condition of a Target
type TargetCondition struct {
	// Type is the type of condition
	Type TargetConditionType `json:"type"`

	// Status is the status of the condition
	Status metav1.ConditionStatus `json:"status"`

	// LastTransitionTime is the last time the condition transitioned
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`

	// Reason is a one-word reason for the condition
	Reason string `json:"reason,omitempty"`

	// Message is a human-readable message
	Message string `json:"message,omitempty"`
}

// TargetConditionType is a type of Target condition
type TargetConditionType string

const (
	// TargetConditionHealthy indicates the target is healthy
	TargetConditionHealthy TargetConditionType = "Healthy"
	// TargetConditionDegraded indicates the target is degraded
	TargetConditionDegraded TargetConditionType = "Degraded"
	// TargetConditionUnhealthy indicates the target is unhealthy
	TargetConditionUnhealthy TargetConditionType = "Unhealthy"
)

// +kubebuilder:object:root=true

// TargetList contains a list of Target
type TargetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Target `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Target{}, &TargetList{})
}
