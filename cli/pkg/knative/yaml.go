package knative

import (
	"sigs.k8s.io/yaml"
)

// ToYAML converts a Service to YAML format
func (s *Service) ToYAML() ([]byte, error) {
	return yaml.Marshal(s)
}

// ToYAMLString converts a Service to YAML string
func (s *Service) ToYAMLString() (string, error) {
	data, err := s.ToYAML()
	if err != nil {
		return "", err
	}
	return string(data), nil
}
