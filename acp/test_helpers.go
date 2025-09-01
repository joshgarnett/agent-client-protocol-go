package acp

// Test helper functions used across multiple test files.

// IntPtr returns a pointer to the given int value.
func IntPtr(i int) *int {
	return &i
}

// StringPtr returns a pointer to the given string value.
func StringPtr(s string) *string {
	return &s
}
