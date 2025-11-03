package k8s

import (
	"k8s.io/client-go/kubernetes"
)

// MockClient implements ClientInterface for testing
type MockClient struct {
	Clientset     *kubernetes.Clientset
	TestConnError error
	ServerVersion string
	VersionError  error
}

// NewMockClient creates a new mock client for testing
func NewMockClient() *MockClient {
	return &MockClient{
		ServerVersion: "v1.28.0",
	}
}

// GetClientset returns the mock clientset
func (m *MockClient) GetClientset() *kubernetes.Clientset {
	return m.Clientset
}

// TestConnection returns the configured test connection error
func (m *MockClient) TestConnection() error {
	return m.TestConnError
}

// GetServerVersion returns the configured server version
func (m *MockClient) GetServerVersion() (string, error) {
	if m.VersionError != nil {
		return "", m.VersionError
	}
	return m.ServerVersion, nil
}

// WithConnectionError configures the mock to return an error on TestConnection
func (m *MockClient) WithConnectionError(err error) *MockClient {
	m.TestConnError = err
	return m
}

// WithServerVersion configures the mock server version
func (m *MockClient) WithServerVersion(version string) *MockClient {
	m.ServerVersion = version
	return m
}

// WithVersionError configures the mock to return an error on GetServerVersion
func (m *MockClient) WithVersionError(err error) *MockClient {
	m.VersionError = err
	return m
}
