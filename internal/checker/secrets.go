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
	"fmt"
	"os"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	k8swatchv1 "github.com/k8swatch/k8s-monitor/api/v1"
)

type resolvedCredentials struct {
	username string
	password string
	token    string
}

var (
	secretClientOnce sync.Once
	secretClient     kubernetes.Interface
	secretClientErr  error
)

func getSecretClient() (kubernetes.Interface, error) {
	secretClientOnce.Do(func() {
		var cfg *rest.Config
		var err error

		if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
			cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		} else {
			cfg, err = rest.InClusterConfig()
			if err != nil {
				loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
				clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
					loadingRules,
					&clientcmd.ConfigOverrides{},
				)
				cfg, err = clientConfig.ClientConfig()
			}
		}
		if err != nil {
			secretClientErr = fmt.Errorf("failed to initialize Kubernetes config: %w", err)
			return
		}

		secretClient, secretClientErr = kubernetes.NewForConfig(cfg)
		if secretClientErr != nil {
			secretClientErr = fmt.Errorf("failed to create Kubernetes client: %w", secretClientErr)
		}
	})

	return secretClient, secretClientErr
}

func resolveSecretNamespace(targetNamespace, refNamespace string) string {
	if refNamespace != "" {
		return refNamespace
	}
	if targetNamespace != "" {
		return targetNamespace
	}
	return "default"
}

func loadSecret(ctx context.Context, namespace, name string) (*corev1.Secret, error) {
	clientset, err := getSecretClient()
	if err != nil {
		return nil, err
	}

	secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to load secret %s/%s: %w", namespace, name, err)
	}

	return secret, nil
}

func loadSecretKeyRef(ctx context.Context, targetNamespace string, ref *k8swatchv1.SecretKeyRef) (string, error) {
	if ref == nil {
		return "", fmt.Errorf("secret reference is nil")
	}

	namespace := resolveSecretNamespace(targetNamespace, ref.SecretNamespace)
	secret, err := loadSecret(ctx, namespace, ref.SecretName)
	if err != nil {
		return "", err
	}

	val, ok := secret.Data[ref.Key]
	if !ok {
		return "", fmt.Errorf("key %q not found in secret %s/%s", ref.Key, namespace, ref.SecretName)
	}

	return string(val), nil
}

func loadTLSCertRef(ctx context.Context, targetNamespace string, ref *k8swatchv1.TLSCertRef) ([]byte, []byte, error) {
	if ref == nil {
		return nil, nil, fmt.Errorf("TLS certificate reference is nil")
	}

	namespace := resolveSecretNamespace(targetNamespace, "")
	secret, err := loadSecret(ctx, namespace, ref.SecretName)
	if err != nil {
		return nil, nil, err
	}

	certBytes, certOK := secret.Data[ref.CertKey]
	keyBytes, keyOK := secret.Data[ref.KeyKey]
	if !certOK || !keyOK {
		return nil, nil, fmt.Errorf(
			"certificate keys (%s,%s) not found in secret %s/%s",
			ref.CertKey,
			ref.KeyKey,
			namespace,
			ref.SecretName,
		)
	}

	return certBytes, keyBytes, nil
}

func loadCredentialsRef(ctx context.Context, targetNamespace string, ref *k8swatchv1.CredentialsRef) (*resolvedCredentials, error) {
	if ref == nil {
		return nil, fmt.Errorf("credentials reference is nil")
	}

	namespace := resolveSecretNamespace(targetNamespace, ref.SecretNamespace)
	secret, err := loadSecret(ctx, namespace, ref.SecretName)
	if err != nil {
		return nil, err
	}

	creds := &resolvedCredentials{}
	if ref.UsernameKey != "" {
		val, ok := secret.Data[ref.UsernameKey]
		if !ok {
			return nil, fmt.Errorf("username key %q not found in secret %s/%s", ref.UsernameKey, namespace, ref.SecretName)
		}
		creds.username = string(val)
	}
	if ref.PasswordKey != "" {
		val, ok := secret.Data[ref.PasswordKey]
		if !ok {
			return nil, fmt.Errorf("password key %q not found in secret %s/%s", ref.PasswordKey, namespace, ref.SecretName)
		}
		creds.password = string(val)
	}
	if ref.TokenKey != "" {
		val, ok := secret.Data[ref.TokenKey]
		if !ok {
			return nil, fmt.Errorf("token key %q not found in secret %s/%s", ref.TokenKey, namespace, ref.SecretName)
		}
		creds.token = string(val)
	}

	return creds, nil
}
