package acp

import (
	"testing"

	"github.com/joshgarnett/agent-client-protocol-go/acp/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProtocolVersion(t *testing.T) {
	t.Run("NewProtocolVersion", func(t *testing.T) {
		version := NewProtocolVersion(1, 2, 3)
		assert.Equal(t, 1, version.Major)
		assert.Equal(t, 2, version.Minor)
		assert.Equal(t, 3, version.Patch)
	})

	t.Run("String Representation", func(t *testing.T) {
		tests := []struct {
			name     string
			version  *ProtocolVersion
			expected string
		}{
			{"Major only", &ProtocolVersion{Major: 1}, "1"},
			{"Major.Minor", &ProtocolVersion{Major: 1, Minor: 2}, "1.2"},
			{"Full version", &ProtocolVersion{Major: 1, Minor: 2, Patch: 3}, "1.2.3"},
			{"Zero version", &ProtocolVersion{}, "0"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				assert.Equal(t, tt.expected, tt.version.String())
			})
		}
	})

	t.Run("Compare Versions", func(t *testing.T) {
		v1 := &ProtocolVersion{Major: 1, Minor: 0, Patch: 0}
		v2 := &ProtocolVersion{Major: 1, Minor: 0, Patch: 0}
		v3 := &ProtocolVersion{Major: 2, Minor: 0, Patch: 0}
		v4 := &ProtocolVersion{Major: 1, Minor: 1, Patch: 0}
		v5 := &ProtocolVersion{Major: 1, Minor: 0, Patch: 1}

		// Equal versions
		assert.Equal(t, 0, v1.Compare(v2))
		assert.True(t, v1.Equals(v2))

		// Major version difference
		assert.Equal(t, -1, v1.Compare(v3))
		assert.Equal(t, 1, v3.Compare(v1))
		assert.True(t, v1.IsOlder(v3))
		assert.True(t, v3.IsNewer(v1))

		// Minor version difference
		assert.Equal(t, -1, v1.Compare(v4))
		assert.Equal(t, 1, v4.Compare(v1))

		// Patch version difference
		assert.Equal(t, -1, v1.Compare(v5))
		assert.Equal(t, 1, v5.Compare(v1))
	})

	t.Run("IsCompatible", func(t *testing.T) {
		v1_0_0 := &ProtocolVersion{Major: 1, Minor: 0, Patch: 0}
		v1_1_0 := &ProtocolVersion{Major: 1, Minor: 1, Patch: 0}
		v1_0_1 := &ProtocolVersion{Major: 1, Minor: 0, Patch: 1}
		v2_0_0 := &ProtocolVersion{Major: 2, Minor: 0, Patch: 0}

		// Same major version, compatible
		assert.True(t, v1_1_0.IsCompatible(v1_0_0))
		assert.True(t, v1_0_1.IsCompatible(v1_0_0))

		// Older minor version, not compatible
		assert.False(t, v1_0_0.IsCompatible(v1_1_0))

		// Different major version, not compatible
		assert.False(t, v1_0_0.IsCompatible(v2_0_0))
		assert.False(t, v2_0_0.IsCompatible(v1_0_0))

		// Nil versions
		assert.False(t, v1_0_0.IsCompatible(nil))
		var nilVersion *ProtocolVersion
		assert.False(t, nilVersion.IsCompatible(v1_0_0))
	})
}

func TestParseVersion(t *testing.T) {
	t.Run("Parse Float64", func(t *testing.T) {
		version, err := ParseVersion(float64(2))
		require.NoError(t, err)
		assert.Equal(t, 2, version.Major)
		assert.Equal(t, 0, version.Minor)
		assert.Equal(t, 0, version.Patch)
	})

	t.Run("Parse Int", func(t *testing.T) {
		version, err := ParseVersion(3)
		require.NoError(t, err)
		assert.Equal(t, 3, version.Major)
		assert.Equal(t, 0, version.Minor)
		assert.Equal(t, 0, version.Patch)
	})

	t.Run("Parse Int64", func(t *testing.T) {
		version, err := ParseVersion(int64(4))
		require.NoError(t, err)
		assert.Equal(t, 4, version.Major)
		assert.Equal(t, 0, version.Minor)
		assert.Equal(t, 0, version.Patch)
	})

	t.Run("Parse String Semantic Version", func(t *testing.T) {
		tests := []struct {
			input         string
			expectedMajor int
			expectedMinor int
			expectedPatch int
		}{
			{"1", 1, 0, 0},
			{"2.3", 2, 3, 0},
			{"4.5.6", 4, 5, 6},
			{"1.0.0", 1, 0, 0},
		}

		for _, tt := range tests {
			t.Run(tt.input, func(t *testing.T) {
				version, err := ParseVersion(tt.input)
				require.NoError(t, err)
				assert.Equal(t, tt.expectedMajor, version.Major)
				assert.Equal(t, tt.expectedMinor, version.Minor)
				assert.Equal(t, tt.expectedPatch, version.Patch)
			})
		}
	})

	t.Run("Parse Legacy String Version", func(t *testing.T) {
		// Non-numeric strings are treated as v0
		version, err := ParseVersion("legacy")
		require.NoError(t, err)
		assert.Equal(t, 0, version.Major)
		assert.Equal(t, 0, version.Minor)
		assert.Equal(t, 0, version.Patch)

		// Invalid semantic version parts
		version, err = ParseVersion("1.invalid.3")
		require.NoError(t, err)
		assert.Equal(t, 1, version.Major)
		assert.Equal(t, 0, version.Minor)
		assert.Equal(t, 0, version.Patch)
	})

	t.Run("Parse Invalid Type", func(t *testing.T) {
		_, err := ParseVersion([]byte{1, 2, 3})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid version format")
	})
}

func TestVersionNegotiator(t *testing.T) {
	t.Run("Default Negotiator", func(t *testing.T) {
		negotiator := NewVersionNegotiator()

		// Should have current version and v0 supported
		assert.Len(t, negotiator.GetSupported(), 2)
		assert.Equal(t, api.ACPProtocolVersion, negotiator.GetPreferred().Major)
	})

	t.Run("Add Supported Version", func(t *testing.T) {
		negotiator := NewVersionNegotiator()
		initialCount := len(negotiator.GetSupported())

		newVersion := ProtocolVersion{Major: 2, Minor: 0, Patch: 0}
		negotiator.AddSupported(newVersion)
		assert.Len(t, negotiator.GetSupported(), initialCount+1)

		// Adding same version again should not duplicate
		negotiator.AddSupported(newVersion)
		assert.Len(t, negotiator.GetSupported(), initialCount+1)
	})

	t.Run("Set Preferred Version", func(t *testing.T) {
		negotiator := NewVersionNegotiator()
		newPreferred := ProtocolVersion{Major: 2, Minor: 1, Patch: 0}
		negotiator.SetPreferred(newPreferred)
		assert.Equal(t, newPreferred, negotiator.GetPreferred())
	})

	t.Run("Negotiate Exact Match", func(t *testing.T) {
		negotiator := NewVersionNegotiator()
		negotiator.AddSupported(ProtocolVersion{Major: 2, Minor: 0, Patch: 0})

		// Negotiate with exact match
		result, err := negotiator.Negotiate(2)
		require.NoError(t, err)
		assert.Equal(t, 2, result.Major)
		assert.Equal(t, result, negotiator.GetCurrent())
	})

	t.Run("Negotiate Compatible Version", func(t *testing.T) {
		negotiator := NewVersionNegotiator()
		negotiator.AddSupported(ProtocolVersion{Major: 1, Minor: 2, Patch: 0})
		negotiator.AddSupported(ProtocolVersion{Major: 1, Minor: 3, Patch: 0})

		// Client requests v1.1, we support v1.2 and v1.3
		result, err := negotiator.Negotiate("1.1.0")
		require.NoError(t, err)
		// Should pick the best compatible version (1.3)
		assert.Equal(t, 1, result.Major)
		assert.Equal(t, 3, result.Minor)
	})

	t.Run("Negotiate No Compatible Version", func(t *testing.T) {
		negotiator := NewVersionNegotiator()
		// Only support v1
		negotiator.supported = []ProtocolVersion{{Major: 1}}

		// Client requests v2
		_, err := negotiator.Negotiate(2)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no compatible version")
	})

	t.Run("Negotiate Invalid Version", func(t *testing.T) {
		negotiator := NewVersionNegotiator()

		_, err := negotiator.Negotiate(struct{}{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse")
	})
}

func TestFeatureSet(t *testing.T) {
	t.Run("Version 0 Features", func(t *testing.T) {
		v0 := &ProtocolVersion{Major: 0}
		features := GetFeatureSet(v0)

		assert.True(t, features.HasFeature(FeatureAuthentication))
		assert.False(t, features.HasFeature(FeatureLoadSession))
		assert.False(t, features.HasFeature(FeatureTerminal))
		assert.False(t, features.HasFeature(FeaturePlans))
	})

	t.Run("Version 1 Features", func(t *testing.T) {
		v1 := &ProtocolVersion{Major: 1}
		features := GetFeatureSet(v1)

		// Should have all features
		assert.True(t, features.HasFeature(FeatureAuthentication))
		assert.True(t, features.HasFeature(FeatureLoadSession))
		assert.True(t, features.HasFeature(FeatureTerminal))
		assert.True(t, features.HasFeature(FeaturePlans))
		assert.True(t, features.HasFeature(FeatureRichContent))
		assert.True(t, features.HasFeature(FeatureToolCalls))
		assert.True(t, features.HasFeature(FeatureApproval))
		assert.True(t, features.HasFeature(FeatureStreaming))
		assert.True(t, features.HasFeature(FeatureProgress))
	})

	t.Run("Get Features List", func(t *testing.T) {
		v1 := &ProtocolVersion{Major: 1}
		features := GetFeatureSet(v1)

		featureList := features.GetFeatures()
		assert.NotEmpty(t, featureList)

		// Check that returned features are all enabled
		for _, feature := range featureList {
			assert.True(t, features.HasFeature(feature))
		}
	})

	t.Run("Get Version", func(t *testing.T) {
		v1 := &ProtocolVersion{Major: 1, Minor: 2, Patch: 3}
		features := GetFeatureSet(v1)

		assert.Equal(t, v1, features.GetVersion())
	})

	t.Run("Unknown Feature", func(t *testing.T) {
		v1 := &ProtocolVersion{Major: 1}
		features := GetFeatureSet(v1)

		// Check for a feature that doesn't exist
		assert.False(t, features.HasFeature(Feature("unknown_feature")))
	})
}

func TestConnectionVersionIntegration(t *testing.T) {
	t.Run("AgentConnection GetProtocolVersion", func(t *testing.T) {
		// This would need a mock AgentConnection
		// For now, test the helper function directly
		version := NewProtocolVersion(api.ACPProtocolVersion, 0, 0)
		assert.Equal(t, api.ACPProtocolVersion, version.Major)
		assert.Equal(t, 0, version.Minor)
		assert.Equal(t, 0, version.Patch)
	})

	t.Run("Feature Detection", func(t *testing.T) {
		// Test feature detection for current version
		currentVersion := NewProtocolVersion(api.ACPProtocolVersion, 0, 0)
		features := GetFeatureSet(currentVersion)

		// Current version (1) should have all features
		if api.ACPProtocolVersion == 1 {
			assert.True(t, features.HasFeature(FeatureLoadSession))
			assert.True(t, features.HasFeature(FeatureTerminal))
			assert.True(t, features.HasFeature(FeaturePlans))
		}
	})
}

func TestVersionCompatibilityMatrix(t *testing.T) {
	// Test a compatibility matrix to ensure correct behavior
	tests := []struct {
		name       string
		version1   *ProtocolVersion
		version2   *ProtocolVersion
		compatible bool
	}{
		{
			name:       "Same version",
			version1:   &ProtocolVersion{Major: 1, Minor: 0, Patch: 0},
			version2:   &ProtocolVersion{Major: 1, Minor: 0, Patch: 0},
			compatible: true,
		},
		{
			name:       "Newer minor version",
			version1:   &ProtocolVersion{Major: 1, Minor: 2, Patch: 0},
			version2:   &ProtocolVersion{Major: 1, Minor: 1, Patch: 0},
			compatible: true,
		},
		{
			name:       "Older minor version",
			version1:   &ProtocolVersion{Major: 1, Minor: 1, Patch: 0},
			version2:   &ProtocolVersion{Major: 1, Minor: 2, Patch: 0},
			compatible: false,
		},
		{
			name:       "Different major version",
			version1:   &ProtocolVersion{Major: 1, Minor: 0, Patch: 0},
			version2:   &ProtocolVersion{Major: 2, Minor: 0, Patch: 0},
			compatible: false,
		},
		{
			name:       "Newer patch version",
			version1:   &ProtocolVersion{Major: 1, Minor: 0, Patch: 2},
			version2:   &ProtocolVersion{Major: 1, Minor: 0, Patch: 1},
			compatible: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.version1.IsCompatible(tt.version2)
			assert.Equal(t, tt.compatible, result)
		})
	}
}
