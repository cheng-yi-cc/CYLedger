package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/shopspring/decimal"
	"strconv"
	"time"
	"xorm.io/xorm"
)

type AssetAdjustmentInput struct {
	AccountId         string `json:"accountId"`
	Balance           string `json:"balance"`
	ExpectedBalance   string `json:"expectedBalance"`
	BookId            string `json:"bookId"`
	CountInStatistics bool   `json:"countInStatistics"`
	TimeZone          string `json:"timeZone"`
	RequestId         string `json:"requestId"`
}

func (s *InvestmentService) AdjustAssetBalance(c core.Context, uid int64, input AssetAdjustmentInput) (*models.AssetAdjustment, error) {
	id, err := strconv.ParseInt(input.AccountId, 10, 64)
	zone, zoneErr := time.LoadLocation(input.TimeZone)
	if err != nil || uid <= 0 || id <= 0 || zoneErr != nil || input.TimeZone == "" || len(input.TimeZone) > 64 || len(input.RequestId) < 8 || len(input.RequestId) > 64 || len(input.BookId) > 64 || !assetAmountPattern.MatchString(input.Balance) || !assetAmountPattern.MatchString(input.ExpectedBalance) {
		return nil, investmentError("请检查余额、账户和时区")
	}
	target, _ := decimal.NewFromString(input.Balance)
	expected, _ := decimal.NewFromString(input.ExpectedBalance)
	data, _ := json.Marshal(input)
	hash := sha256.Sum256(data)
	digest := hex.EncodeToString(hash[:])
	row := &models.AssetAdjustment{Id: fmt.Sprintf("%d:%s", uid, input.RequestId), Uid: uid, Digest: digest}
	defer s.lock(uid)()
	err = s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		var existing models.AssetAdjustment
		has, e := sess.ID(row.Id).Where("uid=?", uid).Get(&existing)
		if e != nil {
			return e
		}
		if has {
			if existing.Digest != digest {
				return investmentError("请求内容已变化，请重试")
			}
			*row = existing
			return nil
		}
		var a models.Account
		has, e = sess.ID(id).Where("uid=? AND deleted=? AND hidden=? AND system_role=?", uid, false, false, "").Get(&a)
		if e != nil {
			return e
		}
		if !has || a.Type != models.ACCOUNT_TYPE_SINGLE_ACCOUNT || a.IsReimbursement() {
			return investmentError("该账户不能直接校准余额")
		}
		if a.Balance != expected.Shift(2).IntPart() {
			return investmentError("账户余额已变化，请刷新后重新核对")
		}
		delta := target.Sub(expected)
		if delta.IsZero() {
			_, e = sess.Insert(row)
			return e
		}
		if delta.Abs().Shift(2).GreaterThan(decimal.NewFromInt(models.MaximumTransactionAmount)) {
			return investmentError("调整差额超出支持范围")
		}
		book, e := Books.ResolveInSession(sess, uid, input.BookId, false)
		if e != nil {
			return e
		}
		kind := models.CATEGORY_TYPE_INCOME
		txType := models.TRANSACTION_DB_TYPE_INCOME
		if delta.IsNegative() {
			kind = models.CATEGORY_TYPE_EXPENSE
			txType = models.TRANSACTION_DB_TYPE_EXPENSE
		}
		category, e := assetIncomeCategory(sess, uid, book, kind, "余额校准")
		if e != nil {
			return e
		}
		now := time.Now()
		_, offset := now.In(zone).Zone()
		tx := &models.Transaction{Uid: uid, BookId: book, AccountId: id, Type: txType, CategoryId: category, Amount: delta.Abs().Shift(2).IntPart(), TransactionTime: utils.GetMinTransactionTimeFromUnixTime(now.Unix()), TimezoneUtcOffset: int16(offset / 60), Comment: "余额校准", ExcludeFromStatistics: !input.CountInStatistics}
		if e = Transactions.createTransactionInSession(c, sess, tx, nil, nil); e != nil {
			return e
		}
		row.TransactionId = tx.TransactionId
		_, e = sess.Insert(row)
		return e
	})
	return row, err
}
