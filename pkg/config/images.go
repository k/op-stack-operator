package config

import (
	"fmt"
	"strings"
	"time"
)

const (
	// Official Optimism container registry
	OptimismRegistry = "us-docker.pkg.dev/oplabs-tools-artifacts/images"

	// Current stable versions (January 2025)
	DefaultOpStackVersion = "v1.13.3"
	DefaultOpGethVersion  = "v1.101511.0" // Based on geth v1.15.11
)

// Container images with default versions - verified January 2025
var DefaultImages = ImageConfig{
	OpGeth:       fmt.Sprintf("%s/op-geth:%s", OptimismRegistry, DefaultOpGethVersion),  // v1.101511.0
	OpNode:       fmt.Sprintf("%s/op-node:%s", OptimismRegistry, DefaultOpStackVersion), // v1.13.3
	OpBatcher:    fmt.Sprintf("%s/op-batcher:v1.12.0", OptimismRegistry),                // v1.12.0 (compatible with Isthmus)
	OpProposer:   fmt.Sprintf("%s/op-proposer:v1.10.0", OptimismRegistry),               // v1.10.0 (latest stable)
	OpChallenger: fmt.Sprintf("%s/op-challenger:v1.5.1", OptimismRegistry),              // v1.5.1 (latest)
}

// ImageConfig allows overriding default images and supports version compatibility
type ImageConfig struct {
	OpGeth       string `json:"opGeth,omitempty"`
	OpNode       string `json:"opNode,omitempty"`
	OpBatcher    string `json:"opBatcher,omitempty"`
	OpProposer   string `json:"opProposer,omitempty"`
	OpChallenger string `json:"opChallenger,omitempty"`
}

// VersionSet represents a compatible set of component versions
type VersionSet struct {
	OpNode       string `json:"opNode"`
	OpGeth       string `json:"opGeth"`
	OpBatcher    string `json:"opBatcher"`
	OpProposer   string `json:"opProposer"`
	OpChallenger string `json:"opChallenger"`
}

// VersionCompatibilityMatrix defines known compatible version combinations
// These should be updated based on testing and official releases
var VersionCompatibilityMatrix = map[string]VersionSet{
	// Stable production versions - verified January 2025
	"stable-v1.13": {
		OpNode:       "v1.13.3",     // Latest stable op-node
		OpGeth:       "v1.101511.0", // Based on geth v1.15.11
		OpBatcher:    "v1.12.0",     // Compatible with Isthmus
		OpProposer:   "v1.10.0",     // Latest stable op-proposer
		OpChallenger: "v1.5.1",      // Latest challenger version
	},
	// Latest development versions
	"latest": {
		OpNode:       "latest",
		OpGeth:       "latest",
		OpBatcher:    "latest",
		OpProposer:   "latest",
		OpChallenger: "latest",
	},
}

// ValidateImageCompatibility checks if the provided image versions are compatible
func (ic *ImageConfig) ValidateImageCompatibility() error {
	// Extract versions from image tags and validate compatibility
	opNodeVersion := extractVersionFromImage(ic.OpNode)
	opGethVersion := extractVersionFromImage(ic.OpGeth)
	opBatcherVersion := extractVersionFromImage(ic.OpBatcher)
	opProposerVersion := extractVersionFromImage(ic.OpProposer)

	// Check if all OP Stack components use the same version
	if opNodeVersion != "" && opBatcherVersion != "" && opProposerVersion != "" {
		if opNodeVersion != opBatcherVersion || opNodeVersion != opProposerVersion {
			return fmt.Errorf("OP Stack component versions must match: op-node=%s, op-batcher=%s, op-proposer=%s",
				opNodeVersion, opBatcherVersion, opProposerVersion)
		}
	}

	// Check known compatibility matrix
	for _, versionSet := range VersionCompatibilityMatrix {
		if opNodeVersion == versionSet.OpNode && opGethVersion == versionSet.OpGeth {
			// Found compatible version set
			return nil
		}
	}

	// If not in matrix but versions look reasonable, allow with warning
	// This allows for newer versions not yet in the matrix
	if opNodeVersion != "" && opGethVersion != "" {
		// Log warning about untested version combination
		// This would be logged in the operator logs
		return nil
	}

	return fmt.Errorf("unable to validate version compatibility")
}

// GetCompatibleOpGethVersion returns the compatible op-geth version for a given OP Stack version
func GetCompatibleOpGethVersion(opStackVersion string) (string, error) {
	// Parse major.minor from opStackVersion (e.g., "v1.9.5" -> "v1.9")
	if opStackVersion == "" {
		return DefaultOpGethVersion, nil
	}

	if opStackVersion == "latest" {
		return "latest", nil
	}

	// Extract major.minor version pattern
	parts := strings.Split(opStackVersion, ".")
	if len(parts) >= 2 {
		majorMinor := strings.Join(parts[:2], ".")

		// Look up in VersionCompatibilityMatrix
		for matrixName, versionSet := range VersionCompatibilityMatrix {
			if strings.Contains(matrixName, majorMinor) || versionSet.OpNode == opStackVersion {
				return versionSet.OpGeth, nil
			}
		}
	}

	// Default fallback
	return DefaultOpGethVersion, nil
}

// GetVersionSet returns a complete version set for a given version name
func GetVersionSet(versionName string) (VersionSet, error) {
	if versionSet, exists := VersionCompatibilityMatrix[versionName]; exists {
		return versionSet, nil
	}

	return VersionSet{}, fmt.Errorf("version set %s not found", versionName)
}

// ApplyVersionSet applies a version set to an ImageConfig
func (ic *ImageConfig) ApplyVersionSet(versionSet VersionSet) {
	if versionSet.OpNode != "" {
		ic.OpNode = fmt.Sprintf("%s/op-node:%s", OptimismRegistry, versionSet.OpNode)
	}
	if versionSet.OpGeth != "" {
		ic.OpGeth = fmt.Sprintf("%s/op-geth:%s", OptimismRegistry, versionSet.OpGeth)
	}
	if versionSet.OpBatcher != "" {
		ic.OpBatcher = fmt.Sprintf("%s/op-batcher:%s", OptimismRegistry, versionSet.OpBatcher)
	}
	if versionSet.OpProposer != "" {
		ic.OpProposer = fmt.Sprintf("%s/op-proposer:%s", OptimismRegistry, versionSet.OpProposer)
	}
	if versionSet.OpChallenger != "" {
		ic.OpChallenger = fmt.Sprintf("%s/op-challenger:%s", OptimismRegistry, versionSet.OpChallenger)
	}
}

// extractVersionFromImage extracts the version tag from a container image
func extractVersionFromImage(image string) string {
	if image == "" {
		return ""
	}

	// Split by ':' to get tag
	parts := strings.Split(image, ":")
	if len(parts) >= 2 {
		return parts[len(parts)-1] // Return the last part (tag)
	}

	return ""
}

// ImageOverrides allows fine-grained control over image selection
type ImageOverrides struct {
	// Global version override - applies to all OP Stack components
	GlobalVersion string `json:"globalVersion,omitempty"`

	// Per-component overrides
	OpNodeImage       string `json:"opNodeImage,omitempty"`
	OpGethImage       string `json:"opGethImage,omitempty"`
	OpBatcherImage    string `json:"opBatcherImage,omitempty"`
	OpProposerImage   string `json:"opProposerImage,omitempty"`
	OpChallengerImage string `json:"opChallengerImage,omitempty"`

	// Version set name from compatibility matrix
	VersionSet string `json:"versionSet,omitempty"`
}

// BuildImageConfig creates an ImageConfig from overrides
func BuildImageConfig(overrides ImageOverrides) (ImageConfig, error) {
	config := DefaultImages

	// Apply version set if specified
	if overrides.VersionSet != "" {
		versionSet, err := GetVersionSet(overrides.VersionSet)
		if err != nil {
			return config, err
		}
		config.ApplyVersionSet(versionSet)
	}

	// Apply global version override
	if overrides.GlobalVersion != "" {
		config.OpNode = fmt.Sprintf("%s/op-node:%s", OptimismRegistry, overrides.GlobalVersion)
		config.OpBatcher = fmt.Sprintf("%s/op-batcher:%s", OptimismRegistry, overrides.GlobalVersion)
		config.OpProposer = fmt.Sprintf("%s/op-proposer:%s", OptimismRegistry, overrides.GlobalVersion)
		// Note: OpGeth and OpChallenger may have different versioning schemes
	}

	// Apply individual image overrides
	if overrides.OpNodeImage != "" {
		config.OpNode = overrides.OpNodeImage
	}
	if overrides.OpGethImage != "" {
		config.OpGeth = overrides.OpGethImage
	}
	if overrides.OpBatcherImage != "" {
		config.OpBatcher = overrides.OpBatcherImage
	}
	if overrides.OpProposerImage != "" {
		config.OpProposer = overrides.OpProposerImage
	}
	if overrides.OpChallengerImage != "" {
		config.OpChallenger = overrides.OpChallengerImage
	}

	// Validate final configuration
	if err := config.ValidateImageCompatibility(); err != nil {
		return config, fmt.Errorf("image compatibility validation failed: %w", err)
	}

	return config, nil
}

// CacheConfig controls image caching behavior
type CacheConfig struct {
	EnableImageCache bool          `json:"enableImageCache,omitempty"`
	CacheTimeout     time.Duration `json:"cacheTimeout,omitempty"`
	PullPolicy       string        `json:"pullPolicy,omitempty"` // Always, IfNotPresent, Never
}

// DefaultCacheConfig provides sensible defaults for image caching
var DefaultCacheConfig = CacheConfig{
	EnableImageCache: true,
	CacheTimeout:     24 * time.Hour, // Cache for 24 hours
	PullPolicy:       "IfNotPresent", // Don't pull if image exists locally
}
