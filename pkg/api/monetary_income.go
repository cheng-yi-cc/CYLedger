package api

import (
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/marketquotes"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
	"strconv"
)

type MonetaryIncomeApi struct{}

var MonetaryIncome = &MonetaryIncomeApi{}
var errMonetaryFundSearchUnavailable = errs.NewNormalError(24, 4, 502, "无法连接基金数据源，请检查网络或 VPN 后重试")

func (a *MonetaryIncomeApi) Search(c *core.WebContext) (any, *errs.Error) {
	items, err := marketquotes.Default.SearchMonetaryFunds(c.Request.Context(), c.Query("q"))
	if err != nil {
		// The upstream URL contains only the public fund query, never ledger data.
		log.Warnf(c, "[monetary_income.Search] public fund source failed, because %s", err.Error())
		return nil, errMonetaryFundSearchUnavailable
	}
	return items, nil
}
func (a *MonetaryIncomeApi) List(c *core.WebContext) (any, *errs.Error) {
	return investmentResponse(services.MonetaryIncome.List(c, c.GetCurrentUid()))
}
func (a *MonetaryIncomeApi) Save(c *core.WebContext) (any, *errs.Error) {
	var req models.MonetaryIncomeSaveRequest
	if err := investmentBind(c, &req); err != nil {
		return nil, err
	}
	return investmentResponse(services.MonetaryIncome.Save(c, c.GetCurrentUid(), req))
}
func (a *MonetaryIncomeApi) Pause(c *core.WebContext) (any, *errs.Error) {
	var req struct {
		AccountId string `json:"accountId" binding:"required,max=24"`
	}
	if err := investmentBind(c, &req); err != nil {
		return nil, err
	}
	id, err := strconv.ParseInt(req.AccountId, 10, 64)
	if err != nil || id <= 0 {
		return nil, services.ErrMonetaryBinding
	}
	return investmentResponse(services.MonetaryIncome.Pause(c, c.GetCurrentUid(), id))
}
func (a *MonetaryIncomeApi) Sync(c *core.WebContext) (any, *errs.Error) {
	var req struct {
		AccountId string `json:"accountId" binding:"max=24"`
		Force     bool   `json:"force"`
	}
	if err := investmentBind(c, &req); err != nil {
		return nil, err
	}
	var id int64
	if req.AccountId != "" {
		var err error
		id, err = strconv.ParseInt(req.AccountId, 10, 64)
		if err != nil || id <= 0 {
			return nil, services.ErrMonetaryBinding
		}
	}
	return investmentResponse(services.MonetaryIncome.Sync(c, c.GetCurrentUid(), id, req.Force))
}
