package cli

import "github.com/alexanderramin/kairos/internal/app"

func (a *App) logSessionUseCase() app.LogSessionUseCase {
	if a.LogSession != nil {
		return a.LogSession
	}
	return a.Sessions
}

func (a *App) initProjectUseCase() app.InitProjectUseCase {
	if a.InitProject != nil {
		return a.InitProject
	}
	return a.Templates
}

func (a *App) importProjectUseCase() app.ImportProjectUseCase {
	if a.ImportProject != nil {
		return a.ImportProject
	}
	return a.Import
}
