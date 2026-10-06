package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"strconv"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"xorm.io/xorm"
)

type AccountDeletionInput struct {
	ID            string `json:"id" binding:"required,max=64"`
	Kind          string `json:"kind" binding:"required,oneof=cash portfolio"`
	Token         string `json:"token" binding:"max=64"`
	DeleteRelated bool   `json:"deleteRelated"`
}

type AccountDeletionPreview struct {
	Name              string   `json:"name"`
	TransactionCount  int      `json:"transactionCount"`
	InvestmentCount   int      `json:"investmentCount"`
	TemplateCount     int      `json:"templateCount"`
	DueCount          int      `json:"dueCount"`
	DepositCount      int      `json:"depositCount"`
	InstallmentCount  int      `json:"installmentCount"`
	SubAccountCount   int      `json:"subAccountCount"`
	ParentAccountName string   `json:"parentAccountName"`
	AffectedAccounts  []string `json:"affectedAccounts"`
	BlockedReason     string   `json:"blockedReason"`
	Token             string   `json:"token"`
}

// DeleteAssetAccount previews and commits the same plan. Every balance, fact,
// attachment index and account change shares one SQLite transaction.
func (s *InvestmentService) DeleteAssetAccount(c core.Context, uid int64, input AccountDeletionInput, preview bool) (*AccountDeletionPreview, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	if len(input.ID) == 0 || len(input.ID) > 64 || (input.Kind != "cash" && input.Kind != "portfolio") {
		return nil, errs.ErrAccountIdInvalid
	}
	defer s.lock(uid)()
	var result *AccountDeletionPreview
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		// Acquire SQLite's write lock before reading the facts, as Mutate does.
		settings := new(models.InvestmentSettings)
		has, err := sess.Where("uid=?", uid).Get(settings)
		if err != nil {
			return err
		}
		if !has {
			if _, err := sess.Insert(&models.InvestmentSettings{Uid: uid, BaseCurrency: "CNY", TimeZone: "Asia/Shanghai"}); err != nil {
				return err
			}
		}
		if _, err := sess.Where("uid=?", uid).Incr("revision", 1).Update(&models.InvestmentSettings{}); err != nil {
			return err
		}
		result = &AccountDeletionPreview{AffectedAccounts: []string{}}
		var cash []models.Account
		if err := sess.Where("uid=? AND deleted=?", uid, false).Asc("account_id").Find(&cash); err != nil {
			return err
		}
		targets := map[int64]bool{}
		var portfolio models.PortfolioAccount
		if input.Kind == "cash" {
			id, err := strconv.ParseInt(input.ID, 10, 64)
			if err != nil || id <= 0 {
				return errs.ErrAccountIdInvalid
			}
			for _, a := range cash {
				if a.AccountId == id {
					result.Name = a.Name
					targets[id] = true
				}
			}
			if len(targets) == 0 {
				return errs.ErrAccountNotFound
			}
			for _, a := range cash {
				if a.ParentAccountId == id {
					targets[a.AccountId] = true
					result.SubAccountCount++
				}
			}
			// Removing the final child also removes its now-empty parent. Never
			// leave a multi-currency account without a selectable child.
			for _, selected := range cash {
				if selected.AccountId != id || selected.ParentAccountId == 0 {
					continue
				}
				siblings := 0
				for _, a := range cash {
					if a.ParentAccountId == selected.ParentAccountId {
						siblings++
					}
				}
				if siblings == 1 {
					for _, a := range cash {
						if a.AccountId == selected.ParentAccountId {
							targets[a.AccountId] = true
							result.ParentAccountName = a.Name
						}
					}
				}
			}
			for _, a := range cash {
				if targets[a.AccountId] && a.SystemRole != "" {
					return ErrInvestmentLinked
				}
			}
		} else {
			has, err := sess.Where("uid=? AND id=?", uid, input.ID).Get(&portfolio)
			if err != nil {
				return err
			}
			if !has {
				return errs.ErrAccountNotFound
			}
			result.Name = portfolio.Name
		}
		events, err := readInvestmentEvents(sess, uid)
		if err != nil {
			return err
		}
		voidIDs := map[string]bool{}
		for i, e := range events {
			cashID, _ := strconv.ParseInt(e.CashAccountID, 10, 64)
			linked := input.Kind == "cash" && targets[cashID] || input.Kind == "portfolio" && (e.AccountID == input.ID || e.ToAccountID == input.ID || e.SettlementAccountID == input.ID)
			if !e.Voided && linked {
				voidIDs[e.ID] = true
				events[i].Voided = true
				result.InvestmentCount++
			}
		}
		var allTx []models.Transaction
		if err := sess.Where("uid=? AND deleted=?", uid, false).Asc("transaction_id").Find(&allTx); err != nil {
			return err
		}
		selected := map[int64]bool{}
		for _, tx := range allTx {
			if targets[tx.AccountId] || targets[tx.RelatedAccountId] || targets[tx.ReimbursementAccountId] || voidIDs[tx.InvestmentEventId] {
				selected[tx.TransactionId] = true
			}
		}
		var receipts []models.ReimbursementReceipt
		if err := sess.Where("uid=?", uid).Find(&receipts); err != nil {
			return err
		}
		for _, receipt := range receipts {
			if selected[receipt.ExpenseId] {
				selected[receipt.IncomeId] = true
			}
		}

		var debtMovements []models.DebtMovement
		if err := sess.Where("uid=?", uid).Find(&debtMovements); err != nil {
			return err
		}
		for _, movement := range debtMovements {
			if targets[movement.DebtAccountId] || selected[movement.PrincipalTransactionId] {
				selected[movement.InterestTransactionId] = true
			}
		}
		for _, tx := range allTx {
			if tx.TransferFeeParentId > 0 && selected[tx.TransferFeeParentId] {
				selected[tx.TransactionId] = true
			}
		}
		var txs []models.Transaction
		for _, tx := range allTx {
			if selected[tx.TransactionId] {
				txs = append(txs, tx)
				if tx.InvestmentEventId != "" && !voidIDs[tx.InvestmentEventId] {
					return investmentError("关联投资结算缺少对应记录，请先处理关联交易")
				}
				if tx.InvestmentEventId == "" && tx.Type != models.TRANSACTION_DB_TYPE_TRANSFER_IN {
					result.TransactionCount++
				}
			}
		}
		byID := map[int64]models.Transaction{}
		for _, tx := range txs {
			byID[tx.TransactionId] = tx
		}
		for _, tx := range txs {
			if tx.Type != models.TRANSACTION_DB_TYPE_TRANSFER_IN && tx.Type != models.TRANSACTION_DB_TYPE_TRANSFER_OUT {
				continue
			}
			paired, exists := byID[tx.RelatedId]
			oppositeType := tx.Type == models.TRANSACTION_DB_TYPE_TRANSFER_IN && paired.Type == models.TRANSACTION_DB_TYPE_TRANSFER_OUT || tx.Type == models.TRANSACTION_DB_TYPE_TRANSFER_OUT && paired.Type == models.TRANSACTION_DB_TYPE_TRANSFER_IN
			if !exists || !oppositeType || paired.RelatedId != tx.TransactionId || paired.AccountId != tx.RelatedAccountId || paired.RelatedAccountId != tx.AccountId || paired.Amount != tx.RelatedAccountAmount || paired.RelatedAccountAmount != tx.Amount || paired.InvestmentEventId != tx.InvestmentEventId {
				return investmentError("关联转账两侧不一致，请先核对关联交易")
			}
		}
		var templates []models.TransactionTemplate
		if len(targets) > 0 {
			var all []models.TransactionTemplate
			if err := sess.Where("uid=? AND deleted=?", uid, false).Asc("template_id").Find(&all); err != nil {
				return err
			}
			for _, item := range all {
				if targets[item.AccountId] || targets[item.RelatedAccountId] {
					templates = append(templates, item)
				}
			}
		}
		result.TemplateCount = len(templates)
		var dueItems []models.CalendarEvent
		var depositItems []models.FixedDeposit
		// Both tables are part of the required schema. Opening another database
		// session here would deadlock a single-connection SQLite transaction.
		hasDueTable, hasDepositTable := true, true
		if len(targets) > 0 {
			var ids []string
			for id := range targets {
				ids = append(ids, strconv.FormatInt(id, 10))
			}
			if hasDueTable {
				if err := sess.Where("uid=?", uid).In("account_id", ids).Asc("id").Find(&dueItems); err != nil {
					return err
				}
			}
			if hasDepositTable {
				if err := sess.Where("uid=? AND closed=?", uid, false).In("account_id", ids).Asc("id").Find(&depositItems); err != nil {
					return err
				}
			}
		}
		result.DueCount = len(dueItems)
		result.DepositCount = len(depositItems)
		var plans []models.CreditInstallment
		var installmentItems []models.CreditInstallment
		if err := sess.Where("uid=? AND closed=?", uid, false).Asc("id").Find(&plans); err != nil {
			return err
		}
		for _, plan := range plans {
			if targets[plan.AccountId] || selected[plan.ExpenseId] {
				installmentItems = append(installmentItems, plan)
			}
		}
		result.InstallmentCount = len(installmentItems)

		var portfolios []models.PortfolioAccount
		if err := sess.Where("uid=?", uid).Asc("id").Find(&portfolios); err != nil {
			return err
		}
		for _, account := range portfolios {
			if input.Kind == "portfolio" && account.Id == input.ID {
				continue
			}
			for _, e := range events {
				if voidIDs[e.ID] && (e.AccountID == account.Id || e.ToAccountID == account.Id || e.SettlementAccountID == account.Id) {
					result.AffectedAccounts = append(result.AffectedAccounts, account.Name)
					break
				}
			}
		}
		// Fingerprint the complete investment history and current balances as well:
		// another account's sale can change whether this deletion is valid.
		raw, err := json.Marshal([]any{input.ID, input.Kind, cash, portfolio, portfolios, events, txs, templates, dueItems, depositItems, installmentItems})
		if err != nil {
			return err
		}
		hash := sha256.Sum256(raw)
		result.Token = hex.EncodeToString(hash[:])
		if !preview && input.Token != result.Token {
			return investmentError("账户或关联记录已变化，请重新查看删除影响后确认")
		}
		if _, err := replayInvestments(events); err != nil {
			result.BlockedReason = "删除会使其他账户的投资持仓无法重算，请先处理关联投资记录：" + err.Error()
		}
		balances := map[int64]*big.Int{}
		for _, tx := range txs {
			if balances[tx.AccountId] == nil {
				balances[tx.AccountId] = new(big.Int)
			}
			delta := big.NewInt(tx.Amount)
			switch tx.Type {
			case models.TRANSACTION_DB_TYPE_MODIFY_BALANCE:
				delta.SetInt64(tx.RelatedAccountAmount)
				delta.Neg(delta)
			case models.TRANSACTION_DB_TYPE_INCOME, models.TRANSACTION_DB_TYPE_TRANSFER_IN:
				delta.Neg(delta)
			case models.TRANSACTION_DB_TYPE_EXPENSE, models.TRANSACTION_DB_TYPE_TRANSFER_OUT:
			default:
				return errs.ErrTransactionTypeInvalid
			}
			balances[tx.AccountId].Add(balances[tx.AccountId], delta)
		}
		for _, a := range cash {
			if delta := balances[a.AccountId]; delta != nil {
				delta.Add(delta, big.NewInt(a.Balance))
				if !targets[a.AccountId] && !delta.IsInt64() {
					result.BlockedReason = "删除会使关联账户余额超出范围，请先处理关联交易"
				}
				if !targets[a.AccountId] && a.SystemRole == "" {
					result.AffectedAccounts = append(result.AffectedAccounts, a.Name)
				}
			}
		}
		if preview {
			return errInvestmentPreviewRollback
		}
		if result.BlockedReason != "" {
			return investmentError(result.BlockedReason)
		}
		if !input.DeleteRelated && result.TransactionCount+result.InvestmentCount+result.TemplateCount+result.DueCount+result.DepositCount+result.InstallmentCount > 0 {
			return investmentError("账户还有关联交易或模板，请先处理或确认一并删除")
		}
		if err := s.voidAccountInvestments(sess, uid, events, voidIDs); err != nil {
			return err
		}
		now := time.Now().Unix()
		// Apply the net reversal once per surviving account, avoiding intermediate
		// overflow and handling both halves of cross-currency transfers exactly.
		for _, a := range cash {
			if targets[a.AccountId] {
				continue
			}
			if balance := balances[a.AccountId]; balance != nil {
				if _, err := sess.Where("uid=? AND account_id=? AND deleted=?", uid, a.AccountId, false).Cols("balance", "updated_unix_time").Update(&models.Account{Balance: balance.Int64(), UpdatedUnixTime: now}); err != nil {
					return err
				}
			}
		}
		// Small batches keep SQLite's parameter limit independent of ledger size.
		for start := 0; start < len(txs); start += 200 {
			end := start + 200
			if end > len(txs) {
				end = len(txs)
			}
			ids := make([]int64, 0, end-start)
			for _, tx := range txs[start:end] {
				ids = append(ids, tx.TransactionId)
			}
			if _, err := sess.Where("uid=?", uid).In("transaction_id", ids).Cols("deleted", "deleted_unix_time").Update(&models.Transaction{Deleted: true, DeletedUnixTime: now}); err != nil {
				return err
			}
			if hasDueTable {
				if _, err := sess.Where("uid=?", uid).In("transaction_id", ids).Delete(&models.CalendarEvent{}); err != nil {
					return err
				}
			}
			if _, err := sess.Where("uid=?", uid).In("transaction_id", ids).Cols("deleted", "deleted_unix_time").Update(&models.TransactionTagIndex{Deleted: true, DeletedUnixTime: now}); err != nil {
				return err
			}
			if _, err := sess.Where("uid=?", uid).In("transaction_id", ids).Cols("deleted", "deleted_unix_time").Update(&models.TransactionPictureInfo{Deleted: true, DeletedUnixTime: now}); err != nil {
				return err
			}
		}
		for _, item := range templates {
			if _, err := sess.Where("uid=? AND template_id=?", uid, item.TemplateId).Cols("deleted", "deleted_unix_time").Update(&models.TransactionTemplate{Deleted: true, DeletedUnixTime: now}); err != nil {
				return err
			}
		}
		if input.Kind == "portfolio" {
			if _, err := sess.Where("uid=? AND id=?", uid, input.ID).Delete(&models.PortfolioAccount{}); err != nil {
				return err
			}
		} else {
			for id := range targets {
				if hasDueTable {
					if _, err := sess.Where("uid=? AND account_id=?", uid, strconv.FormatInt(id, 10)).Delete(&models.CalendarEvent{}); err != nil {
						return err
					}
				}
				if hasDepositTable {
					if _, err := sess.Where("uid=? AND account_id=?", uid, strconv.FormatInt(id, 10)).Cols("closed").Update(&models.FixedDeposit{Closed: true}); err != nil {
						return err
					}
				}
				if err := pauseMonetaryIncome(sess, uid, []int64{id}); err != nil {
					return err
				}
				if _, err := sess.Where("uid=? AND account_id=?", uid, id).Cols("balance", "deleted", "deleted_unix_time").Update(&models.Account{Balance: 0, Deleted: true, DeletedUnixTime: now}); err != nil {
					return err
				}
			}
		}

		for _, plan := range installmentItems {
			if _, err := sess.ID(plan.Id).Where("uid=?", uid).Cols("closed").Update(&models.CreditInstallment{Closed: true}); err != nil {
				return err
			}
		}
		if err := refreshReimbursementBalances(sess, uid); err != nil {
			return err
		}
		return InvalidateWealthSnapshots(sess, uid, 0)
	})
	if err == errInvestmentPreviewRollback {
		return result, nil
	}
	return result, err
}
