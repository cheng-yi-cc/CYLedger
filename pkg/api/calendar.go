package api

import (
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
)

type CalendarApi struct{}

var Calendar = &CalendarApi{}

func (a *CalendarApi) Pending(c *core.WebContext) (any, *errs.Error) {
	return investmentResponse(services.Calendar.Pending(c, c.GetCurrentUid()))
}

func (a *CalendarApi) List(c *core.WebContext) (any, *errs.Error) {
	return investmentResponse(services.Calendar.List(c, c.GetCurrentUid(), c.Query("month")))
}
func (a *CalendarApi) Save(c *core.WebContext) (any, *errs.Error) {
	var req models.CalendarSaveRequest
	if err := investmentBind(c, &req); err != nil {
		return nil, err
	}
	return investmentResponse(services.Calendar.Save(c, c.GetCurrentUid(), req))
}
func (a *CalendarApi) Complete(c *core.WebContext) (any, *errs.Error) {
	var req struct {
		Id        string `json:"id" binding:"required,max=64"`
		Completed bool   `json:"completed"`
	}
	if err := investmentBind(c, &req); err != nil {
		return nil, err
	}
	return investmentResponse(services.Calendar.SetCompleted(c, c.GetCurrentUid(), req.Id, req.Completed))
}
func (a *CalendarApi) Delete(c *core.WebContext) (any, *errs.Error) {
	var req struct {
		Id string `json:"id" binding:"required,max=64"`
	}
	if err := investmentBind(c, &req); err != nil {
		return nil, err
	}
	return investmentResponse(services.Calendar.Delete(c, c.GetCurrentUid(), req.Id))
}
