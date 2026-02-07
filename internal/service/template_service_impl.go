package service

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/repository"
	tmpl "github.com/alexanderramin/kairos/internal/template"
)

type templateService struct {
	templateDir string
	projects    repository.ProjectRepo
	nodes       repository.PlanNodeRepo
	workItems   repository.WorkItemRepo
	deps        repository.DependencyRepo
}

func NewTemplateService(
	templateDir string,
	projects repository.ProjectRepo,
	nodes repository.PlanNodeRepo,
	workItems repository.WorkItemRepo,
	deps repository.DependencyRepo,
) TemplateService {
	return &templateService{
		templateDir: templateDir,
		projects:    projects,
		nodes:       nodes,
		workItems:   workItems,
		deps:        deps,
	}
}

func (s *templateService) List(ctx context.Context) ([]domain.Template, error) {
	files, err := filepath.Glob(filepath.Join(s.templateDir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("listing templates: %w", err)
	}

	var templates []domain.Template
	for _, f := range files {
		schema, err := tmpl.LoadSchema(f)
		if err != nil {
			continue // skip invalid templates
		}
		templates = append(templates, domain.Template{
			ID:      schema.ID,
			Name:    schema.Name,
			Domain:  schema.Domain,
			Version: schema.Version,
		})
	}
	return templates, nil
}

func (s *templateService) Get(ctx context.Context, name string) (*domain.Template, error) {
	path, err := s.resolveTemplatePath(name)
	if err != nil {
		return nil, err
	}

	schema, err := tmpl.LoadSchema(path)
	if err != nil {
		return nil, fmt.Errorf("template '%s' not found: %w", name, err)
	}

	data, _ := json.Marshal(schema)
	return &domain.Template{
		ID:         schema.ID,
		Name:       schema.Name,
		Domain:     schema.Domain,
		Version:    schema.Version,
		ConfigJSON: string(data),
	}, nil
}

func (s *templateService) InitProject(ctx context.Context, templateName, projectName, shortID, startDate string, dueDate *string, vars map[string]string) (*domain.Project, error) {
	path, err := s.resolveTemplatePath(templateName)
	if err != nil {
		return nil, err
	}

	schema, err := tmpl.LoadSchema(path)
	if err != nil {
		return nil, fmt.Errorf("template '%s' not found: %w", templateName, err)
	}

	generated, err := tmpl.Execute(schema, projectName, startDate, dueDate, vars)
	if err != nil {
		return nil, fmt.Errorf("executing template: %w", err)
	}

	// Set short ID on generated project
	generated.Project.ShortID = shortID

	// Persist project
	if err := s.projects.Create(ctx, generated.Project); err != nil {
		return nil, fmt.Errorf("creating project: %w", err)
	}

	// Persist nodes
	for _, node := range generated.Nodes {
		if err := s.nodes.Create(ctx, node); err != nil {
			return nil, fmt.Errorf("creating node '%s': %w", node.Title, err)
		}
	}

	// Persist work items
	for _, wi := range generated.WorkItems {
		if err := s.workItems.Create(ctx, wi); err != nil {
			return nil, fmt.Errorf("creating work item '%s': %w", wi.Title, err)
		}
	}

	// Persist dependencies
	for _, dep := range generated.Dependencies {
		if err := s.deps.Create(ctx, &dep); err != nil {
			return nil, fmt.Errorf("creating dependency: %w", err)
		}
	}

	return generated.Project, nil
}

func (s *templateService) resolveTemplatePath(name string) (string, error) {
	input := strings.TrimSpace(name)
	if input == "" {
		return "", fmt.Errorf("template '%s' not found: empty template name", name)
	}

	// Fast path: treat input as file stem (current behavior).
	stemPath := filepath.Join(s.templateDir, input+".json")
	if schema, err := tmpl.LoadSchema(stemPath); err == nil && schema != nil {
		return stemPath, nil
	}

	// Also allow passing an explicit .json filename.
	if strings.HasSuffix(strings.ToLower(input), ".json") {
		filenamePath := filepath.Join(s.templateDir, input)
		if schema, err := tmpl.LoadSchema(filenamePath); err == nil && schema != nil {
			return filenamePath, nil
		}
	}

	// Fallback: resolve by schema ID or display name (case-insensitive).
	files, err := filepath.Glob(filepath.Join(s.templateDir, "*.json"))
	if err != nil {
		return "", fmt.Errorf("template '%s' not found: listing templates: %w", name, err)
	}

	for _, file := range files {
		schema, err := tmpl.LoadSchema(file)
		if err != nil {
			continue
		}

		fileStem := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
		if strings.EqualFold(fileStem, input) || strings.EqualFold(schema.ID, input) || strings.EqualFold(schema.Name, input) {
			return file, nil
		}
	}

	return "", fmt.Errorf("template '%s' not found: open %s: no such file or directory", name, stemPath)
}
