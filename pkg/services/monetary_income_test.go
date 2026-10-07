package services

import (
	"fmt"
	"github.com/mayswind/ezbookkeeping/pkg/marketquotes"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func monetaryFixture(t *testing.T) (*investmentDBFixture, *models.MonetaryIncomeBinding) {
	f := newInvestmentDBFixture(t)
	z, _ := time.LoadLocation("Asia/Shanghai")
	at, _ := time.ParseInLocation("2006-01-02", "2026-09-28", z)
	_, err := f.engine.Where("uid=? AND account_id=?", f.uid, f.bankID).Cols("transaction_time").Update(&models.Transaction{TransactionTime: utils.GetMinTransactionTimeFromUnixTime(at.Unix())})
	require.NoError(t, err)
	_, err = f.engine.Insert(&models.TransactionCategory{CategoryId: 9100, Uid: f.uid, Name: "投资收益", Type: models.CATEGORY_TYPE_INCOME}, &models.TransactionCategory{CategoryId: 9101, Uid: f.uid, ParentCategoryId: 9100, Name: "投资收益", Type: models.CATEGORY_TYPE_INCOME})
	require.NoError(t, err)
	b := &models.MonetaryIncomeBinding{Id: "binding", Uid: f.uid, AccountId: f.bankID, Code: "000198", Name: "货币基金", StartDate: "2026-09-29", NextDate: "2026-09-29", BookId: DefaultBookID(f.uid), CategoryId: 9101, TimeZone: "Asia/Shanghai", Enabled: true}
	_, err = f.engine.Insert(b)
	require.NoError(t, err)
	return f, b
}

func TestMonetaryRetryAfterPublicationWithinMinuteAndReturnsNewTotal(t *testing.T) {
	f, b := monetaryFixture(t)
	var published atomic.Bool
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if !published.Load() {
			http.Error(w, "not ready", 503)
			return
		}
		fmt.Fprint(w, `{"ErrCode":0,"Data":{"FundType":"005","LSJZList":[{"FSRQ":"2026-09-29","DWJZ":"1"}]}}`)
	}))
	defer srv.Close()
	marketquotes.Default = marketquotes.New(marketquotes.Config{FundNAVURL: srv.URL})
	now, _ := time.Parse(time.RFC3339, "2026-09-30T09:00:00+08:00")
	result, err := MonetaryIncome.syncAt(nil, f.uid, b.AccountId, false, now)
	require.NoError(t, err)
	require.Zero(t, result.Created)
	published.Store(true)
	_, err = MonetaryIncome.syncAt(nil, f.uid, b.AccountId, false, now.Add(30*time.Second))
	require.NoError(t, err)
	require.Equal(t, int32(1), calls.Load())
	result, err = MonetaryIncome.syncAt(nil, f.uid, b.AccountId, false, now.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, 1, result.Created)
	require.Equal(t, "2", result.Bindings[0].TotalIncome)
	_, err = MonetaryIncome.syncAt(nil, f.uid, b.AccountId, false, now.Add(2*time.Minute))
	require.NoError(t, err)
	require.Equal(t, int32(2), calls.Load())
	require.Equal(t, int64(1), f.count(&models.MonetaryIncomeDay{}))
	detail, err := MonetaryIncome.TransactionDetail(nil, f.uid, result.Bindings[0].LastTransactionId)
	require.NoError(t, err)
	require.Equal(t, "2026-09-29", detail.Date)
	other, err := MonetaryIncome.TransactionDetail(nil, f.uid+1, result.Bindings[0].LastTransactionId)
	require.NoError(t, err)
	require.Nil(t, other)
}

func TestMonetarySQLiteAtomicIncomeBalanceIdempotenceAndDeletion(t *testing.T) {
	f, b := monetaryFixture(t)
	n, err := MonetaryIncome.settleDay(nil, f.uid, b.Id, b.NextDate, "1")
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, int64(2000200), f.balance())
	var day models.MonetaryIncomeDay
	has, err := f.engine.Where("uid=?", f.uid).Get(&day)
	require.NoError(t, err)
	require.True(t, has)
	require.Equal(t, "20000.00", day.Principal)
	require.Equal(t, "2.00", day.Amount)
	var tx models.Transaction
	_, err = f.engine.ID(day.TransactionId).Get(&tx)
	require.NoError(t, err)
	require.Equal(t, models.TRANSACTION_DB_TYPE_INCOME, tx.Type)
	require.Equal(t, int64(200), tx.Amount)
	listed, err := MonetaryIncome.List(nil, f.uid)
	require.NoError(t, err)
	require.Equal(t, "2", listed[0].TotalIncome)
	z, _ := time.LoadLocation("Asia/Shanghai")
	require.Equal(t, "2026-09-30", time.Unix(utils.GetUnixTimeFromTransactionTime(tx.TransactionTime), 0).In(z).Format("2006-01-02"))
	_, err = MonetaryIncome.settleDay(nil, f.uid, b.Id, b.NextDate, "1")
	require.Error(t, err)
	require.Equal(t, int64(2000200), f.balance())
	require.NoError(t, Transactions.DeleteTransaction(nil, f.uid, tx.TransactionId))
	listed, err = MonetaryIncome.List(nil, f.uid)
	require.NoError(t, err)
	require.Equal(t, "0", listed[0].TotalIncome)
	require.Equal(t, int64(2000000), f.balance())
	// Even a restored older cursor cannot recreate a processed, deleted day.
	_, err = f.engine.ID(b.Id).Cols("next_date").Update(b)
	require.NoError(t, err)
	n, err = MonetaryIncome.settleDay(nil, f.uid, b.Id, b.NextDate, "1")
	require.NoError(t, err)
	require.Zero(t, n)
	require.Equal(t, int64(2000000), f.balance())
	require.Equal(t, int64(1), f.count(&models.MonetaryIncomeDay{}))
}

func TestMonetarySQLiteFailureRollsBackAndCannotUseOtherUser(t *testing.T) {
	f, b := monetaryFixture(t)
	_, err := f.engine.ID(9101).Cols("hidden").Update(&models.TransactionCategory{Hidden: true})
	require.NoError(t, err)
	_, err = MonetaryIncome.settleDay(nil, f.uid, b.Id, b.NextDate, "1")
	require.Error(t, err)
	require.Zero(t, f.count(&models.MonetaryIncomeDay{}))
	require.Equal(t, int64(2000000), f.balance())
	var stored models.MonetaryIncomeBinding
	_, err = f.engine.ID(b.Id).Get(&stored)
	require.NoError(t, err)
	require.Equal(t, b.NextDate, stored.NextDate)
	_, err = MonetaryIncome.settleDay(nil, f.uid+1, b.Id, b.NextDate, "1")
	require.Error(t, err)
	_, err = MonetaryIncome.settleDay(nil, f.uid, b.Id, b.NextDate, "-0.1")
	require.ErrorIs(t, err, errNegativeMonetaryYield)
}

func TestMonetarySQLiteInheritancePauseAndMergeProtection(t *testing.T) {
	f, b := monetaryFixture(t)
	_, err := MonetaryIncome.settleDay(nil, f.uid, b.Id, b.NextDate, "1")
	require.NoError(t, err)
	var stored models.MonetaryIncomeBinding
	_, err = f.engine.ID(b.Id).Get(&stored)
	require.NoError(t, err)
	book, err := Books.Create(nil, f.uid, models.BookCreateRequest{Name: "收益账本"})
	require.NoError(t, err)
	require.NoError(t, Books.MoveTransactions(nil, f.uid, []int64{stored.LastTransactionId}, book.Id))
	_, err = MonetaryIncome.settleDay(nil, f.uid, b.Id, "2026-09-30", "1")
	require.NoError(t, err)
	var day models.MonetaryIncomeDay
	_, err = f.engine.ID(fmt.Sprintf("%d:%d:2026-09-30", f.uid, f.bankID)).Get(&day)
	require.NoError(t, err)
	var tx models.Transaction
	_, err = f.engine.ID(day.TransactionId).Get(&tx)
	require.NoError(t, err)
	require.Equal(t, book.Id, tx.BookId)
	require.ErrorIs(t, Transactions.MoveAllTransactionsBetweenAccounts(nil, f.uid, f.bankID, 1003), ErrMonetaryMove)
	require.NoError(t, Transactions.DeleteAllTransactions(nil, f.uid, false))
	stored = models.MonetaryIncomeBinding{}
	_, err = f.engine.ID(b.Id).Get(&stored)
	require.NoError(t, err)
	require.False(t, stored.Enabled)
	_, err = MonetaryIncome.settleDay(nil, f.uid, b.Id, stored.NextDate, "1")
	require.Error(t, err)
}

func TestMonetarySQLiteSyncWaitsForGapAndConcurrentRetryNeverDuplicates(t *testing.T) {
	f, b := monetaryFixture(t)
	var complete atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		from, to := r.URL.Query().Get("startDate"), r.URL.Query().Get("endDate")
		rows := ""
		for _, date := range []string{"2026-09-29", "2026-09-30", "2026-10-01"} {
			if date < from || date > to || (date == "2026-09-30" && !complete.Load()) {
				continue
			}
			if rows != "" {
				rows += ","
			}
			rows += fmt.Sprintf(`{"FSRQ":%q,"DWJZ":"1"}`, date)
		}
		fmt.Fprintf(w, `{"ErrCode":0,"Data":{"FundType":"005","LSJZList":[%s]}}`, rows)
	}))
	defer srv.Close()
	marketquotes.Default = marketquotes.New(marketquotes.Config{FundNAVURL: srv.URL})
	now, _ := time.Parse(time.RFC3339, "2026-10-02T12:00:00+08:00")
	result, err := MonetaryIncome.syncAt(nil, f.uid, b.AccountId, true, now)
	require.NoError(t, err)
	require.Equal(t, 1, result.Created)
	require.Equal(t, "2026-09-30", result.Bindings[0].NextDate)
	require.Contains(t, result.Bindings[0].Status, "等待")
	complete.Store(true)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := MonetaryIncome.syncAt(nil, f.uid, b.AccountId, true, now)
			require.NoError(t, err)
		}()
	}
	wg.Wait()
	require.Equal(t, int64(3), f.count(&models.MonetaryIncomeDay{}))
	require.Equal(t, int64(2000600), f.balance())
}

func TestMonetarySimpleBindingDefaultsBackdatesOpeningAndRebindsWithoutDuplicate(t *testing.T) {
	f := newInvestmentDBFixture(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"ErrCode":0,"Datas":[{"CODE":"000198","NAME":"测试货币基金","CATEGORY":700,"FundBaseInfo":{"FCODE":"000198","FUNDTYPE":"005"}},{"CODE":"000199","NAME":"测试第二只货币基金","CATEGORY":700,"FundBaseInfo":{"FCODE":"000199","FUNDTYPE":"005"}}]}`)
	}))
	defer srv.Close()
	marketquotes.Default = marketquotes.New(marketquotes.Config{FundSearchURL: srv.URL})
	req := models.MonetaryIncomeSaveRequest{AccountId: fmt.Sprint(f.bankID), Code: "000198", StartDate: "2026-09-29", TimeZone: "Asia/Shanghai", Enabled: true}
	b, err := MonetaryIncome.Save(nil, f.uid, req)
	require.NoError(t, err)
	require.NotZero(t, b.CategoryId)
	require.NotEmpty(t, b.BookId)
	var opening models.Transaction
	_, err = f.engine.Where("uid=? AND account_id=?", f.uid, f.bankID).Asc("transaction_time").Get(&opening)
	require.NoError(t, err)
	zone, _ := time.LoadLocation(req.TimeZone)
	require.Equal(t, "2026-09-28", time.Unix(utils.GetUnixTimeFromTransactionTime(opening.TransactionTime), 0).In(zone).Format("2006-01-02"))
	_, err = MonetaryIncome.settleDay(nil, f.uid, b.Id, b.NextDate, "1")
	require.NoError(t, err)
	var stored models.MonetaryIncomeBinding
	_, err = f.engine.ID(b.Id).Get(&stored)
	require.NoError(t, err)
	_, err = f.engine.ID(stored.LastTransactionId).Cols("exclude_from_statistics").Update(&models.Transaction{ExcludeFromStatistics: true})
	require.NoError(t, err)
	req.Code = "000199"
	b, err = MonetaryIncome.Save(nil, f.uid, req)
	require.NoError(t, err)
	require.Equal(t, "2026-09-29", b.NextDate)
	n, err := MonetaryIncome.settleDay(nil, f.uid, b.Id, b.NextDate, "2")
	require.NoError(t, err)
	require.Zero(t, n)
	require.Equal(t, int64(2000200), f.balance())
	_, err = MonetaryIncome.settleDay(nil, f.uid, b.Id, "2026-09-30", "1")
	require.NoError(t, err)
	var day models.MonetaryIncomeDay
	_, err = f.engine.ID(fmt.Sprintf("%d:%d:2026-09-30", f.uid, f.bankID)).Get(&day)
	require.NoError(t, err)
	var tx models.Transaction
	_, err = f.engine.ID(day.TransactionId).Get(&tx)
	require.NoError(t, err)
	require.True(t, tx.ExcludeFromStatistics)
}
