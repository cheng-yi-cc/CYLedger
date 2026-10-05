package api

import (
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
)

type StatisticsWorkspaceApi struct{}

var StatisticsWorkspace = &StatisticsWorkspaceApi{}

func (a *StatisticsWorkspaceApi) Preferences(c *core.WebContext) (any, *errs.Error) {
	return investmentResponse(services.StatisticsWorkspace.Preferences(c, c.GetCurrentUid()))
}
func (a *StatisticsWorkspaceApi) SavePreferences(c *core.WebContext) (any, *errs.Error) {
	var req models.StatisticsPreferences
	if err := investmentBind(c, &req); err != nil {
		return nil, err
	}
	return investmentResponse(services.StatisticsWorkspace.SavePreferences(c, c.GetCurrentUid(), req))
}
func (a *StatisticsWorkspaceApi) Budgets(c *core.WebContext) (any, *errs.Error) {
	return investmentResponse(services.StatisticsWorkspace.Budgets(c, c.GetCurrentUid()))
}
func (a *StatisticsWorkspaceApi) SaveBudget(c *core.WebContext) (any, *errs.Error) {
	var req models.StatisticsBudget
	if err := investmentBind(c, &req); err != nil {
		return nil, err
	}
	return investmentResponse(services.StatisticsWorkspace.SaveBudget(c, c.GetCurrentUid(), req))
}
func (a *StatisticsWorkspaceApi) DeleteBudget(c *core.WebContext) (any, *errs.Error) {
	var req models.StatisticsDeleteRequest
	if err := investmentBind(c, &req); err != nil {
		return nil, err
	}
	return investmentResponse(true, services.StatisticsWorkspace.DeleteBudget(c, c.GetCurrentUid(), req))
}
func (a *StatisticsWorkspaceApi) Notes(c *core.WebContext) (any, *errs.Error) {
	return investmentResponse(services.StatisticsWorkspace.Notes(c, c.GetCurrentUid()))
}
func (a *StatisticsWorkspaceApi) SaveNote(c *core.WebContext) (any, *errs.Error) {
	var req models.StatisticsNote
	if err := investmentBind(c, &req); err != nil {
		return nil, err
	}
	return investmentResponse(services.StatisticsWorkspace.SaveNote(c, c.GetCurrentUid(), req))
}
func (a *StatisticsWorkspaceApi) Auxiliary(c *core.WebContext) (any, *errs.Error) {
	return investmentResponse(services.StatisticsWorkspace.Auxiliary(c, c.GetCurrentUid()))
}
