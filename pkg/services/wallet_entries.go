package services

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/investments"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
	"github.com/shopspring/decimal"
	"xorm.io/xorm"
)

// WalletEntry stores the original payment unit and category alongside the
// investment fact. The ordinary ledger contains one linked CNY posting for
// reporting; that system posting is excluded from all asset balances.
type WalletEntry struct {
	Currency   string `json:"currency"`
	CategoryID string `json:"categoryId"`
	FXDate     string `json:"fxDate"`
	FXSource   string `json:"fxSource"`
}

func (s *InvestmentService) validateWalletEntry(sess *xorm.Session, uid int64, e *InvestmentEvent) error {
	w := e.Wallet
	if w == nil || (w.Currency != "CNY" && w.Currency != "USD") || len(w.CategoryID) > 20 || len(w.FXSource) > 100 || len(w.FXDate) > 10 || utf8.RuneCountInString(e.Note) > 255 || e.CashAccountID != "" || e.Conversion != nil || e.OccurredAt <= 0 || e.OccurredAt > time.Now().Unix()+60 {
		return investmentError("请核对钱包收支的币种、日期和备注")
	}
	if _, err := time.Parse("2006-01-02", w.FXDate); err != nil || w.FXSource == "" {
		return investmentError("请确认本笔收支的人民币汇率及日期")
	}
	if w.Currency == "CNY" && e.ExchangeRate != "1" {
		return investmentError("人民币记账汇率必须为 1")
	}
	for _, value := range []string{e.Amount, e.ExchangeRate} {
		if err := investments.ValidateDecimal(value); err != nil {
			return investmentError("金额和汇率必须是有效十进制字符串")
		}
	}
	amount, _ := decimal.NewFromString(e.Amount)
	rate, _ := decimal.NewFromString(e.ExchangeRate)
	minor := amount.Mul(rate).Round(2).Shift(2)
	if !amount.IsPositive() || amount.Exponent() < -2 || !rate.IsPositive() || !minor.IsPositive() || minor.GreaterThan(decimal.NewFromInt(models.MaximumTransactionAmount)) {
		return investmentError("金额或折算金额超出范围，记账金额最多两位小数")
	}
	var account models.PortfolioAccount
	found, err := sess.Where("uid=? AND id=?", uid, e.AccountID).Get(&account)
	if err != nil {
		return err
	}
	if !found || (account.Kind != "WALLET" && account.Kind != "EXCHANGE") {
		return investmentError("请选择自己的加密钱包或交易所账户")
	}
	prefs, err := readAssetPreferences(sess, uid)
	if err != nil {
		return err
	}
	for _, bookID := range prefs.Rules["portfolio:"+e.AccountID].DisabledBooks {
		if bookID == e.BookID {
			return investmentError("此账户在所选账本中未启用")
		}
	}
	movements := append([]investments.AssetMovement{{InstrumentID: e.InstrumentID, Quantity: e.Quantity}}, e.AdditionalMovements...)
	if len(movements) > 2 {
		return investmentError("一笔钱包收支最多使用两种币")
	}
	for _, movement := range movements {
		if len(movement.InstrumentID) > 100 {
			return investmentError("币种无效")
		}
		known := false
		for _, preset := range investmentPresets {
			if preset.Id == movement.InstrumentID && preset.Type == "CRYPTO" {
				known = true
			}
		}
		if !known {
			known, err = sess.Where("uid=? AND id=? AND type=?", uid, movement.InstrumentID, "CRYPTO").Exist(&models.InvestmentInstrument{})
			if err != nil {
				return err
			}
		}
		if !known {
			return investmentError("请选择当前账本中的加密货币")
		}
	}
	category, err := strconv.ParseInt(w.CategoryID, 10, 64)
	if err != nil || category <= 0 {
		return investmentError("请选择收支分类")
	}
	tx := &models.Transaction{Uid: uid, CategoryId: category, Type: models.TRANSACTION_DB_TYPE_EXPENSE}
	if e.Type == investments.Income {
		tx.Type = models.TRANSACTION_DB_TYPE_INCOME
	}
	// Linked investment postings normally bypass category validation; wallet
	// consumption is ordinary income/expense and must validate it explicitly.
	return Transactions.isCategoryValid(sess, tx)
}

func (s *InvestmentService) settleWallet(c core.Context, sess *xorm.Session, uid int64, e InvestmentEvent, zone string) error {
	system := new(models.Account)
	has, err := sess.Where("uid=? AND currency=? AND system_role=? AND deleted=?", uid, "CNY", InvestmentSettlementRole, false).Get(system)
	if err != nil {
		return err
	}
	if !has {
		system = &models.Account{AccountId: Accounts.GenerateUuid(uuid.UUID_TYPE_ACCOUNT), Uid: uid, Category: models.ACCOUNT_CATEGORY_VIRTUAL, Type: models.ACCOUNT_TYPE_SINGLE_ACCOUNT, Name: "投资结算（系统）", Currency: "CNY", SystemRole: InvestmentSettlementRole, Icon: 1, Color: "387F79", CreatedUnixTime: time.Now().Unix()}
		if system.AccountId <= 0 {
			return errs.ErrSystemIsBusy
		}
		if _, err = sess.Insert(system); err != nil {
			return err
		}
	}
	amount, _ := decimal.NewFromString(e.Amount)
	rate, _ := decimal.NewFromString(e.ExchangeRate)
	category, _ := strconv.ParseInt(e.Wallet.CategoryID, 10, 64)
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return err
	}
	_, offset := time.Unix(e.OccurredAt, 0).In(loc).Zone()
	kind := models.TRANSACTION_DB_TYPE_EXPENSE
	if e.Type == investments.Income {
		kind = models.TRANSACTION_DB_TYPE_INCOME
	}
	tx := &models.Transaction{Uid: uid, BookId: e.BookID, Type: kind, AccountId: system.AccountId, Amount: amount.Mul(rate).Round(2).Shift(2).IntPart(), CategoryId: category, TransactionTime: utils.GetMinTransactionTimeFromUnixTime(e.OccurredAt), TimezoneUtcOffset: int16(offset / 60), Comment: e.Note, InvestmentEventId: e.ID}
	if err = Transactions.createTransactionInSession(c, sess, tx, nil, nil); err != nil {
		return err
	}
	if _, err = sess.Insert(&models.InvestmentTransactionLink{Uid: uid, EventId: e.ID, TransactionId: tx.TransactionId}); err != nil {
		return err
	}
	return nil
}

func validatePortfolioCurrency(currency string) bool {
	return currency == "" || currency == "CNY" || currency == "USD"
}

func validatePaymentInstruments(ids []string, selected []string) error {
	if len(ids) > 2 {
		return investmentError("最多选择两个收支币种")
	}
	seen := map[string]bool{}
	allowed := map[string]bool{}
	for _, id := range selected {
		allowed[id] = true
	}
	for _, id := range ids {
		if !allowed[id] || seen[id] {
			return investmentError("收支币种须为账户内不重复的持有币种")
		}
		seen[id] = true
	}
	return nil
}

// Transaction response metadata is resolved from the immutable wallet event,
// never from today's account currency or FX. No second original-amount copy is
// stored in the ordinary transaction table.
func (s *InvestmentService) WalletTransactionDetails(c core.Context, uid int64, eventIDs []string) (map[string]*models.WalletTransactionInfo, error) {
	result := map[string]*models.WalletTransactionInfo{}
	if len(eventIDs) == 0 {
		return result, nil
	}
	var rows []models.InvestmentEventRecord
	sess := s.UserDataDB(uid).NewSession(c)
	defer sess.Close()
	if err := sess.Where("uid=? AND voided=?", uid, false).In("id", eventIDs).Find(&rows); err != nil {
		return nil, err
	}
	var accounts []models.PortfolioAccount
	if err := sess.Where("uid=?", uid).Find(&accounts); err != nil {
		return nil, err
	}
	names := map[string]string{}
	for _, a := range accounts {
		names[a.Id] = a.Name
	}
	for _, row := range rows {
		e, err := decodeWalletEvent(row.Payload)
		if err != nil {
			return nil, err
		}
		if e.Wallet == nil {
			continue
		}
		result[row.Id] = &models.WalletTransactionInfo{AccountID: e.AccountID, AccountName: names[e.AccountID], Currency: e.Wallet.Currency, Amount: e.Amount, ExchangeRate: e.ExchangeRate, FXDate: e.Wallet.FXDate}
	}
	return result, nil
}

// Keep decoding shared with the service event schema.
func decodeWalletEvent(payload string) (InvestmentEvent, error) {
	var e InvestmentEvent
	err := json.Unmarshal([]byte(payload), &e)
	if err != nil {
		return e, fmt.Errorf("invalid wallet event: %w", err)
	}
	return e, nil
}
