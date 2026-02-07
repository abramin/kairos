package service

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

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
	path := filepath.Join(s.templateDir, name+".json")
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

func (s *templateService) InitProject(ctx context.Context, templateName, projectName, startDate string, dueDate *string, vars map[string]string) (*domain.Project, error) {
	path := filepath.Join(s.templateDir, templateName+".json")
	schema, err := tmpl.LoadSchema(path)
	if err != nil {
		return nil, fmt.Errorf("template '%s' not found: %w", templateName, err)
	}

	generated, err := tmpl.Execute(schema, projectName, startDate, dueDate, vars)
	if err != nil {
		return nil, fmt.Errorf("executing template: %w", err)
	}

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
