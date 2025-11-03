package knative

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// domainRegex matches valid domain names
	domainRegex = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)

	// subdomainRegex matches valid subdomains
	subdomainRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?$`)
)

// DomainConfig contains domain configuration for a service
type DomainConfig struct {
	Primary    string   // Primary domain
	Additional []string // Additional domains (aliases)
}

// ValidateDomain validates a domain name format
func ValidateDomain(domain string) error {
	if domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}

	// Trim whitespace
	domain = strings.TrimSpace(domain)

	// Check length
	if len(domain) > 253 {
		return fmt.Errorf("domain is too long (max 253 characters)")
	}

	// Check format
	if !domainRegex.MatchString(domain) {
		return fmt.Errorf("invalid domain format: %s", domain)
	}

	// Check for valid TLD
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return fmt.Errorf("domain must have at least one subdomain and TLD")
	}

	// Validate each part
	for _, part := range parts {
		if len(part) == 0 {
			return fmt.Errorf("domain parts cannot be empty")
		}
		if len(part) > 63 {
			return fmt.Errorf("domain part '%s' is too long (max 63 characters)", part)
		}
	}

	return nil
}

// ValidateDomains validates multiple domains
func ValidateDomains(domains []string) error {
	if len(domains) == 0 {
		return fmt.Errorf("at least one domain is required")
	}

	seen := make(map[string]bool)
	for _, domain := range domains {
		domain = strings.TrimSpace(domain)

		if err := ValidateDomain(domain); err != nil {
			return fmt.Errorf("invalid domain '%s': %w", domain, err)
		}

		// Check for duplicates
		if seen[domain] {
			return fmt.Errorf("duplicate domain: %s", domain)
		}
		seen[domain] = true
	}

	return nil
}

// IsApexDomain checks if a domain is an apex domain (no subdomain)
func IsApexDomain(domain string) bool {
	parts := strings.Split(domain, ".")
	return len(parts) == 2
}

// GetSubdomain extracts the subdomain from a full domain
// e.g., "app.example.com" -> "app"
func GetSubdomain(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) <= 2 {
		return ""
	}
	return parts[0]
}

// GetBaseDomain extracts the base domain (apex domain) from a full domain
// e.g., "app.example.com" -> "example.com"
func GetBaseDomain(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) <= 2 {
		return domain
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

// NormalizeDomain normalizes a domain name (lowercase, trim)
func NormalizeDomain(domain string) string {
	return strings.ToLower(strings.TrimSpace(domain))
}

// NormalizeDomains normalizes multiple domains
func NormalizeDomains(domains []string) []string {
	normalized := make([]string, len(domains))
	for i, domain := range domains {
		normalized[i] = NormalizeDomain(domain)
	}
	return normalized
}
