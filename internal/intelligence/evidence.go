package intelligence

import (
	"fmt"
	"strings"
)

// ValidateEvidenceBindings checks that every factor in the explanation
// references a real key from the trace. Returns an error listing all
// invalid references.
func ValidateEvidenceBindings(factors []ExplanationFactor, traceKeys map[string]bool) error {
	var invalid []string
	for _, f := range factors {
		if f.EvidenceRefKey == "" {
			invalid = append(invalid, fmt.Sprintf("factor %q: empty evidence_ref_key", f.Name))
			continue
		}
		if !traceKeys[f.EvidenceRefKey] {
			invalid = append(invalid, fmt.Sprintf("factor %q: unknown evidence_ref_key %q", f.Name, f.EvidenceRefKey))
		}
	}
	if len(invalid) > 0 {
		return fmt.Errorf("invalid evidence bindings: %s", strings.Join(invalid, "; "))
	}
	return nil
}
