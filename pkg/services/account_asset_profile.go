package services

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/shopspring/decimal"
	"xorm.io/xorm"
)

var assetAmountPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]{0,12})(\.[0-9]{1,2})?$`)
var assetIconPattern = regexp.MustCompile(`^[1-9][0-9]{0,18}$`)
var assetColorPattern = regexp.MustCompile(`^[0-9a-fA-F]{6}$`)

func validateAccountAssetProfile(sess *xorm.Session, a *models.Account) error {
	var old models.Account
	has, err := sess.ID(a.AccountId).Where("uid=? AND deleted=?", a.Uid, false).Get(&old)
	if err != nil {
		return err
	}
	if has && (old.IsReimbursement() != a.IsReimbursement() || (old.IsReimbursement() && old.Currency != a.Currency)) {
		return investmentError("报销账户的类型和币种不能更改，请新建对应账户")
	}

	if has {
		oldDay, newDay := 0, 0
		if old.Extend != nil && old.Extend.CreditCardStatementDate != nil {
			oldDay = *old.Extend.CreditCardStatementDate
		}
		if a.Extend != nil && a.Extend.CreditCardStatementDate != nil {
			newDay = *a.Extend.CreditCardStatementDate
		}
		oldNext, newNext := false, false
		if old.AssetProfile() != nil {
			oldNext = old.AssetProfile().StatementNextCycle
		}
		if a.AssetProfile() != nil {
			newNext = a.AssetProfile().StatementNextCycle
		}
		if old.Category != a.Category || oldDay != newDay || oldNext != newNext {
			used, e := sess.Where("uid=? AND account_id=? AND closed=?", a.Uid, a.AccountId, false).Exist(&models.CreditInstallment{})
			if e != nil {
				return e
			}
			if used {
				return investmentError("账户仍有分期，请先结束分期再更改类型或账单周期")
			}
		}
	}
	p := a.AssetProfile()
	if p == nil {
		return nil
	}
	if p.Kind != "" && p.Kind != "prepaid" && p.Kind != "secondhand" && p.Kind != "insurance" && p.Kind != "reimbursement" {
		return investmentError("账户类型无效")
	}
	if utf8.RuneCountInString(p.Group) > 40 || utf8.RuneCountInString(p.ShortName) > 24 || utf8.RuneCountInString(p.CardNumber) > 64 || strings.ContainsAny(p.CardNumber, "\r\n") || (p.NightIcon != "" && !assetIconPattern.MatchString(p.NightIcon)) || p.NightIconType < 0 || p.NightIconType > 1 || (p.NightColor != "" && !assetColorPattern.MatchString(p.NightColor)) {
		return investmentError("请检查账户分组、简称、卡号和图标")
	}
	if p.RepaymentDay < 0 || p.RepaymentDay > 31 || p.RepaymentAfterDays < 0 || p.RepaymentAfterDays > 60 || (p.RepaymentDay > 0 && p.RepaymentAfterDays > 0) || p.AnnualWaiverCount < 0 || p.AnnualWaiverCount > 10000 {
		return investmentError("请检查还款日和年费减免条件")
	}
	for _, value := range []string{p.AnnualFee, p.AnnualWaiverAmount} {
		if value == "" {
			continue
		}
		if len(value) > 20 || !assetAmountPattern.MatchString(value) {
			return investmentError("金额最多两位小数")
		}
		d, err := decimal.NewFromString(value)
		if err != nil || d.IsNegative() {
			return investmentError("金额不能为负数")
		}
	}
	if (p.AnnualFeeDate != "" && !validCalendarDate(p.AnnualFeeDate)) || (p.DebtDueDate != "" && !validCalendarDate(p.DebtDueDate)) {
		return investmentError("年费或借还款日期无效")
	}
	if p.Kind == "reimbursement" && (a.Category != models.ACCOUNT_CATEGORY_RECEIVABLES || a.Type != models.ACCOUNT_TYPE_SINGLE_ACCOUNT) {
		return investmentError("报销账户须使用独立的应收账户")
	}
	if p.SharedLimitAccount != "" {
		id, err := strconv.ParseInt(p.SharedLimitAccount, 10, 64)
		if err != nil || id <= 0 || id == a.AccountId || a.Category != models.ACCOUNT_CATEGORY_CREDIT_CARD {
			return investmentError("请选择有效的共享额度信用卡")
		}
		var other models.Account
		has, err := sess.ID(id).Where("uid=? AND deleted=? AND system_role=?", a.Uid, false, "").Get(&other)
		if err != nil {
			return err
		}
		if !has || other.Category != models.ACCOUNT_CATEGORY_CREDIT_CARD || other.Currency != a.Currency || (other.AssetProfile() != nil && other.AssetProfile().SharedLimitAccount != "") {
			return investmentError("共享额度账户必须是同币种的主信用卡")
		}
	}
	return nil
}

func guardReimbursementAccounts(sess *xorm.Session, uid int64, ids []int64) error {
	used, err := sess.Where("uid=? AND deleted=?", uid, false).In("reimbursement_account_id", ids).Exist(&models.Transaction{})
	if err != nil {
		return err
	}
	if used {
		return investmentError("账户有关联报销，请从资产页面查看并处理关联账单")
	}
	return nil
}
