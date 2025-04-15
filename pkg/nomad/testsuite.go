// Package nomad provides types and functions for building test execution configurations
package nomad

import (
	"fmt"
	"strings"

	"github.com/brucellino/test-operator/pkg/config"
)

// TestPhase represents the phase of testing we're in
type TestPhase string

// Constants represent the different phases of testing.
const (
	Critical TestPhase = "critical"
	Inner    TestPhase = "inner"
	Outer    TestPhase = "outer"
)

// Service represents a service that can be tested
type Service string

// TestConfig represents the configuration for a test run
type TestConfig struct {
	Phase            TestPhase
	PrimaryService   Service
	AffectedServices []Service
	ServicesConfig   *config.ServicesConfig
}

// BuildRobotTags constructs Robot Framework tags based on the test configuration
func (tc *TestConfig) BuildRobotTags() string {
	var tags []string

	// Add service tag
	tags = append(tags, fmt.Sprintf("allure.label.service:%s", tc.PrimaryService))

	// Add severity tag for critical tests
	if tc.Phase == Critical {
		tags = append(tags, "allure.label.severity:critical")
	}

	// For outer tests, add all affected services
	if tc.Phase == Outer {
		for _, service := range tc.AffectedServices {
			if service != tc.PrimaryService {
				tags = append(tags, fmt.Sprintf("allure.label.service:%s", service))
			}
		}
	}

	return strings.Join(tags, " AND ")
}

// Validate checks if the test configuration is valid
func (tc *TestConfig) Validate() error {
	if tc.ServicesConfig == nil {
		return fmt.Errorf("services configuration is required")
	}

	// Convert primary service to string for comparison
	primaryStr := string(tc.PrimaryService)
	validPrimary := false
	for _, s := range tc.ServicesConfig.Lot1Services {
		if s == primaryStr {
			validPrimary = true
			break
		}
	}
	if !validPrimary {
		return fmt.Errorf("invalid primary service: %s", tc.PrimaryService)
	}

	// Validate affected services
	if tc.Phase == Outer {
		for _, s := range tc.AffectedServices {
			valid := false
			sStr := string(s)
			for _, vs := range tc.ServicesConfig.Lot1Services {
				if vs == sStr {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("invalid affected service: %s", s)
			}
		}
	}

	return nil
}
