package domain

// CoalesceStr returns the first non-empty string from vals.
func CoalesceStr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// IntFromPtrWithDefault returns the first non-nil *int value, or the fallback.
func IntFromPtrWithDefault(fallback int, ptrs ...*int) int {
	for _, p := range ptrs {
		if p != nil {
			return *p
		}
	}
	return fallback
}

// BoolFromPtrWithDefault returns the first non-nil *bool value, or the fallback.
func BoolFromPtrWithDefault(fallback bool, ptrs ...*bool) bool {
	for _, p := range ptrs {
		if p != nil {
			return *p
		}
	}
	return fallback
}

// Float64FromPtrWithDefault returns the first non-nil *float64 value, or the fallback.
func Float64FromPtrWithDefault(fallback float64, ptrs ...*float64) float64 {
	for _, p := range ptrs {
		if p != nil {
			return *p
		}
	}
	return fallback
}
