package service

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/repository"
	"github.com/google/uuid"
)

var shortIDPattern = regexp.MustCompile(`^[A-Z]{3,6}[0-9]{2,4}$`)

type projectService struct {
	projects repository.ProjectRepo
}

func NewProjectService(projects repository.ProjectRepo) ProjectService {
	return &projectService{projects: projects}
}

func (s *projectService) Create(ctx context.Context, p *domain.Project) error {
	if p.ShortID == "" {
		return fmt.Errorf("short ID is required (use --id flag)")
	}
	if !shortIDPattern.MatchString(p.ShortID) {
		return fmt.Errorf("short ID %q must be 3-6 uppercase letters followed by 2-4 digits (e.g. PHI01)", p.ShortID)
	}
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	p.CreatedAt = now
	p.UpdatedAt = now
	if p.Status == "" {
		p.Status = domain.ProjectActive
	}
	return s.projects.Create(ctx, p)
}

func (s *projectService) GetByID(ctx context.Context, id string) (*domain.Project, error) {
	return s.projects.GetByID(ctx, id)
}

func (s *projectService) List(ctx context.Context, includeArchived bool) ([]*domain.Project, error) {
	return s.projects.List(ctx, includeArchived)
}

func (s *projectService) Update(ctx context.Context, p *domain.Project) error {
	p.UpdatedAt = time.Now().UTC()
	return s.projects.Update(ctx, p)
}

func (s *projectService) Archive(ctx context.Context, id string) error {
	return s.projects.Archive(ctx, id)
}

func (s *projectService) Unarchive(ctx context.Context, id string) error {
	return s.projects.Unarchive(ctx, id)
}

func (s *projectService) Delete(ctx context.Context, id string, force bool) error {
	if !force {
		p, err := s.projects.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if p.Status != domain.ProjectArchived {
			return fmt.Errorf("project must be archived before deletion (use --force to override)")
		}
	}
	return s.projects.Delete(ctx, id)
}
