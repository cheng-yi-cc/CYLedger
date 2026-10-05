package api

import (
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
	"time"
)

type AssetToolsApi struct{}

func (a *AssetToolsApi) AdjustBalance(c *core.WebContext) (any, *errs.Error) {
	var input services.AssetAdjustmentInput
	if err := investmentBind(c, &input); err != nil {
		return nil, err
	}
	return investmentResponse(services.Investments.AdjustAssetBalance(c, c.GetCurrentUid(), input))
}

var AssetTools = &AssetToolsApi{}

func (a *AssetToolsApi) DebtReports(c *core.WebContext) (any, *errs.Error) {
	return investmentResponse(services.Investments.DebtReports(c, c.GetCurrentUid()))
}
func (a *AssetToolsApi) RecordDebt(c *core.WebContext) (any, *errs.Error) {
	var input services.DebtMovementInput
	if err := investmentBind(c, &input); err != nil {
		return nil, err
	}
	return investmentResponse(services.Investments.RecordDebtMovement(c, c.GetCurrentUid(), input))
}

func (a *AssetToolsApi) CreditReports(c *core.WebContext) (any, *errs.Error) {
	return investmentResponse(services.Investments.CreditReports(c, c.GetCurrentUid(), c.Query("accountId"), c.Query("timeZone")))
}
func (a *AssetToolsApi) Installments(c *core.WebContext) (any, *errs.Error) {
	return investmentResponse(services.Investments.CreditInstallments(c, c.GetCurrentUid()))
}
func (a *AssetToolsApi) PreviewInstallment(c *core.WebContext) (any, *errs.Error) {
	var input services.CreditInstallmentInput
	if err := investmentBind(c, &input); err != nil {
		return nil, err
	}
	return investmentResponse(services.Investments.PreviewCreditInstallment(input))
}
func (a *AssetToolsApi) SaveInstallment(c *core.WebContext) (any, *errs.Error) {
	var input services.CreditInstallmentInput
	if err := investmentBind(c, &input); err != nil {
		return nil, err
	}
	return investmentResponse(services.Investments.SaveCreditInstallment(c, c.GetCurrentUid(), input))
}
func (a *AssetToolsApi) CloseInstallment(c *core.WebContext) (any, *errs.Error) {
	var input struct {
		Id      string `json:"id" binding:"required,max=64"`
		Version string `json:"version" binding:"required,max=20"`
	}
	if err := investmentBind(c, &input); err != nil {
		return nil, err
	}
	return investmentResponse(services.Investments.CloseCreditInstallment(c, c.GetCurrentUid(), input.Id, input.Version))
}
func (a *AssetToolsApi) SyncInstallments(c *core.WebContext) (any, *errs.Error) {
	return investmentResponse(services.Investments.SyncCreditInstallments(c, c.GetCurrentUid(), time.Now()))
}

func (a *AssetToolsApi) Preferences(c *core.WebContext) (any, *errs.Error) {
	return investmentResponse(services.Investments.AssetPreferences(c, c.GetCurrentUid()))
}
func (a *AssetToolsApi) SavePreferences(c *core.WebContext) (any, *errs.Error) {
	var input models.AssetPreferences
	if err := investmentBind(c, &input); err != nil {
		return nil, err
	}
	return investmentResponse(services.Investments.SaveAssetPreferences(c, c.GetCurrentUid(), input))
}
func (a *AssetToolsApi) Deposits(c *core.WebContext) (any, *errs.Error) {
	return investmentResponse(services.Investments.FixedDeposits(c, c.GetCurrentUid()))
}
func (a *AssetToolsApi) SaveDeposit(c *core.WebContext) (any, *errs.Error) {
	var input models.FixedDeposit
	if err := investmentBind(c, &input); err != nil {
		return nil, err
	}
	return investmentResponse(services.Investments.SaveFixedDeposit(c, c.GetCurrentUid(), input))
}
func (a *AssetToolsApi) CloseDeposit(c *core.WebContext) (any, *errs.Error) {
	var input struct {
		Id string `json:"id" binding:"required,max=64"`
	}
	if err := investmentBind(c, &input); err != nil {
		return nil, err
	}
	return investmentResponse(services.Investments.CloseFixedDeposit(c, c.GetCurrentUid(), input.Id))
}
func (a *AssetToolsApi) SyncDeposits(c *core.WebContext) (any, *errs.Error) {
	return investmentResponse(services.Investments.SyncFixedDeposits(c, c.GetCurrentUid(), time.Now()))
}

func (a *AssetToolsApi) Reimbursements(c *core.WebContext) (any, *errs.Error) {
	return investmentResponse(services.Investments.ReimbursementClaims(c, c.GetCurrentUid()))
}
func (a *AssetToolsApi) ReceiveReimbursement(c *core.WebContext) (any, *errs.Error) {
	var input services.ReimbursementReceiveRequest
	if err := investmentBind(c, &input); err != nil {
		return nil, err
	}
	return investmentResponse(services.Investments.ReceiveReimbursement(c, c.GetCurrentUid(), input))
}
func (a *AssetToolsApi) ReimbursementState(c *core.WebContext) (any, *errs.Error) {
	var input struct {
		ExpenseId int64  `json:"expenseId,string" binding:"required,min=1"`
		Action    string `json:"action" binding:"required,oneof=end reopen unassign"`
	}
	if err := investmentBind(c, &input); err != nil {
		return nil, err
	}
	return investmentResponse(services.Investments.SetReimbursementState(c, c.GetCurrentUid(), input.ExpenseId, input.Action))
}
