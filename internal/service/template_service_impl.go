package service

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alexanderramin/kairos/internal/db"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/repository"
	tmpl "github.com/alexanderramin/kairos/internal/template"
)

type templateService struct {
	templateDir string
	uow         db.UnitOfWork
	observer    UseCaseObserver
}

type templateEntry struct {
	Index  int
	Path   string
	Schema *tmpl.TemplateSchema
}

func NewTemplateService(
	templateDir string,
	uow db.UnitOfWork,
	observers ...UseCaseObserver,
) TemplateService {
	return &templateService{
		templateDir: templateDir,
		uow:         uow,
		observer:    useCaseObserverOrNoop(observers),
	}
}

func (s *templateService) List(ctx context.Context) ([]domain.Template, error) {
	entries, err := s.loadTemplateEntries()
	if err != nil {
		return nil, fmt.Errorf("listing templates: %w", err)
	}

	templates := make([]domain.Template, 0, len(entries))
	for _, entry := range entries {
		templates = append(templates, domain.Template{
			NumericID: entry.Index,
			ID:        entry.Schema.ID,
			Name:      entry.Schema.Name,
			Domain:    entry.Schema.Domain,
			Version:   entry.Schema.Version,
		})
	}
	return templates, nil
}

func (s *templateService) Get(ctx context.Context, name string) (*domain.Template, error) {
	entry, err := s.resolveTemplate(name)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(entry.Schema)
	if err != nil {
		return nil, fmt.Errorf("encoding template '%s': %w", name, err)
	}

	return &domain.Template{
		NumericID:  entry.Index,
		ID:         entry.Schema.ID,
		Name:       entry.Schema.Name,
		Domain:     entry.Schema.Domain,
		Version:    entry.Schema.Version,
		ConfigJSON: string(data),
	}, nil
}

func (s *templateService) InitProject(ctx context.Context, templateName, projectName, shortID, startDate string, dueDate *string, vars map[string]string) (project *domain.Project, err error) {
	startedAt := time.Now().UTC()
	fields := map[string]any{
		"template": templateName,
		"project":  projectName,
	}
	defer func() {
		s.observer.ObserveUseCase(ctx, UseCaseEvent{
			Name:      "init-project",
			StartedAt: startedAt,
			Duration:  time.Since(startedAt),
			Success:   err == nil,
			Err:       err,
			Fields:    fields,
		})
	}()

	var entry *templateEntry
	entry, err = s.resolveTemplate(templateName)
	if err != nil {
		return nil, err
	}

	var generated *tmpl.GeneratedProject
	generated, err = tmpl.Execute(entry.Schema, projectName, startDate, dueDate, vars)
	if err != nil {
		return nil, fmt.Errorf("executing template: %w", err)
	}
	fields["node_count"] = len(generated.Nodes)
	fields["work_item_count"] = len(generated.WorkItems)
	fields["dependency_count"] = len(generated.Dependencies)

	// Set short ID on generated project
	generated.Project.ShortID = shortID

	// Assign sequential IDs in tree order
	wiByNode := make(map[string][]*domain.WorkItem, len(generated.Nodes))
	for _, wi := range generated.WorkItems {
		wiByNode[wi.NodeID] = append(wiByNode[wi.NodeID], wi)
	}
	seq := 1
	for _, node := range generated.Nodes {
		node.Seq = seq
		seq++
		for _, wi := range wiByNode[node.ID] {
			wi.Seq = seq
			seq++
		}
	}

	// Persist all entities atomically
	err = s.uow.WithinTx(ctx, func(ctx context.Context, tx db.DBTX) error {
		txProjects := repository.NewSQLiteProjectRepo(tx)
		txNodes := repository.NewSQLitePlanNodeRepo(tx)
		txWorkItems := repository.NewSQLiteWorkItemRepo(tx)
		txDeps := repository.NewSQLiteDependencyRepo(tx)

		if err := txProjects.Create(ctx, generated.Project); err != nil {
			return fmt.Errorf("creating project: %w", err)
		}

		for _, node := range generated.Nodes {
			if err := txNodes.Create(ctx, node); err != nil {
				return fmt.Errorf("creating node '%s': %w", node.Title, err)
			}
		}

		for _, wi := range generated.WorkItems {
			if err := txWorkItems.Create(ctx, wi); err != nil {
				return fmt.Errorf("creating work item '%s': %w", wi.Title, err)
			}
		}

		for _, dep := range generated.Dependencies {
			if err := txDeps.Create(ctx, &dep); err != nil {
				return fmt.Errorf("creating dependency: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	project = generated.Project
	return project, nil
}

func (s *templateService) resolveTemplate(name string) (*templateEntry, error) {
	input := strings.TrimSpace(name)
	if input == "" {
		return nil, fmt.Errorf("template '%s' not found: empty template name", name)
	}

	entries, err := s.loadTemplateEntries()
	if err != nil {
		return nil, fmt.Errorf("template '%s' not found: listing templates: %w", name, err)
	}

	// Resolve by file stem, filename, schema ID, or display name (case-insensitive).
	for i := range entries {
		entry := &entries[i]
		fileStem := strings.TrimSuffix(filepath.Base(entry.Path), filepath.Ext(entry.Path))
		filename := filepath.Base(entry.Path)
		if strings.EqualFold(fileStem, input) ||
			strings.EqualFold(filename, input) ||
			strings.EqualFold(entry.Schema.ID, input) ||
			strings.EqualFold(entry.Schema.Name, input) {
			return entry, nil
		}
	}

	// Resolve by integer selector from `template list`.
	if numericID, err := strconv.Atoi(input); err == nil {
		for i := range entries {
			entry := &entries[i]
			if entry.Index == numericID {
				return entry, nil
			}
		}
	}

	stemPath := filepath.Join(s.templateDir, input+".json")
	return nil, fmt.Errorf("template '%s' not found: open %s: no such file or directory", name, stemPath)
}

func (s *templateService) loadTemplateEntries() ([]templateEntry, error) {
	files, err := filepath.Glob(filepath.Join(s.templateDir, "*.json"))
	if err != nil {
		return nil, err
	}

	entries := make([]templateEntry, 0, len(files))
	for _, file := range files {
		schema, err := tmpl.LoadSchema(file)
		if err != nil {
			continue // skip invalid templates
		}

		entries = append(entries, templateEntry{
			Index:  len(entries) + 1,
			Path:   file,
			Schema: schema,
		})
	}

	return entries, nil
}
