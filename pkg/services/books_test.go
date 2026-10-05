package services

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/investments"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/stretchr/testify/require"
)

func bookPtr[T any](v T) *T { return &v }

func addBookExpense(f *investmentDBFixture, bookID string, amount, offset int64) *models.Transaction {
	f.t.Helper()
	has, err := f.engine.ID(8101).Exist(&models.TransactionCategory{})
	require.NoError(f.t, err)
	if !has {
		_, err = f.engine.Insert(&models.TransactionCategory{CategoryId: 8100, Uid: f.uid, Type: models.CATEGORY_TYPE_EXPENSE, Name: "支出"}, &models.TransactionCategory{CategoryId: 8101, Uid: f.uid, Type: models.CATEGORY_TYPE_EXPENSE, ParentCategoryId: 8100, Name: "餐饮"})
		require.NoError(f.t, err)
	}
	tx := &models.Transaction{Uid: f.uid, BookId: bookID, AccountId: f.bankID, Type: models.TRANSACTION_DB_TYPE_EXPENSE, CategoryId: 8101, Amount: amount, TransactionTime: utils.GetMinTransactionTimeFromUnixTime(f.at + offset)}
	require.NoError(f.t, Transactions.CreateTransaction(nil, tx, nil, nil))
	return tx
}

func TestBooksSQLiteScopeSharedBalancesAndMove(t *testing.T) {
	f := newInvestmentDBFixture(t)
	second, err := Books.Create(nil, f.uid, models.BookCreateRequest{Name: "旅行"})
	require.NoError(t, err)
	firstExpense := addBookExpense(f, "", 100, 1)
	secondExpense := addBookExpense(f, second.Id, 230, 2)
	require.Equal(t, DefaultBookID(f.uid), firstExpense.BookId)
	require.Equal(t, int64(1999670), f.balance())
	ctx, err := Books.FilterContext(nil, f.uid, second.Id)
	require.NoError(t, err)
	list, err := Transactions.GetAllTransactions(ctx, f.uid, 50, true)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, secondExpense.TransactionId, list[0].TransactionId)
	count, err := Transactions.GetAllTransactionCount(ctx, f.uid)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
	_, expense, err := Transactions.GetAccountsTotalIncomeAndExpense(ctx, f.uid, f.at-120, f.at+120, nil, nil, time.UTC, false)
	require.NoError(t, err)
	require.Equal(t, "230", expense[f.bankID].String())
	_, daily, err := Transactions.GetAccountsDailyIncomeAndExpense(ctx, f.uid, f.at-120, f.at+120, nil, nil, time.UTC, false)
	require.NoError(t, err)
	day := utils.FormatUnixTimeToNumericYearMonthDay(f.at+2, time.UTC)
	require.Equal(t, "230", daily[day][f.bankID].String())
	total, err := Transactions.GetAccountsAndCategoriesTotalInflowAndOutflow(ctx, f.uid, f.at-120, f.at+120, nil, false, "", core.MATCH_MODE_DEFAULT, time.UTC, false)
	require.NoError(t, err)
	require.Len(t, total, 1)
	require.Equal(t, "230", total[0].Amount.String())
	year, month, _ := time.Unix(f.at, 0).UTC().Date()
	monthly, err := Transactions.GetAccountsAndCategoriesMonthlyInflowAndOutflow(ctx, f.uid, int32(year), int32(month), int32(year), int32(month), nil, false, "", core.MATCH_MODE_DEFAULT, time.UTC, false)
	require.NoError(t, err)
	require.Equal(t, "230", monthly[int32(year*100+int(month))][0].Amount.String())
	// Even a reporting-scoped caller must receive the real shared-account balance.
	_, _, _, _, closing, err := Transactions.GetAllTransactionsInOneAccountWithAccountBalanceByMaxTime(ctx, f.uid, 50, utils.GetMaxTransactionTimeFromUnixTime(f.at+120), 0, f.bankID, models.ACCOUNT_CATEGORY_CHECKING_ACCOUNT)
	require.NoError(t, err)
	require.Equal(t, "1999670", closing.String())
	require.NoError(t, Books.MoveTransactions(nil, f.uid, []int64{secondExpense.TransactionId}, firstExpense.BookId))
	require.Equal(t, int64(1999670), f.balance())
	count, err = Transactions.GetAllTransactionCount(ctx, f.uid)
	require.NoError(t, err)
	require.Zero(t, count)
	// Old clients omitting bookId preserve the existing attribution on edit.
	secondExpense.BookId = ""
	secondExpense.Comment = "补充备注"
	require.NoError(t, Transactions.ModifyTransaction(nil, secondExpense, false, 0, nil, nil, nil, nil))
	require.Equal(t, firstExpense.BookId, secondExpense.BookId)
	require.Equal(t, int64(1999670), f.balance())
}

func TestBooksSQLiteTransferPairOwnershipAndArchive(t *testing.T) {
	f := newInvestmentDBFixture(t)
	second, err := Books.Create(nil, f.uid, models.BookCreateRequest{Name: "旅行"})
	require.NoError(t, err)
	_, err = f.engine.Insert(&models.Account{AccountId: 1003, Uid: f.uid, Type: models.ACCOUNT_TYPE_SINGLE_ACCOUNT, Category: models.ACCOUNT_CATEGORY_CASH, Name: "现金", Currency: "CNY"}, &models.TransactionCategory{CategoryId: 8200, Uid: f.uid, Type: models.CATEGORY_TYPE_TRANSFER, Name: "转账"}, &models.TransactionCategory{CategoryId: 8201, Uid: f.uid, Type: models.CATEGORY_TYPE_TRANSFER, ParentCategoryId: 8200, Name: "取现"})
	require.NoError(t, err)
	tx := &models.Transaction{Uid: f.uid, BookId: second.Id, Type: models.TRANSACTION_DB_TYPE_TRANSFER_OUT, AccountId: f.bankID, RelatedAccountId: 1003, CategoryId: 8201, Amount: 500, RelatedAccountAmount: 500, TransactionTime: utils.GetMinTransactionTimeFromUnixTime(f.at + 1)}
	require.NoError(t, Transactions.CreateTransaction(nil, tx, nil, nil))
	var paired models.Transaction
	_, err = f.engine.ID(tx.RelatedId).Get(&paired)
	require.NoError(t, err)
	require.Equal(t, second.Id, paired.BookId)
	require.NoError(t, Books.MoveTransactions(nil, f.uid, []int64{paired.TransactionId}, DefaultBookID(f.uid)))
	outID, inID := tx.TransactionId, paired.TransactionId
	tx = &models.Transaction{}
	paired = models.Transaction{}
	_, err = f.engine.ID(outID).Get(tx)
	require.NoError(t, err)
	_, err = f.engine.ID(inID).Get(&paired)
	require.NoError(t, err)
	require.Equal(t, tx.BookId, paired.BookId)
	require.Equal(t, DefaultBookID(f.uid), tx.BookId)
	require.Equal(t, int64(1999500), f.balance())
	other, err := Books.Create(nil, f.uid+1, models.BookCreateRequest{Name: "其他用户"})
	require.NoError(t, err)
	_, err = Books.FilterContext(nil, f.uid, second.Id+","+other.Id)
	require.ErrorIs(t, err, ErrBookNotFound)
	require.ErrorIs(t, Books.MoveTransactions(nil, f.uid, []int64{tx.TransactionId}, other.Id), ErrBookNotFound)
	_, err = Books.Modify(nil, f.uid, models.BookModifyRequest{Id: DefaultBookID(f.uid), Archived: bookPtr(true)})
	require.ErrorIs(t, err, ErrDefaultBookArchived)
	_, err = Books.Modify(nil, f.uid, models.BookModifyRequest{Id: second.Id, IsDefault: bookPtr(true)})
	require.NoError(t, err)
	_, err = Books.Modify(nil, f.uid, models.BookModifyRequest{Id: DefaultBookID(f.uid), Archived: bookPtr(true)})
	require.NoError(t, err)
	newTx := addBookExpense(f, "", 30, 3)
	require.Equal(t, second.Id, newTx.BookId)
	require.ErrorIs(t, Books.MoveTransactions(nil, f.uid, []int64{newTx.TransactionId}, DefaultBookID(f.uid)), ErrBookArchived)
}

func TestBooksSQLiteLegacyMigrationAndInvestmentCostHistory(t *testing.T) {
	f := newInvestmentDBFixture(t)
	buy := f.create(f.event(investments.Buy, "2", "200", "0", 1), "books-buy")
	sell := f.create(f.event(investments.Sell, "1", "140", "0", 2), "books-sell")
	before := f.balance()
	// Simulate records created by the old schema, including historical revision payloads.
	_, err := f.engine.Where("uid=?", f.uid).Cols("book_id").Update(&models.Transaction{BookId: ""})
	require.NoError(t, err)
	var rows []models.InvestmentEventRecord
	require.NoError(t, f.engine.Where("uid=?", f.uid).Find(&rows))
	for _, row := range rows {
		var payload map[string]json.RawMessage
		require.NoError(t, json.Unmarshal([]byte(row.Payload), &payload))
		delete(payload, "bookId")
		data, err := json.Marshal(payload)
		require.NoError(t, err)
		_, err = f.engine.ID(row.Id).Cols("payload").Update(&models.InvestmentEventRecord{Payload: string(data)})
		require.NoError(t, err)
	}
	require.NoError(t, Books.MigrateUser(nil, f.uid))
	require.NoError(t, Books.MigrateUser(nil, f.uid))
	books, err := Books.List(nil, f.uid)
	require.NoError(t, err)
	require.Len(t, books, 1)
	require.Equal(t, before, f.balance())
	var migrated []models.Transaction
	require.NoError(t, f.engine.Where("uid=?", f.uid).Find(&migrated))
	for _, tx := range migrated {
		require.Equal(t, DefaultBookID(f.uid), tx.BookId)
	}
	other, err := Books.Create(nil, f.uid, models.BookCreateRequest{Name: "投资复盘"})
	require.NoError(t, err)
	revised := sell.Event
	revised.BookID = other.Id
	_, err = f.s.Mutate(nil, f.uid, revised, "", "revise", false)
	require.NoError(t, err)
	require.Equal(t, before, f.balance())
	events, err := f.s.Events(nil, f.uid)
	require.NoError(t, err)
	result, err := replayInvestments(events)
	require.NoError(t, err)
	require.Len(t, result.Positions, 1)
	requireMoney(t, "100", result.Positions[0].Cost)
	for _, effect := range result.Effects {
		if effect.EventID == sell.Event.ID {
			requireMoney(t, "40", effect.RealizedPNL)
		}
	}
	// Investment-linked cash cannot be moved independently of the event.
	var linked models.Transaction
	has, err := f.engine.Where("uid=? AND investment_event_id=?", f.uid, buy.Event.ID).Get(&linked)
	require.NoError(t, err)
	require.True(t, has)
	require.Equal(t, buy.Event.ID, linked.ToTransactionInfoResponse(nil, false).InvestmentEventId)
	require.ErrorIs(t, Books.MoveTransactions(nil, f.uid, []int64{linked.TransactionId}, other.Id), ErrInvestmentLinked)
	require.Equal(t, before, f.balance())
}

func TestBooksSQLiteTemplatePreservesAttributionAcrossDefaultChange(t *testing.T) {
	f := newInvestmentDBFixture(t)
	second, err := Books.Create(nil, f.uid, models.BookCreateRequest{Name: "旅行"})
	require.NoError(t, err)
	tx := addBookExpense(f, second.Id, 10, 1)
	template := &models.TransactionTemplate{Uid: f.uid, BookId: second.Id, TemplateType: models.TRANSACTION_TEMPLATE_TYPE_NORMAL, Name: "餐饮模板", Type: models.TRANSACTION_TYPE_EXPENSE, CategoryId: tx.CategoryId, AccountId: f.bankID, Amount: 10}
	require.NoError(t, TransactionTemplates.CreateTemplate(nil, template))
	_, err = Books.Modify(nil, f.uid, models.BookModifyRequest{Id: second.Id, IsDefault: bookPtr(true), ShowTransfers: bookPtr(false), ShowInvestments: bookPtr(false)})
	require.NoError(t, err)
	template.BookId = ""
	template.Name = "更新餐饮模板"
	require.NoError(t, TransactionTemplates.ModifyTemplate(nil, template))
	require.Equal(t, second.Id, template.BookId)
	legacy := &models.TransactionTemplate{TemplateId: 8801, Uid: f.uid, Name: "旧模板", TemplateType: models.TRANSACTION_TEMPLATE_TYPE_NORMAL}
	_, err = f.engine.Insert(legacy)
	require.NoError(t, err)
	require.NoError(t, Books.MigrateUser(nil, f.uid))
	require.NoError(t, Books.MigrateUser(nil, f.uid))
	stored, err := TransactionTemplates.GetTemplateByTemplateId(nil, f.uid, template.TemplateId)
	require.NoError(t, err)
	require.Equal(t, second.Id, stored.BookId)
	storedLegacy, err := TransactionTemplates.GetTemplateByTemplateId(nil, f.uid, legacy.TemplateId)
	require.NoError(t, err)
	require.Equal(t, DefaultBookID(f.uid), storedLegacy.BookId)
	books, err := Books.List(nil, f.uid)
	require.NoError(t, err)
	for _, book := range books {
		if book.Id == second.Id {
			require.True(t, book.IsDefault)
			require.False(t, book.ShowTransfers)
			require.False(t, book.ShowInvestments)
		}
	}
	other, err := Books.Create(nil, f.uid+1, models.BookCreateRequest{Name: "其他用户"})
	require.NoError(t, err)
	template.BookId = other.Id
	require.ErrorIs(t, TransactionTemplates.ModifyTemplate(nil, template), ErrBookNotFound)
	stored, err = TransactionTemplates.GetTemplateByTemplateId(nil, f.uid, template.TemplateId)
	require.NoError(t, err)
	require.Equal(t, second.Id, stored.BookId)
}
