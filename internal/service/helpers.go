package service

import (
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/alexanderramin/kairos/internal/repository"
)

// filterProjectsByScope returns only projects whose ID is in scope.
// If scope is empty, all projects are returned unchanged.
func filterProjectsByScope(projects []*domain.Project, scope []string) []*domain.Project {
	if len(scope) == 0 {
		return projects
	}
	scopeSet := make(map[string]bool, len(scope))
	for _, id := range scope {
		scopeSet[id] = true
	}
	var filtered []*domain.Project
	for _, p := range projects {
		if scopeSet[p.ID] {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// filterCandidatesByScope returns only candidates whose ProjectID is in scope.
// If scope is empty, all candidates are returned unchanged.
func filterCandidatesByScope(candidates []repository.SchedulableCandidate, scope []string) []repository.SchedulableCandidate {
	if len(scope) == 0 {
		return candidates
	}
	scopeSet := make(map[string]bool, len(scope))
	for _, id := range scope {
		scopeSet[id] = true
	}
	var filtered []repository.SchedulableCandidate
	for _, c := range candidates {
		if scopeSet[c.ProjectID] {
			filtered = append(filtered, c)
		}
	}
	return filtered
}
