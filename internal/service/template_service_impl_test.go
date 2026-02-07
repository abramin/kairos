package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestTemplateServiceGet_ResolvesByStemIDAndName(t *testing.T) {
	t.Parallel()

	templateDir := t.TempDir()
	templateJSON := `{
  "id": "ou_module_weekly",
  "name": "OU Module Weekly",
  "version": "1.0.0",
  "domain": "education"
}`

	path := filepath.Join(templateDir, "ou_module_weekly.json")
	if err := os.WriteFile(path, []byte(templateJSON), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	svc := NewTemplateService(templateDir, nil, nil, nil, nil)

	tests := []string{
		"ou_module_weekly",
		"OU_MODULE_WEEKLY",
		"OU Module Weekly",
		"ou_module_weekly.json",
	}

	for _, input := range tests {
		input := input
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			got, err := svc.Get(context.Background(), input)
			if err != nil {
				t.Fatalf("Get(%q) error: %v", input, err)
			}
			if got.Name != "OU Module Weekly" {
				t.Fatalf("Get(%q) returned wrong template name: %q", input, got.Name)
			}
		})
	}
}

func TestTemplateServiceGet_NotFound(t *testing.T) {
	t.Parallel()

	svc := NewTemplateService(t.TempDir(), nil, nil, nil, nil)
	if _, err := svc.Get(context.Background(), "missing"); err == nil {
		t.Fatal("expected error for missing template")
	}
}
