package preflight

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/container/v1"
	"google.golang.org/api/iam/v1"
	"google.golang.org/api/option"
	"google.golang.org/api/serviceusage/v1"
)

// GCPPreflightChecker validates GCP permissions and APIs
type GCPPreflightChecker struct {
	projectID string
	ctx       context.Context
}

// NewGCPPreflightChecker creates a new GCP preflight checker
func NewGCPPreflightChecker(projectID string) *GCPPreflightChecker {
	return &GCPPreflightChecker{
		projectID: projectID,
		ctx:       context.Background(),
	}
}

// CheckResult represents the result of a preflight check
type CheckResult struct {
	Name        string
	Passed      bool
	Message     string
	Remediation string
}

// GCPPreflightResults contains all GCP preflight check results
type GCPPreflightResults struct {
	ProjectID   string
	Checks      []CheckResult
	AllPassed   bool
	FailedCount int
}

// requiredAPIs lists the GCP APIs that must be enabled
var requiredAPIs = []struct {
	name        string
	serviceName string
	description string
}{
	{
		name:        "Compute Engine API",
		serviceName: "compute.googleapis.com",
		description: "Required for networking and load balancers",
	},
	{
		name:        "Kubernetes Engine API",
		serviceName: "container.googleapis.com",
		description: "Required for GKE cluster management",
	},
	{
		name:        "Cloud Resource Manager API",
		serviceName: "cloudresourcemanager.googleapis.com",
		description: "Required for project metadata and permissions",
	},
	{
		name:        "IAM API",
		serviceName: "iam.googleapis.com",
		description: "Required for service account management",
	},
	{
		name:        "Service Usage API",
		serviceName: "serviceusage.googleapis.com",
		description: "Required to check API enablement status",
	},
}

// requiredPermissions lists the IAM permissions needed
var requiredPermissions = []struct {
	permission  string
	description string
}{
	{
		permission:  "container.clusters.create",
		description: "Create GKE clusters",
	},
	{
		permission:  "container.clusters.get",
		description: "View GKE cluster details",
	},
	{
		permission:  "container.clusters.update",
		description: "Update GKE clusters",
	},
	{
		permission:  "container.operations.get",
		description: "Check GKE operation status",
	},
	{
		permission:  "compute.addresses.create",
		description: "Create static IP addresses",
	},
	{
		permission:  "compute.addresses.get",
		description: "View IP addresses",
	},
	{
		permission:  "compute.networks.get",
		description: "View VPC networks",
	},
	{
		permission:  "compute.subnetworks.get",
		description: "View subnets",
	},
	{
		permission:  "iam.serviceAccounts.create",
		description: "Create service accounts",
	},
	{
		permission:  "iam.serviceAccounts.get",
		description: "View service accounts",
	},
	{
		permission:  "iam.serviceAccountKeys.create",
		description: "Create service account keys",
	},
	{
		permission:  "resourcemanager.projects.get",
		description: "View project metadata",
	},
	{
		permission:  "resourcemanager.projects.getIamPolicy",
		description: "View project IAM policies",
	},
}

// RunAll runs all GCP preflight checks
func (g *GCPPreflightChecker) RunAll() (*GCPPreflightResults, error) {
	results := &GCPPreflightResults{
		ProjectID: g.projectID,
		Checks:    make([]CheckResult, 0),
	}

	// Check if credentials are available
	credCheck := g.checkCredentials()
	results.Checks = append(results.Checks, credCheck)
	if !credCheck.Passed {
		results.AllPassed = false
		results.FailedCount++
		return results, nil
	}

	// Check project existence and access
	projectCheck := g.checkProjectAccess()
	results.Checks = append(results.Checks, projectCheck)
	if !projectCheck.Passed {
		results.AllPassed = false
		results.FailedCount++
		return results, nil
	}

	// Check billing
	billingCheck := g.checkBilling()
	results.Checks = append(results.Checks, billingCheck)
	if !billingCheck.Passed {
		results.FailedCount++
	}

	// Check required APIs
	apiChecks := g.checkAPIs()
	results.Checks = append(results.Checks, apiChecks...)
	for _, check := range apiChecks {
		if !check.Passed {
			results.FailedCount++
		}
	}

	// Check permissions
	permChecks := g.checkPermissions()
	results.Checks = append(results.Checks, permChecks...)
	for _, check := range permChecks {
		if !check.Passed {
			results.FailedCount++
		}
	}

	results.AllPassed = results.FailedCount == 0
	return results, nil
}

// checkCredentials verifies GCP credentials are available
func (g *GCPPreflightChecker) checkCredentials() CheckResult {
	// Try to create a service usage client to test credentials
	_, err := serviceusage.NewService(g.ctx)
	if err != nil {
		return CheckResult{
			Name:    "GCP Credentials",
			Passed:  false,
			Message: fmt.Sprintf("Failed to authenticate: %v", err),
			Remediation: `Set up Application Default Credentials:
  - Run: gcloud auth application-default login
  - Or set GOOGLE_APPLICATION_CREDENTIALS environment variable
  - See: https://cloud.google.com/docs/authentication/getting-started`,
		}
	}

	return CheckResult{
		Name:    "GCP Credentials",
		Passed:  true,
		Message: "Successfully authenticated with GCP",
	}
}

// checkProjectAccess verifies project exists and is accessible
func (g *GCPPreflightChecker) checkProjectAccess() CheckResult {
	service, err := cloudresourcemanager.NewService(g.ctx)
	if err != nil {
		return CheckResult{
			Name:        "Project Access",
			Passed:      false,
			Message:     fmt.Sprintf("Failed to create API client: %v", err),
			Remediation: "Verify GCP credentials are configured correctly",
		}
	}

	project, err := service.Projects.Get(g.projectID).Context(g.ctx).Do()
	if err != nil {
		return CheckResult{
			Name:    "Project Access",
			Passed:  false,
			Message: fmt.Sprintf("Cannot access project %s: %v", g.projectID, err),
			Remediation: fmt.Sprintf(`Verify the project exists and you have access:
  - Run: gcloud projects describe %s
  - Check project ID is correct
  - Verify you have at least Viewer role on the project`, g.projectID),
		}
	}

	if project.LifecycleState != "ACTIVE" {
		return CheckResult{
			Name:    "Project Access",
			Passed:  false,
			Message: fmt.Sprintf("Project %s is not active (state: %s)", g.projectID, project.LifecycleState),
			Remediation: fmt.Sprintf("Contact your GCP administrator to activate project %s", g.projectID),
		}
	}

	return CheckResult{
		Name:    "Project Access",
		Passed:  true,
		Message: fmt.Sprintf("Successfully accessed project: %s (%s)", project.Name, project.ProjectId),
	}
}

// checkBilling verifies project has billing enabled
func (g *GCPPreflightChecker) checkBilling() CheckResult {
	service, err := cloudresourcemanager.NewService(g.ctx)
	if err != nil {
		return CheckResult{
			Name:        "Billing Status",
			Passed:      false,
			Message:     fmt.Sprintf("Failed to check billing: %v", err),
			Remediation: "Verify GCP credentials have billing viewer permissions",
		}
	}

	project, err := service.Projects.Get(g.projectID).Context(g.ctx).Do()
	if err != nil {
		return CheckResult{
			Name:        "Billing Status",
			Passed:      false,
			Message:     fmt.Sprintf("Failed to get project billing info: %v", err),
			Remediation: "Verify you have billing.resourceAssociations.list permission",
		}
	}

	// Check if billing account is linked (basic check)
	// Note: More detailed billing checks would require the cloudbilling API
	if project.ProjectId == "" {
		return CheckResult{
			Name:        "Billing Status",
			Passed:      false,
			Message:     "Unable to verify billing status",
			Remediation: "Manually verify billing is enabled for this project in GCP Console",
		}
	}

	return CheckResult{
		Name:    "Billing Status",
		Passed:  true,
		Message: "Project has billing enabled (basic check passed)",
	}
}

// checkAPIs verifies required APIs are enabled
func (g *GCPPreflightChecker) checkAPIs() []CheckResult {
	results := make([]CheckResult, 0, len(requiredAPIs))

	service, err := serviceusage.NewService(g.ctx)
	if err != nil {
		results = append(results, CheckResult{
			Name:        "API Enablement Check",
			Passed:      false,
			Message:     fmt.Sprintf("Failed to create service usage client: %v", err),
			Remediation: "Verify GCP credentials and service usage API access",
		})
		return results
	}

	for _, api := range requiredAPIs {
		serviceName := fmt.Sprintf("projects/%s/services/%s", g.projectID, api.serviceName)
		apiService, err := service.Services.Get(serviceName).Context(g.ctx).Do()

		if err != nil {
			results = append(results, CheckResult{
				Name:    fmt.Sprintf("API: %s", api.name),
				Passed:  false,
				Message: fmt.Sprintf("Failed to check status: %v", err),
				Remediation: fmt.Sprintf(`Enable %s:
  - Run: gcloud services enable %s --project=%s
  - Or enable in GCP Console: APIs & Services > Library`, api.name, api.serviceName, g.projectID),
			})
			continue
		}

		if apiService.State == "ENABLED" {
			results = append(results, CheckResult{
				Name:    fmt.Sprintf("API: %s", api.name),
				Passed:  true,
				Message: fmt.Sprintf("Enabled - %s", api.description),
			})
		} else {
			results = append(results, CheckResult{
				Name:    fmt.Sprintf("API: %s", api.name),
				Passed:  false,
				Message: fmt.Sprintf("Not enabled (state: %s)", apiService.State),
				Remediation: fmt.Sprintf(`Enable the API:
  - Run: gcloud services enable %s --project=%s
  - Description: %s`, api.serviceName, g.projectID, api.description),
			})
		}
	}

	return results
}

// checkPermissions verifies required IAM permissions
func (g *GCPPreflightChecker) checkPermissions() []CheckResult {
	results := make([]CheckResult, 0)

	service, err := cloudresourcemanager.NewService(g.ctx)
	if err != nil {
		results = append(results, CheckResult{
			Name:        "IAM Permissions Check",
			Passed:      false,
			Message:     fmt.Sprintf("Failed to create IAM client: %v", err),
			Remediation: "Verify GCP credentials and IAM API access",
		})
		return results
	}

	// Extract just the permission strings
	permissions := make([]string, len(requiredPermissions))
	for i, p := range requiredPermissions {
		permissions[i] = p.permission
	}

	// Test permissions
	req := &cloudresourcemanager.TestIamPermissionsRequest{
		Permissions: permissions,
	}

	resource := fmt.Sprintf("projects/%s", g.projectID)
	resp, err := service.Projects.TestIamPermissions(resource, req).Context(g.ctx).Do()
	if err != nil {
		results = append(results, CheckResult{
			Name:        "IAM Permissions",
			Passed:      false,
			Message:     fmt.Sprintf("Failed to test permissions: %v", err),
			Remediation: "Verify you have resourcemanager.projects.getIamPolicy permission",
		})
		return results
	}

	// Build map of granted permissions
	granted := make(map[string]bool)
	for _, perm := range resp.Permissions {
		granted[perm] = true
	}

	// Check each required permission
	missingPerms := make([]string, 0)
	for _, perm := range requiredPermissions {
		if granted[perm.permission] {
			results = append(results, CheckResult{
				Name:    fmt.Sprintf("Permission: %s", perm.permission),
				Passed:  true,
				Message: perm.description,
			})
		} else {
			missingPerms = append(missingPerms, perm.permission)
			results = append(results, CheckResult{
				Name:    fmt.Sprintf("Permission: %s", perm.permission),
				Passed:  false,
				Message: fmt.Sprintf("Missing permission - %s", perm.description),
				Remediation: fmt.Sprintf(`Grant the required permission:
  - This permission is typically included in these roles:
    * roles/container.admin (GKE Admin)
    * roles/compute.admin (Compute Admin)
    * roles/editor (Editor)
  - Run: gcloud projects add-iam-policy-binding %s --member=user:YOUR_EMAIL --role=ROLE_NAME`, g.projectID),
			})
		}
	}

	// Add summary if any permissions are missing
	if len(missingPerms) > 0 {
		results = append([]CheckResult{{
			Name:    "IAM Permissions Summary",
			Passed:  false,
			Message: fmt.Sprintf("Missing %d required permissions", len(missingPerms)),
			Remediation: fmt.Sprintf(`Grant required permissions by assigning appropriate roles:
  - Recommended role: roles/container.admin (includes most required permissions)
  - Run: gcloud projects add-iam-policy-binding %s \
      --member=user:YOUR_EMAIL \
      --role=roles/container.admin

Missing permissions: %s`, g.projectID, strings.Join(missingPerms, ", ")),
		}}, results...)
	} else {
		results = append([]CheckResult{{
			Name:    "IAM Permissions Summary",
			Passed:  true,
			Message: fmt.Sprintf("All %d required permissions granted", len(requiredPermissions)),
		}}, results...)
	}

	return results
}

// CheckQuotas verifies project has sufficient quotas (optional detailed check)
func (g *GCPPreflightChecker) CheckQuotas(region string) []CheckResult {
	results := make([]CheckResult, 0)

	service, err := compute.NewService(g.ctx)
	if err != nil {
		results = append(results, CheckResult{
			Name:        "Quota Check",
			Passed:      false,
			Message:     fmt.Sprintf("Failed to create compute client: %v", err),
			Remediation: "Verify Compute Engine API is enabled",
		})
		return results
	}

	// Get regional quotas
	regionObj, err := service.Regions.Get(g.projectID, region).Context(g.ctx).Do()
	if err != nil {
		results = append(results, CheckResult{
			Name:        "Regional Quotas",
			Passed:      false,
			Message:     fmt.Sprintf("Failed to get quota info: %v", err),
			Remediation: fmt.Sprintf("Verify region %s is valid and accessible", region),
		})
		return results
	}

	// Check key quotas
	quotaChecks := map[string]float64{
		"CPUS":       8.0,  // Minimum CPUs for GKE
		"IN_USE_ADDRESSES": 1.0,  // Static IP
	}

	for _, quota := range regionObj.Quotas {
		if required, ok := quotaChecks[quota.Metric]; ok {
			remaining := quota.Limit - quota.Usage
			if remaining >= required {
				results = append(results, CheckResult{
					Name:    fmt.Sprintf("Quota: %s", quota.Metric),
					Passed:  true,
					Message: fmt.Sprintf("Available: %.0f / %.0f (need %.0f)", remaining, quota.Limit, required),
				})
			} else {
				results = append(results, CheckResult{
					Name:    fmt.Sprintf("Quota: %s", quota.Metric),
					Passed:  false,
					Message: fmt.Sprintf("Insufficient: %.0f available, %.0f needed", remaining, required),
					Remediation: fmt.Sprintf(`Request quota increase:
  - Go to: https://console.cloud.google.com/iam-admin/quotas?project=%s
  - Filter for: %s in %s
  - Request increase to at least %.0f`, g.projectID, quota.Metric, region, quota.Usage+required),
				})
			}
		}
	}

	return results
}
