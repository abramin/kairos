package contract

import "github.com/alexanderramin/kairos/internal/app"

type StatusRequest = app.StatusRequest

func NewStatusRequest() StatusRequest {
	return app.NewStatusRequest()
}

type ProjectStatusView = app.ProjectStatusView

type GlobalStatusSummary = app.GlobalStatusSummary

type StatusResponse = app.StatusResponse

type StatusErrorCode = app.StatusErrorCode

const (
	StatusErrInvalidScope StatusErrorCode = app.StatusErrInvalidScope
)

type StatusError = app.StatusError
