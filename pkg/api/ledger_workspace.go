package api

import (
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
)

type LedgerWorkspaceApi struct{}

var LedgerWorkspace = &LedgerWorkspaceApi{}

func (a *LedgerWorkspaceApi) Items(c *core.WebContext) (any, *errs.Error) {
	return investmentResponse(services.LedgerWorkspace.Items(c, c.GetCurrentUid(), c.Query("kind")))
}
func (a *LedgerWorkspaceApi) Save(c *core.WebContext) (any, *errs.Error) {
	var input models.LedgerWorkspaceItem
	if err := investmentBind(c, &input); err != nil {
		return nil, err
	}
	return investmentResponse(services.LedgerWorkspace.Save(c, c.GetCurrentUid(), input))
}
func (a *LedgerWorkspaceApi) Delete(c *core.WebContext) (any, *errs.Error) {
	var input models.LedgerWorkspaceItem
	if err := investmentBind(c, &input); err != nil {
		return nil, err
	}
	return investmentResponse(true, services.LedgerWorkspace.Delete(c, c.GetCurrentUid(), input))
}

func (a *LedgerWorkspaceApi) ImportBatches(c *core.WebContext) (any, *errs.Error) {
	return investmentResponse(services.LedgerWorkspace.ImportBatches(c, c.GetCurrentUid()))
}
func (a *LedgerWorkspaceApi) UndoImportBatch(c *core.WebContext) (any, *errs.Error) {
	var input struct {
		Id string `json:"id" binding:"required,max=64"`
	}
	if err := investmentBind(c, &input); err != nil {
		return nil, err
	}
	return investmentResponse(true, services.LedgerWorkspace.UndoImportBatch(c, c.GetCurrentUid(), input.Id))
}
