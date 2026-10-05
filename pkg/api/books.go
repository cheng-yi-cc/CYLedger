package api

import (
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

type BooksApi struct{}

var Books = &BooksApi{}

func (a *BooksApi) List(c *core.WebContext) (any, *errs.Error) {
	return investmentResponse(services.Books.List(c, c.GetCurrentUid()))
}
func (a *BooksApi) Create(c *core.WebContext) (any, *errs.Error) {
	var req models.BookCreateRequest
	if err := investmentBind(c, &req); err != nil {
		return nil, err
	}
	return investmentResponse(services.Books.Create(c, c.GetCurrentUid(), req))
}
func (a *BooksApi) Modify(c *core.WebContext) (any, *errs.Error) {
	var req models.BookModifyRequest
	if err := investmentBind(c, &req); err != nil {
		return nil, err
	}
	return investmentResponse(services.Books.Modify(c, c.GetCurrentUid(), req))
}
func (a *BooksApi) Move(c *core.WebContext) (any, *errs.Error) {
	var req models.BookMoveRequest
	if err := investmentBind(c, &req); err != nil {
		return nil, err
	}
	ids, err := utils.StringArrayToInt64Array(req.TransactionIds)
	if err != nil {
		return nil, errs.ErrTransactionIdInvalid
	}
	uid := c.GetCurrentUid()
	user, err := services.Users.GetUserById(c, uid)
	if err != nil {
		return nil, errs.ErrUserNotFound
	}
	zone, err := c.GetClientTimezone()
	if err != nil {
		return nil, errs.ErrClientTimezoneOffsetInvalid
	}
	transactions, err := services.Transactions.GetTransactionsByTransactionIds(c, uid, ids)
	if err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}
	accounts, err := services.Accounts.GetAllAccountsByUid(c, uid)
	if err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}
	accountMap := services.Accounts.GetAccountMapByList(accounts)
	for _, transaction := range transactions {
		if !user.CanEditTransactionByTransactionTime(transaction.TransactionTime, zone, accountMap[transaction.AccountId], accountMap[transaction.RelatedAccountId]) {
			return nil, errs.ErrCannotModifyTransactionWithThisTransactionTime
		}
	}
	return investmentResponse(true, services.Books.MoveTransactions(c, uid, ids, req.BookId))
}
