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

func TestTemplateServiceGet_ResolvesByNumericID(t *testing.T) {
	t.Parallel()

	templateDir := t.TempDir()

	alphaTemplate := `{
  "id": "alpha_template",
  "name": "Alpha Template",
  "version": "1.0.0",
  "domain": "general"
}`
	if err := os.WriteFile(filepath.Join(templateDir, "alpha_template.json"), []byte(alphaTemplate), 0o644); err != nil {
		t.Fatalf("write alpha template: %v", err)
	}

	betaTemplate := `{
  "id": "beta_template",
  "name": "Beta Template",
  "version": "1.0.0",
  "domain": "general"
}`
	if err := os.WriteFile(filepath.Join(templateDir, "beta_template.json"), []byte(betaTemplate), 0o644); err != nil {
		t.Fatalf("write beta template: %v", err)
	}

	invalidTemplate := `{"id":"broken"`
	if err := os.WriteFile(filepath.Join(templateDir, "invalid_template.json"), []byte(invalidTemplate), 0o644); err != nil {
		t.Fatalf("write invalid template: %v", err)
	}

	svc := NewTemplateService(templateDir, nil, nil, nil, nil)

	first, err := svc.Get(context.Background(), "1")
	if err != nil {
		t.Fatalf("Get(1) error: %v", err)
	}
	if first.Name != "Alpha Template" {
		t.Fatalf("Get(1) returned wrong template: %q", first.Name)
	}
	if first.NumericID != 1 {
		t.Fatalf("Get(1) returned wrong numeric id: %d", first.NumericID)
	}

	second, err := svc.Get(context.Background(), "2")
	if err != nil {
		t.Fatalf("Get(2) error: %v", err)
	}
	if second.Name != "Beta Template" {
		t.Fatalf("Get(2) returned wrong template: %q", second.Name)
	}
	if second.NumericID != 2 {
		t.Fatalf("Get(2) returned wrong numeric id: %d", second.NumericID)
	}

	if _, err := svc.Get(context.Background(), "3"); err == nil {
		t.Fatal("expected error for out-of-range numeric id")
	}
}
