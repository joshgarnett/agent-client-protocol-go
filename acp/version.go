package acp

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/joshgarnett/agent-client-protocol-go/acp/api"
)

const minPartsForPatch = 2

// ProtocolVersion represents a protocol version.
type ProtocolVersion struct {
	Major int
	Minor int
	Patch int
}

// NewProtocolVersion creates a new protocol version.
func NewProtocolVersion(major, minor, patch int) *ProtocolVersion {
	return &ProtocolVersion{
		Major: major,
		Minor: minor,
		Patch: patch,
	}
}

// ParseVersion parses a version from various formats.
// Supports:
// - Numeric (float64): treated as major version only
// - String: "1.2.3" format or legacy string versions (treated as v0)
// - Integer: treated as major version only.
func ParseVersion(v interface{}) (*ProtocolVersion, error) {
	switch version := v.(type) {
	case float64:
		return &ProtocolVersion{Major: int(version)}, nil
	case int:
		return &ProtocolVersion{Major: version}, nil
	case int64:
		return &ProtocolVersion{Major: int(version)}, nil
	case string:
		// Try to parse as semantic version
		parts := strings.Split(version, ".")
		if len(parts) == 1 {
			// Try to parse as number
			if major, err := strconv.Atoi(parts[0]); err == nil {
				return &ProtocolVersion{Major: major}, nil
			}
			// Legacy string versions are treated as v0
			return &ProtocolVersion{Major: 0}, nil
		}

		// Parse semantic version
		return parseSemanticVersion(parts)
	default:
		return nil, fmt.Errorf("invalid version format: %T", v)
	}
}

// parseSemanticVersion parses version parts into a ProtocolVersion.
func parseSemanticVersion(parts []string) (*ProtocolVersion, error) {
	pv := &ProtocolVersion{}

	if len(parts) == 0 {
		return pv, nil
	}

	// Parse major version - if invalid, treat as v0
	if major, err := strconv.Atoi(parts[0]); err == nil {
		pv.Major = major
	} else {
		return &ProtocolVersion{Major: 0}, nil
	}

	if len(parts) <= 1 {
		return pv, nil
	}

	// Parse minor version - if invalid, stop parsing and keep only major
	if minor, err := strconv.Atoi(parts[1]); err == nil {
		pv.Minor = minor
	} else {
		return pv, nil
	}

	if len(parts) <= minPartsForPatch {
		return pv, nil
	}

	// Parse patch version - if invalid, keep major and minor but patch stays 0
	if patch, err := strconv.Atoi(parts[2]); err == nil {
		pv.Patch = patch
	}

	return pv, nil
}

// String returns the string representation of the version.
func (pv *ProtocolVersion) String() string {
	if pv.Minor == 0 && pv.Patch == 0 {
		return strconv.Itoa(pv.Major)
	}
	if pv.Patch == 0 {
		return fmt.Sprintf("%d.%d", pv.Major, pv.Minor)
	}
	return fmt.Sprintf("%d.%d.%d", pv.Major, pv.Minor, pv.Patch)
}

// IsCompatible checks if this version is compatible with another version.
// Uses semantic versioning rules:
// - Major version must match for compatibility
// - Minor version changes add functionality in a backward-compatible manner
// - Patch version changes are backward-compatible bug fixes.
func (pv *ProtocolVersion) IsCompatible(other *ProtocolVersion) bool {
	if pv == nil || other == nil {
		return false
	}

	// Major version must match
	if pv.Major != other.Major {
		return false
	}

	// If we're newer or equal in minor version, we're compatible
	if pv.Minor >= other.Minor {
		return true
	}

	// If minor versions match, check patch
	if pv.Minor == other.Minor {
		return pv.Patch >= other.Patch
	}

	return false
}

// Compare compares two versions.
// Returns:
//
//	-1 if pv < other
//	 0 if pv == other
//	 1 if pv > other
func (pv *ProtocolVersion) Compare(other *ProtocolVersion) int {
	if pv.Major < other.Major {
		return -1
	}
	if pv.Major > other.Major {
		return 1
	}

	if pv.Minor < other.Minor {
		return -1
	}
	if pv.Minor > other.Minor {
		return 1
	}

	if pv.Patch < other.Patch {
		return -1
	}
	if pv.Patch > other.Patch {
		return 1
	}

	return 0
}

// IsNewer returns true if this version is newer than the other.
func (pv *ProtocolVersion) IsNewer(other *ProtocolVersion) bool {
	return pv.Compare(other) > 0
}

// IsOlder returns true if this version is older than the other.
func (pv *ProtocolVersion) IsOlder(other *ProtocolVersion) bool {
	return pv.Compare(other) < 0
}

// Equals returns true if the versions are equal.
func (pv *ProtocolVersion) Equals(other *ProtocolVersion) bool {
	return pv.Compare(other) == 0
}

// VersionNegotiator handles version negotiation between client and agent.
type VersionNegotiator struct {
	supported []ProtocolVersion
	preferred ProtocolVersion
	current   *ProtocolVersion
}

// NewVersionNegotiator creates a new version negotiator.
func NewVersionNegotiator() *VersionNegotiator {
	// Default to the current protocol version
	currentVersion := NewProtocolVersion(api.ACPProtocolVersion, 0, 0)

	return &VersionNegotiator{
		supported: []ProtocolVersion{
			*currentVersion,
			{Major: 0, Minor: 0, Patch: 0}, // Support legacy v0
		},
		preferred: *currentVersion,
	}
}

// AddSupported adds a supported version.
func (vn *VersionNegotiator) AddSupported(version ProtocolVersion) {
	// Check if already supported
	for _, v := range vn.supported {
		if v.Equals(&version) {
			return
		}
	}
	vn.supported = append(vn.supported, version)
}

// SetPreferred sets the preferred version.
func (vn *VersionNegotiator) SetPreferred(version ProtocolVersion) {
	vn.preferred = version
}

// Negotiate negotiates a version with the client/agent.
func (vn *VersionNegotiator) Negotiate(clientVersion interface{}) (*ProtocolVersion, error) {
	// Parse the client version
	parsed, err := ParseVersion(clientVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to parse client version: %w", err)
	}

	// Check if we support this exact version
	for _, supported := range vn.supported {
		if supported.Equals(parsed) {
			vn.current = parsed
			return parsed, nil
		}
	}

	// Check for compatible versions
	var bestMatch *ProtocolVersion
	for i := range vn.supported {
		if vn.supported[i].IsCompatible(parsed) {
			if bestMatch == nil || vn.supported[i].IsNewer(bestMatch) {
				bestMatch = &vn.supported[i]
			}
		}
	}

	if bestMatch != nil {
		vn.current = bestMatch
		return bestMatch, nil
	}

	// No compatible version found
	return nil, fmt.Errorf("no compatible version found for client version %s", parsed.String())
}

// GetCurrent returns the currently negotiated version.
func (vn *VersionNegotiator) GetCurrent() *ProtocolVersion {
	return vn.current
}

// GetSupported returns all supported versions.
func (vn *VersionNegotiator) GetSupported() []ProtocolVersion {
	return vn.supported
}

// GetPreferred returns the preferred version.
func (vn *VersionNegotiator) GetPreferred() ProtocolVersion {
	return vn.preferred
}

// Feature detection based on version

// Feature represents a protocol feature.
type Feature string

// Protocol features.
const (
	FeatureLoadSession    Feature = "loadSession"
	FeatureTerminal       Feature = "terminal"
	FeaturePlans          Feature = "plans"
	FeatureRichContent    Feature = "richContent"
	FeatureToolCalls      Feature = "toolCalls"
	FeatureApproval       Feature = "approval"
	FeatureAuthentication Feature = "authentication"
	FeatureStreaming      Feature = "streaming"
	FeatureProgress       Feature = "progress"
)

// FeatureSet represents the features available for a protocol version.
type FeatureSet struct {
	version  *ProtocolVersion
	features map[Feature]bool
}

// GetFeatureSet returns the feature set for a given version.
func GetFeatureSet(version *ProtocolVersion) *FeatureSet {
	fs := &FeatureSet{
		version:  version,
		features: make(map[Feature]bool),
	}

	// Version 0 features (legacy)
	if version.Major == 0 {
		fs.features[FeatureAuthentication] = true
		// Basic features only
		return fs
	}

	// Version 1 features (current)
	if version.Major >= 1 {
		fs.features[FeatureLoadSession] = true
		fs.features[FeatureTerminal] = true
		fs.features[FeaturePlans] = true
		fs.features[FeatureRichContent] = true
		fs.features[FeatureToolCalls] = true
		fs.features[FeatureApproval] = true
		fs.features[FeatureAuthentication] = true
		fs.features[FeatureStreaming] = true
		fs.features[FeatureProgress] = true
	}

	// Future version features can be added here

	return fs
}

// HasFeature checks if a feature is available.
func (fs *FeatureSet) HasFeature(feature Feature) bool {
	return fs.features[feature]
}

// GetFeatures returns all available features.
func (fs *FeatureSet) GetFeatures() []Feature {
	var features []Feature
	for feature, enabled := range fs.features {
		if enabled {
			features = append(features, feature)
		}
	}
	return features
}

// GetVersion returns the version associated with this feature set.
func (fs *FeatureSet) GetVersion() *ProtocolVersion {
	return fs.version
}

// Integration with connections

// NegotiateVersion negotiates the protocol version during initialization.
func (a *AgentConnection) NegotiateVersion(req *api.InitializeRequest) (*ProtocolVersion, error) {
	negotiator := NewVersionNegotiator()

	// The initialize request should contain the client's protocol version
	// For now, we'll use the constant version
	var clientVersion interface{} = req.ProtocolVersion
	if req.ProtocolVersion == 0 {
		// Default to version 1 if not specified
		clientVersion = api.ACPProtocolVersion
	}

	return negotiator.Negotiate(clientVersion)
}

// GetProtocolVersion returns the current protocol version for the connection.
func (a *AgentConnection) GetProtocolVersion() *ProtocolVersion {
	// For now, return the default version
	// In a real implementation, this would be stored after negotiation
	return NewProtocolVersion(api.ACPProtocolVersion, 0, 0)
}

// GetFeatureSet returns the feature set for the current connection.
func (a *AgentConnection) GetFeatureSet() *FeatureSet {
	return GetFeatureSet(a.GetProtocolVersion())
}

// Similar methods for ClientConnection

// GetProtocolVersion returns the current protocol version for the connection.
func (c *ClientConnection) GetProtocolVersion() *ProtocolVersion {
	// For now, return the default version
	// In a real implementation, this would be stored after negotiation
	return NewProtocolVersion(api.ACPProtocolVersion, 0, 0)
}

// GetFeatureSet returns the feature set for the current connection.
func (c *ClientConnection) GetFeatureSet() *FeatureSet {
	return GetFeatureSet(c.GetProtocolVersion())
}
