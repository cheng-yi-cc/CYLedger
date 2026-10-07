package api

import (
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
)

func (a *InvestmentsApi) CryptoDCAList(c *core.WebContext) (any, *errs.Error) {
	return investmentResponse(services.Investments.CryptoDCAState(c, c.GetCurrentUid(), time.Now()))
}

func (a *InvestmentsApi) CryptoDCASave(c *core.WebContext) (any, *errs.Error) {
	var req models.CryptoDCASaveRequest
	if err := investmentBind(c, &req); err != nil {
		return nil, err
	}
	return investmentResponse(services.Investments.SaveCryptoDCA(c, c.GetCurrentUid(), req))
}

func (a *InvestmentsApi) CryptoDCAEnabled(c *core.WebContext) (any, *errs.Error) {
	var req struct {
		Id       string `json:"id" binding:"required,max=64"`
		Revision int64  `json:"revision" binding:"required,min=1"`
		Enabled  bool   `json:"enabled"`
	}
	if err := investmentBind(c, &req); err != nil {
		return nil, err
	}
	return investmentResponse(services.Investments.SetCryptoDCAEnabled(c, c.GetCurrentUid(), req.Id, req.Revision, req.Enabled))
}

func (a *InvestmentsApi) CryptoDCASync(c *core.WebContext) (any, *errs.Error) {
	var req struct {
		Force bool `json:"force"`
	}
	if err := investmentBind(c, &req); err != nil {
		return nil, err
	}
	return investmentResponse(services.Investments.SyncCryptoDCA(c, c.GetCurrentUid(), req.Force))
}
