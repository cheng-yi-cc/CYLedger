package services

import (
	"fmt"
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"xorm.io/xorm"
)

func accountCurrencyEditable(sess *xorm.Session, a *models.Account) (bool, error) {
	if a.SystemRole != "" || a.Deleted || a.Type != models.ACCOUNT_TYPE_SINGLE_ACCOUNT || a.ParentAccountId != 0 || a.Balance != 0 || a.IsReimbursement() {
		return false, nil
	}
	if a.Extend != nil && a.Extend.CreditCardLimit != nil && *a.Extend.CreditCardLimit != 0 {
		return false, nil
	}
	if p := a.AssetProfile(); p != nil && (p.SharedLimitAccount != "" || p.AnnualFee != "" && p.AnnualFee != "0" || p.AnnualWaiverAmount != "" && p.AnnualWaiverAmount != "0") {
		return false, nil
	}
	if a.Category == models.ACCOUNT_CATEGORY_CREDIT_CARD {
		var cards []models.Account
		if err := sess.Where("uid=? AND deleted=? AND category=?", a.Uid, false, models.ACCOUNT_CATEGORY_CREDIT_CARD).Find(&cards); err != nil {
			return false, err
		}
		for _, card := range cards {
			if p := card.AssetProfile(); p != nil && p.SharedLimitAccount == fmt.Sprint(a.AccountId) {
				return false, nil
			}
		}
	}
	// Retain deleted history too: restoring an old fact must never reinterpret
	// its amount in a different unit. Templates/bindings are also denominated.
	has, err := sess.Where("uid=? AND (account_id=? OR related_account_id=?)", a.Uid, a.AccountId, a.AccountId).Exist(&models.Transaction{})
	if err != nil || has {
		return false, err
	}
	has, err = sess.Where("uid=? AND deleted=? AND (account_id=? OR related_account_id=?)", a.Uid, false, a.AccountId, a.AccountId).Exist(&models.TransactionTemplate{})
	if err != nil || has {
		return false, err
	}
	has, err = sess.Where("uid=? AND account_id=?", a.Uid, a.AccountId).Exist(&models.MonetaryIncomeBinding{})
	if err != nil || has {
		return false, err
	}
	has, err = sess.Where("uid=? AND account_id=?", a.Uid, fmt.Sprint(a.AccountId)).Exist(&models.FixedDeposit{})
	if err != nil || has {
		return false, err
	}
	return true, nil
}

func (s *AccountService) CurrencyEditable(c core.Context, a *models.Account) (bool, error) {
	sess := s.UserDataDB(a.Uid).NewSession(c)
	defer sess.Close()
	return accountCurrencyEditable(sess, a)
}
