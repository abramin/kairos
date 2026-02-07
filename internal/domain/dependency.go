package domain

type Dependency struct {
	PredecessorWorkItemID string
	SuccessorWorkItemID   string
}
