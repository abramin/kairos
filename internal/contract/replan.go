package contract

import (
	"github.com/alexanderramin/kairos/internal/app"
	"github.com/alexanderramin/kairos/internal/domain"
)

type ReplanRequest = app.ReplanRequest

func NewReplanRequest(trigger domain.ReplanTrigger) ReplanRequest {
	return app.NewReplanRequest(trigger)
}

type ProjectReplanDelta = app.ProjectReplanDelta

type ReplanResponse = app.ReplanResponse

type ReplanExplanation = app.ReplanExplanation

type ReplanErrorCode = app.ReplanErrorCode

const (
	ReplanErrInvalidTrigger   ReplanErrorCode = app.ReplanErrInvalidTrigger
	ReplanErrNoActiveProjects ReplanErrorCode = app.ReplanErrNoActiveProjects
	ReplanErrDataIntegrity    ReplanErrorCode = app.ReplanErrDataIntegrity
	ReplanErrInternal         ReplanErrorCode = app.ReplanErrInternal
)

type ReplanError = app.ReplanError
