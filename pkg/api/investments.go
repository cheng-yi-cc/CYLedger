package api

import (
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/marketquotes"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
	"net/http"
	"strings"
)

type InvestmentsApi struct{}

var Investments = &InvestmentsApi{}

func investmentResponse(value any, err error) (any, *errs.Error) {
	if err == nil {
		return value, nil
	}
	return nil, errs.Or(err, errs.ErrOperationFailed)
}
func investmentBind(c *core.WebContext, v any) *errs.Error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64*1024)
	if err := c.ShouldBindJSON(v); err != nil {
		return errs.NewIncompleteOrIncorrectSubmissionError(err)
	}
	return nil
}
func (a *InvestmentsApi) Settings(c *core.WebContext) (any, *errs.Error) {
	if c.Request.Method == "GET" {
		return investmentResponse(services.Investments.Settings(c, c.GetCurrentUid()))
	}
	var v models.InvestmentSettings
	if err := investmentBind(c, &v); err != nil {
		return nil, err
	}
	return investmentResponse(services.Investments.SaveSettings(c, c.GetCurrentUid(), v))
}
func (a *InvestmentsApi) Accounts(c *core.WebContext) (any, *errs.Error) {
	if c.Request.Method == "GET" {
		return investmentResponse(services.Investments.Accounts(c, c.GetCurrentUid()))
	}
	var v models.PortfolioAccount
	if err := investmentBind(c, &v); err != nil {
		return nil, err
	}
	return investmentResponse(services.Investments.CreateAccount(c, c.GetCurrentUid(), v))
}
func (a *InvestmentsApi) PreviewAccountDeletion(c *core.WebContext) (any, *errs.Error) {
	return a.deleteAccount(c, true)
}
func (a *InvestmentsApi) UpdateAccount(c *core.WebContext) (any, *errs.Error) {
	var input models.PortfolioAccount
	if err := investmentBind(c, &input); err != nil {
		return nil, err
	}
	return investmentResponse(services.Investments.UpdatePortfolioAccount(c, c.GetCurrentUid(), input))
}
func (a *InvestmentsApi) DeleteAccount(c *core.WebContext) (any, *errs.Error) {
	return a.deleteAccount(c, false)
}
func (a *InvestmentsApi) deleteAccount(c *core.WebContext, preview bool) (any, *errs.Error) {
	var input services.AccountDeletionInput
	if err := investmentBind(c, &input); err != nil {
		return nil, err
	}
	return investmentResponse(services.Investments.DeleteAssetAccount(c, c.GetCurrentUid(), input, preview))
}
func (a *InvestmentsApi) Instruments(c *core.WebContext) (any, *errs.Error) {
	if c.Request.Method == "GET" {
		return investmentResponse(services.Investments.Instruments(c, c.GetCurrentUid()))
	}
	var v models.InvestmentInstrument
	if err := investmentBind(c, &v); err != nil {
		return nil, err
	}
	return investmentResponse(services.Investments.CreateInstrument(c, c.GetCurrentUid(), v))
}

func (a *InvestmentsApi) CryptoAccount(c *core.WebContext) (any, *errs.Error) {
	var input services.CryptoAccountInput
	if err := investmentBind(c, &input); err != nil {
		return nil, err
	}
	return investmentResponse(services.Investments.CreateCryptoAccount(c, c.GetCurrentUid(), input, c.GetHeader("Idempotency-Key")))
}
func (a *InvestmentsApi) Conversion(c *core.WebContext) (any, *errs.Error) {
	var input services.ConversionInput
	if err := investmentBind(c, &input); err != nil {
		return nil, err
	}
	return investmentResponse(services.Investments.Conversion(c, c.GetCurrentUid(), input))
}
func (a *InvestmentsApi) SearchInstruments(c *core.WebContext) (any, *errs.Error) {
	return investmentResponse(services.Investments.SearchInstruments(c, c.Query("q"), c.Query("market")))
}
func (a *InvestmentsApi) BindInstrument(c *core.WebContext) (any, *errs.Error) {
	var input struct {
		InstrumentID string `json:"instrumentId"`
		marketquotes.Binding
	}
	if err := investmentBind(c, &input); err != nil {
		return nil, err
	}
	return investmentResponse(services.Investments.BindInstrument(c, c.GetCurrentUid(), input.InstrumentID, input.Binding))
}
func (a *InvestmentsApi) Events(c *core.WebContext) (any, *errs.Error) {
	if c.Request.Method == "GET" {
		return investmentResponse(services.Investments.Events(c, c.GetCurrentUid()))
	}
	return a.mutate(c, "create", false)
}
func (a *InvestmentsApi) Preview(c *core.WebContext) (any, *errs.Error) {
	return a.mutate(c, "preview", true)
}
func (a *InvestmentsApi) Revise(c *core.WebContext) (any, *errs.Error) {
	return a.mutate(c, "revise", false)
}
func (a *InvestmentsApi) Void(c *core.WebContext) (any, *errs.Error) {
	return a.mutate(c, "void", false)
}
func (a *InvestmentsApi) mutate(c *core.WebContext, op string, preview bool) (any, *errs.Error) {
	var v services.InvestmentEvent
	if err := investmentBind(c, &v); err != nil {
		return nil, err
	}
	if op == "preview" {
		op = "create"
		if v.ID != "" {
			op = "revise"
		}
	}
	if !preview && op != "create" {
		v.ID = c.Param("id")
	}
	return investmentResponse(services.Investments.Mutate(c, c.GetCurrentUid(), v, c.GetHeader("Idempotency-Key"), op, preview))
}
func (a *InvestmentsApi) Positions(c *core.WebContext) (any, *errs.Error) {
	summary, err := services.Investments.Summary(c, c.GetCurrentUid(), false)
	if err != nil {
		return investmentResponse(nil, err)
	}
	return summary.Positions, nil
}
func (a *InvestmentsApi) Summary(c *core.WebContext) (any, *errs.Error) {
	return investmentResponse(services.Investments.Summary(c, c.GetCurrentUid(), true))
}
func (a *InvestmentsApi) History(c *core.WebContext) (any, *errs.Error) {
	return investmentResponse(services.Investments.History(c, c.GetCurrentUid()))
}
func (a *InvestmentsApi) Quotes(c *core.WebContext) (any, *errs.Error) {
	return investmentResponse(services.Investments.Quotes(c, c.GetCurrentUid()))
}
func (a *InvestmentsApi) ManualQuote(c *core.WebContext) (any, *errs.Error) {
	var v struct {
		InstrumentID string `json:"instrumentId"`
		Price        string `json:"price"`
		Currency     string `json:"currency"`
		AsOf         int64  `json:"asOf"`
		Automatic    bool   `json:"automatic"`
	}
	if err := investmentBind(c, &v); err != nil {
		return nil, err
	}
	if v.Automatic {
		return investmentResponse(true, services.Investments.RemoveManualQuote(c, c.GetCurrentUid(), v.InstrumentID))
	}
	return investmentResponse(services.Investments.ManualQuote(c, c.GetCurrentUid(), v.InstrumentID, v.Price, v.AsOf, v.Currency))
}
func (a *InvestmentsApi) Export(c *core.WebContext) (any, *errs.Error) {
	return investmentResponse(services.Investments.Export(c, c.GetCurrentUid()))
}

func (a *InvestmentsApi) HoldingProfiles(c *core.WebContext) (any, *errs.Error) {
	if c.Request.Method == "GET" {
		return investmentResponse(services.Investments.HoldingProfiles(c, c.GetCurrentUid()))
	}
	var input models.InvestmentHoldingProfile
	if err := investmentBind(c, &input); err != nil {
		return nil, err
	}
	return investmentResponse(services.Investments.SaveHoldingProfile(c, c.GetCurrentUid(), input))
}
func (a *InvestmentsApi) SetupHolding(c *core.WebContext) (any, *errs.Error) {
	var input services.HoldingSetupInput
	if err := investmentBind(c, &input); err != nil {
		return nil, err
	}
	return investmentResponse(services.Investments.SetupHolding(c, c.GetCurrentUid(), input, c.GetHeader("Idempotency-Key")))
}
func (a *InvestmentsApi) Plans(c *core.WebContext) (any, *errs.Error) {
	if c.Request.Method == "GET" {
		return investmentResponse(services.Investments.InvestmentPlans(c, c.GetCurrentUid()))
	}
	var input models.InvestmentPlan
	if err := investmentBind(c, &input); err != nil {
		return nil, err
	}
	return investmentResponse(services.Investments.SaveInvestmentPlan(c, c.GetCurrentUid(), input))
}
func (a *InvestmentsApi) Orders(c *core.WebContext) (any, *errs.Error) {
	if c.Request.Method == "GET" {
		return investmentResponse(services.Investments.InvestmentOrders(c, c.GetCurrentUid()))
	}
	var input models.InvestmentOrder
	if err := investmentBind(c, &input); err != nil {
		return nil, err
	}
	return investmentResponse(services.Investments.SaveInvestmentOrder(c, c.GetCurrentUid(), input, c.GetHeader("Idempotency-Key")))
}
func (a *InvestmentsApi) ConfirmOrder(c *core.WebContext) (any, *errs.Error) {
	var input services.InvestmentOrderConfirmation
	if err := investmentBind(c, &input); err != nil {
		return nil, err
	}
	return investmentResponse(services.Investments.ConfirmInvestmentOrder(c, c.GetCurrentUid(), input))
}
func (a *InvestmentsApi) CancelOrder(c *core.WebContext) (any, *errs.Error) {
	var input services.InvestmentOrderConfirmation
	if err := investmentBind(c, &input); err != nil {
		return nil, err
	}
	return investmentResponse(true, services.Investments.CancelInvestmentOrder(c, c.GetCurrentUid(), input.Id, input.Version))
}
func (a *InvestmentsApi) SyncPlans(c *core.WebContext) (any, *errs.Error) {
	var input struct {
		Force bool `json:"force"`
	}
	if err := investmentBind(c, &input); err != nil {
		return nil, err
	}
	return investmentResponse(services.Investments.SyncInvestmentPlans(c, c.GetCurrentUid(), input.Force))
}
func (a *InvestmentsApi) Report(c *core.WebContext) (any, *errs.Error) {
	if c.Query("assetScope") == "1" {
		var books []string
		if ids := c.Query("bookIds"); ids != "" {
			books = strings.Split(ids, ",")
		}
		return investmentResponse(services.Investments.InvestmentReport(c, c.GetCurrentUid(), c.Query("accountId"), c.Query("instrumentId"), services.InvestmentReportScope{BookIds: books}))
	}
	return investmentResponse(services.Investments.InvestmentReport(c, c.GetCurrentUid(), c.Query("accountId"), c.Query("instrumentId")))
}
func (a *InvestmentsApi) UpdateHolding(c *core.WebContext) (any, *errs.Error) {
	var input services.HoldingSetupInput
	if err := investmentBind(c, &input); err != nil {
		return nil, err
	}
	return investmentResponse(services.Investments.UpdateHolding(c, c.GetCurrentUid(), input, c.GetHeader("Idempotency-Key")))
}
