package contract

import "github.com/alexanderramin/kairos/internal/app"

type WhatNowRequest = app.WhatNowRequest

func NewWhatNowRequest(availableMin int) WhatNowRequest {
	return app.NewWhatNowRequest(availableMin)
}

type WhatNowResponse = app.WhatNowResponse

type WhatNowErrorCode = app.WhatNowErrorCode

const (
	ErrInvalidAvailableMin WhatNowErrorCode = app.ErrInvalidAvailableMin
	ErrNoCandidates        WhatNowErrorCode = app.ErrNoCandidates
	ErrDataIntegrity       WhatNowErrorCode = app.ErrDataIntegrity
	ErrInternalError       WhatNowErrorCode = app.ErrInternalError
)

type WhatNowError = app.WhatNowError
