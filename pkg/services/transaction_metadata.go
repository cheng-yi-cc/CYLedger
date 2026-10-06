package services

import (
	"fmt"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/shopspring/decimal"
	"unicode/utf8"
	"xorm.io/xorm"
)

// Due dates are reminders linked to a borrowing/lending fact, not another balance.
func syncTransactionMetadata(sess *xorm.Session, tx *models.Transaction) error {
	if utf8.RuneCountInString(tx.LocationName) > 200 {
		return investmentError("地点名称最多 200 字")
	}
	if tx.DebtDueDate == "" {
		return nil
	}
	if tx.Type != models.TRANSACTION_DB_TYPE_TRANSFER_OUT || !validCalendarDate(tx.DebtDueDate) {
		return investmentError("请为借入或借出填写有效的约定还款日期")
	}
	var source, destination models.Account
	if ok, err := sess.ID(tx.AccountId).Where("uid=? AND deleted=?", tx.Uid, false).Get(&source); err != nil {
		return err
	} else if !ok {
		return ErrCalendarInvalid
	}
	if ok, err := sess.ID(tx.RelatedAccountId).Where("uid=? AND deleted=?", tx.Uid, false).Get(&destination); err != nil {
		return err
	} else if !ok {
		return ErrCalendarInvalid
	}
	account, amount := &source, tx.Amount
	if source.Category != models.ACCOUNT_CATEGORY_DEBT {
		if destination.Category != models.ACCOUNT_CATEGORY_RECEIVABLES {
			return ErrCalendarInvalid
		}
		account = &destination
		amount = tx.RelatedAccountAmount
	}
	if amount <= 0 {
		return ErrCalendarInvalid
	}
	id := fmt.Sprintf("debt-transaction-%d", tx.TransactionId)
	var current models.CalendarEvent
	has, err := sess.ID(id).Where("uid=?", tx.Uid).Get(&current)
	if err != nil {
		return err
	}
	item := models.CalendarEvent{Id: id, Uid: tx.Uid, BookId: tx.BookId, TransactionId: tx.TransactionId, AccountId: fmt.Sprint(account.AccountId), AccountName: account.Name, Currency: account.Currency, Kind: "repayment", Date: tx.DebtDueDate, Amount: decimal.New(amount, -2).StringFixed(2), Note: "借款约定到期"}
	if has {
		item.Completed = current.Completed && current.Date == item.Date
		_, err = sess.ID(id).Where("uid=?", tx.Uid).AllCols().Update(&item)
	} else {
		_, err = sess.Insert(&item)
	}
	return err
}
