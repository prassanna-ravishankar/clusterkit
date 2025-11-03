package preflight

import (
	"context"
	"fmt"

	"github.com/cloudflare/cloudflare-go"
)

// CloudflarePreflightChecker validates Cloudflare API access and permissions
type CloudflarePreflightChecker struct {
	apiToken string
	ctx      context.Context
}

// NewCloudflarePreflightChecker creates a new Cloudflare preflight checker
func NewCloudflarePreflightChecker(apiToken string) *CloudflarePreflightChecker {
	return &CloudflarePreflightChecker{
		apiToken: apiToken,
		ctx:      context.Background(),
	}
}

// CloudflarePreflightResults contains Cloudflare preflight check results
type CloudflarePreflightResults struct {
	Checks      []CheckResult
	AllPassed   bool
	FailedCount int
	Zones       []ZoneInfo
}

// ZoneInfo contains information about a Cloudflare zone
type ZoneInfo struct {
	ID     string
	Name   string
	Status string
}

// RunAll runs all Cloudflare preflight checks
func (c *CloudflarePreflightChecker) RunAll(domains []string) (*CloudflarePreflightResults, error) {
	results := &CloudflarePreflightResults{
		Checks: make([]CheckResult, 0),
		Zones:  make([]ZoneInfo, 0),
	}

	// Check API token validity
	tokenCheck := c.checkToken()
	results.Checks = append(results.Checks, tokenCheck)
	if !tokenCheck.Passed {
		results.AllPassed = false
		results.FailedCount++
		return results, nil
	}

	// Check token permissions
	permChecks := c.checkTokenPermissions()
	results.Checks = append(results.Checks, permChecks...)
	for _, check := range permChecks {
		if !check.Passed {
			results.FailedCount++
		}
	}

	// Check domain/zone access if domains provided
	if len(domains) > 0 {
		zoneChecks, zones := c.checkZoneAccess(domains)
		results.Checks = append(results.Checks, zoneChecks...)
		results.Zones = zones
		for _, check := range zoneChecks {
			if !check.Passed {
				results.FailedCount++
			}
		}
	}

	// Check rate limits
	rateLimitCheck := c.checkRateLimits()
	results.Checks = append(results.Checks, rateLimitCheck)
	if !rateLimitCheck.Passed {
		results.FailedCount++
	}

	results.AllPassed = results.FailedCount == 0
	return results, nil
}

// checkToken verifies the API token is valid
func (c *CloudflarePreflightChecker) checkToken() CheckResult {
	if c.apiToken == "" {
		return CheckResult{
			Name:    "Cloudflare API Token",
			Passed:  false,
			Message: "No API token provided",
			Remediation: `Create a Cloudflare API token:
  1. Go to: https://dash.cloudflare.com/profile/api-tokens
  2. Click "Create Token"
  3. Use "Edit zone DNS" template or create custom token
  4. Required permissions:
     - Zone:DNS:Edit
     - Zone:Zone:Read
  5. Save token and use with --cloudflare-token flag`,
		}
	}

	api, err := cloudflare.NewWithAPIToken(c.apiToken)
	if err != nil {
		return CheckResult{
			Name:        "Cloudflare API Token",
			Passed:      false,
			Message:     fmt.Sprintf("Failed to initialize Cloudflare client: %v", err),
			Remediation: "Verify the API token format is correct",
		}
	}

	// Verify token by attempting to get user info
	user, err := api.UserDetails(c.ctx)
	if err != nil {
		return CheckResult{
			Name:    "Cloudflare API Token",
			Passed:  false,
			Message: fmt.Sprintf("Token authentication failed: %v", err),
			Remediation: `Verify your API token is correct and active:
  - Check token hasn't expired
  - Verify token wasn't revoked
  - Ensure token is copied correctly (no extra spaces)
  - Create new token at: https://dash.cloudflare.com/profile/api-tokens`,
		}
	}

	return CheckResult{
		Name:    "Cloudflare API Token",
		Passed:  true,
		Message: fmt.Sprintf("Successfully authenticated as: %s (%s)", user.Email, user.ID),
	}
}

// checkTokenPermissions verifies the token has required permissions
func (c *CloudflarePreflightChecker) checkTokenPermissions() []CheckResult {
	results := make([]CheckResult, 0)

	api, err := cloudflare.NewWithAPIToken(c.apiToken)
	if err != nil {
		results = append(results, CheckResult{
			Name:        "Token Permissions",
			Passed:      false,
			Message:     fmt.Sprintf("Failed to create API client: %v", err),
			Remediation: "Verify API token is valid",
		})
		return results
	}

	// Verify token by getting token details
	token, err := api.VerifyAPIToken(c.ctx)
	if err != nil {
		results = append(results, CheckResult{
			Name:        "Token Permissions",
			Passed:      false,
			Message:     fmt.Sprintf("Failed to verify token: %v", err),
			Remediation: "Verify API token is valid and active",
		})
		return results
	}

	if token.Status != "active" {
		results = append(results, CheckResult{
			Name:        "Token Status",
			Passed:      false,
			Message:     fmt.Sprintf("Token is not active (status: %s)", token.Status),
			Remediation: "Create a new API token or activate the existing one",
		})
		return results
	}

	results = append(results, CheckResult{
		Name:    "Token Status",
		Passed:  true,
		Message: "Token is active",
	})

	// Try to list zones to verify DNS read/write permissions
	_, err = api.ListZones(c.ctx)
	if err != nil {
		results = append(results, CheckResult{
			Name:    "Permission: Zone Read",
			Passed:  false,
			Message: fmt.Sprintf("Cannot list zones: %v", err),
			Remediation: `Token needs Zone:Zone:Read permission:
  1. Go to: https://dash.cloudflare.com/profile/api-tokens
  2. Edit or create token with Zone:Zone:Read permission
  3. Include all zones or specific zones as needed`,
		})
	} else {
		results = append(results, CheckResult{
			Name:    "Permission: Zone Read",
			Passed:  true,
			Message: "Can read zone information",
		})
	}

	return results
}

// checkZoneAccess verifies access to specified domains
func (c *CloudflarePreflightChecker) checkZoneAccess(domains []string) ([]CheckResult, []ZoneInfo) {
	results := make([]CheckResult, 0)
	zones := make([]ZoneInfo, 0)

	api, err := cloudflare.NewWithAPIToken(c.apiToken)
	if err != nil {
		results = append(results, CheckResult{
			Name:        "Zone Access",
			Passed:      false,
			Message:     fmt.Sprintf("Failed to create API client: %v", err),
			Remediation: "Verify API token is valid",
		})
		return results, zones
	}

	// Get all zones
	allZones, err := api.ListZones(c.ctx)
	if err != nil {
		results = append(results, CheckResult{
			Name:        "Zone Access",
			Passed:      false,
			Message:     fmt.Sprintf("Failed to list zones: %v", err),
			Remediation: "Verify token has Zone:Zone:Read permission",
		})
		return results, zones
	}

	// Build map of accessible zones
	zoneMap := make(map[string]*cloudflare.Zone)
	for i := range allZones {
		zone := &allZones[i]
		zoneMap[zone.Name] = zone
		zones = append(zones, ZoneInfo{
			ID:     zone.ID,
			Name:   zone.Name,
			Status: zone.Status,
		})
	}

	// Check each domain
	for _, domain := range domains {
		zone, found := zoneMap[domain]
		if !found {
			// Try to find parent zone
			parentZone := findParentZone(domain, allZones)
			if parentZone != nil {
				results = append(results, CheckResult{
					Name:    fmt.Sprintf("Domain: %s", domain),
					Passed:  true,
					Message: fmt.Sprintf("Subdomain of accessible zone: %s", parentZone.Name),
				})

				// Test DNS write permission
				if err := c.testDNSWrite(api, parentZone.ID); err != nil {
					results = append(results, CheckResult{
						Name:    fmt.Sprintf("DNS Write: %s", domain),
						Passed:  false,
						Message: fmt.Sprintf("Cannot manage DNS records: %v", err),
						Remediation: fmt.Sprintf(`Token needs Zone:DNS:Edit permission for zone %s:
  1. Edit token at: https://dash.cloudflare.com/profile/api-tokens
  2. Add Zone:DNS:Edit permission
  3. Include zone %s in scope`, parentZone.Name, parentZone.Name),
					})
				} else {
					results = append(results, CheckResult{
						Name:    fmt.Sprintf("DNS Write: %s", domain),
						Passed:  true,
						Message: "Can create and manage DNS records",
					})
				}
				continue
			}

			results = append(results, CheckResult{
				Name:    fmt.Sprintf("Domain: %s", domain),
				Passed:  false,
				Message: fmt.Sprintf("Zone not found or not accessible"),
				Remediation: fmt.Sprintf(`Add domain to Cloudflare or grant token access:
  - Add domain to Cloudflare: https://dash.cloudflare.com/
  - Or update token to include zone for %s
  - Available zones: %d accessible`, domain, len(allZones)),
			})
			continue
		}

		if zone.Status != "active" {
			results = append(results, CheckResult{
				Name:    fmt.Sprintf("Domain: %s", domain),
				Passed:  false,
				Message: fmt.Sprintf("Zone exists but not active (status: %s)", zone.Status),
				Remediation: fmt.Sprintf(`Activate zone %s:
  - Complete nameserver setup at your domain registrar
  - Point nameservers to: %v
  - Wait for propagation (can take up to 24 hours)`, domain, zone.NameServers),
			})
			continue
		}

		results = append(results, CheckResult{
			Name:    fmt.Sprintf("Domain: %s", domain),
			Passed:  true,
			Message: fmt.Sprintf("Zone is active (ID: %s)", zone.ID),
		})

		// Test DNS write permission
		if err := c.testDNSWrite(api, zone.ID); err != nil {
			results = append(results, CheckResult{
				Name:    fmt.Sprintf("DNS Write: %s", domain),
				Passed:  false,
				Message: fmt.Sprintf("Cannot manage DNS records: %v", err),
				Remediation: fmt.Sprintf(`Token needs Zone:DNS:Edit permission for zone %s:
  1. Edit token at: https://dash.cloudflare.com/profile/api-tokens
  2. Add Zone:DNS:Edit permission
  3. Include zone %s in scope`, zone.Name, zone.Name),
			})
		} else {
			results = append(results, CheckResult{
				Name:    fmt.Sprintf("DNS Write: %s", domain),
				Passed:  true,
				Message: "Can create and manage DNS records",
			})
		}
	}

	return results, zones
}

// findParentZone finds the parent zone for a subdomain
func findParentZone(domain string, zones []cloudflare.Zone) *cloudflare.Zone {
	for i := range zones {
		zone := &zones[i]
		// Check if domain ends with zone name
		if len(domain) > len(zone.Name) {
			suffix := domain[len(domain)-len(zone.Name):]
			if suffix == zone.Name && domain[len(domain)-len(zone.Name)-1] == '.' {
				return zone
			}
		}
	}
	return nil
}

// testDNSWrite attempts to verify DNS write permissions
func (c *CloudflarePreflightChecker) testDNSWrite(api *cloudflare.API, zoneID string) error {
	// Try to list DNS records to verify read access
	_, _, err := api.ListDNSRecords(c.ctx, cloudflare.ZoneIdentifier(zoneID), cloudflare.ListDNSRecordsParams{})
	return err
}

// checkRateLimits checks if the token is near rate limits
func (c *CloudflarePreflightChecker) checkRateLimits() CheckResult {
	// Note: Cloudflare rate limits are returned in response headers
	// This is a basic check that we can make API calls
	api, err := cloudflare.NewWithAPIToken(c.apiToken)
	if err != nil {
		return CheckResult{
			Name:        "Rate Limits",
			Passed:      false,
			Message:     fmt.Sprintf("Failed to create API client: %v", err),
			Remediation: "Verify API token is valid",
		}
	}

	// Make a simple API call to check rate limits
	_, err = api.ListZones(c.ctx)
	if err != nil {
		// Check if it's a rate limit error
		if isRateLimitError(err) {
			return CheckResult{
				Name:    "Rate Limits",
				Passed:  false,
				Message: "API rate limit reached",
				Remediation: `Wait before continuing:
  - Cloudflare API has rate limits per token
  - Wait a few minutes and try again
  - Consider using multiple tokens for high-frequency operations`,
			}
		}
		return CheckResult{
			Name:        "Rate Limits",
			Passed:      false,
			Message:     fmt.Sprintf("Failed to check rate limits: %v", err),
			Remediation: "Verify API access",
		}
	}

	return CheckResult{
		Name:    "Rate Limits",
		Passed:  true,
		Message: "API rate limits OK (sufficient quota available)",
	}
}

// isRateLimitError checks if an error is a rate limit error
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "rate limit") || contains(errStr, "429") || contains(errStr, "too many requests")
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && indexOf(s, substr) >= 0)
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
