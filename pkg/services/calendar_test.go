package services

import (
	"fmt"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestCalendarDueLifecycleDoesNotChangeLedger(t *testing.T) {
	f := newInvestmentDBFixture(t)
	require.NoError(t, f.engine.Sync2(&models.CalendarEvent{}))
	_, err := f.engine.Insert(&models.Account{AccountId: 2001, Uid: f.uid, Name: "信用卡", Type: 1, Category: 3, Currency: "CNY"}, &models.Account{AccountId: 2002, Uid: f.uid, Name: "定存", Type: 1, Category: 9, Currency: "USD"})
	require.NoError(t, err)
	balance, count := f.balance(), f.count(&models.Transaction{})
	req := models.CalendarSaveRequest{BookId: DefaultBookID(f.uid), AccountId: "2001", Kind: "repayment", Date: "2026-10-02", Amount: "123.40", Note: "分期"}
	saved, err := Calendar.Save(nil, f.uid, req)
	require.NoError(t, err)
	require.Equal(t, "123.40", saved.Amount)
	require.Equal(t, "CNY", saved.Currency)
	req.Id = saved.Id
	req.Date = "2026-11-03"
	req.Amount = "23.40"
	_, err = Calendar.Save(nil, f.uid, req)
	require.NoError(t, err)
	october, err := Calendar.List(nil, f.uid, "2026-10")
	require.NoError(t, err)
	require.Empty(t, october)
	november, err := Calendar.List(nil, f.uid, "2026-11")
	require.NoError(t, err)
	require.Len(t, november, 1)
	require.Equal(t, "23.40", november[0].Amount)
	_, err = Calendar.SetCompleted(nil, f.uid, saved.Id, true)
	require.NoError(t, err)
	november, err = Calendar.List(nil, f.uid, "2026-11")
	require.NoError(t, err)
	require.True(t, november[0].Completed)
	_, err = Calendar.SetCompleted(nil, f.uid, saved.Id, false)
	require.NoError(t, err)
	req.Id = ""
	req.AccountId = "2002"
	req.Kind = "deposit"
	req.Date = "2028-02-29"
	deposit, err := Calendar.Save(nil, f.uid, req)
	require.NoError(t, err)
	require.Equal(t, "USD", deposit.Currency)
	_, err = Calendar.Delete(nil, f.uid, saved.Id)
	require.NoError(t, err)
	november, err = Calendar.List(nil, f.uid, "2026-11")
	require.NoError(t, err)
	require.Empty(t, november)
	require.Equal(t, balance, f.balance())
	require.Equal(t, count, f.count(&models.Transaction{}))
}

func TestCalendarOwnershipAndValidation(t *testing.T) {
	f := newInvestmentDBFixture(t)
	require.NoError(t, f.engine.Sync2(&models.CalendarEvent{}))
	_, err := f.engine.Insert(&models.Account{AccountId: 2001, Uid: f.uid, Name: "借入", Type: 1, Category: 5, Currency: "CNY"}, &models.Account{AccountId: 2002, Uid: f.uid + 1, Name: "other", Type: 1, Category: 3, Currency: "CNY"}, &models.Account{AccountId: 2003, Uid: f.uid, Type: 1, Category: 3, SystemRole: "investment_settlement"})
	require.NoError(t, err)
	base := models.CalendarSaveRequest{BookId: DefaultBookID(f.uid), AccountId: "2001", Kind: "repayment", Date: "2026-10-02", Amount: "1.01"}
	saved, err := Calendar.Save(nil, f.uid, base)
	require.NoError(t, err)
	for _, value := range []string{"0", "-1", "1e2", "NaN", "1.001", "01", "99999999999999", strings.Repeat("9", 100)} {
		req := base
		req.Amount = value
		_, err = Calendar.Save(nil, f.uid, req)
		require.Error(t, err, value)
	}
	for _, value := range []string{"2026-02-29", "2026-13-01", "2026-1-01", "1899-01-01"} {
		req := base
		req.Date = value
		_, err = Calendar.Save(nil, f.uid, req)
		require.Error(t, err, value)
	}
	for _, id := range []string{"2002", "2003", fmt.Sprint(f.bankID), "99999"} {
		req := base
		req.AccountId = id
		_, err = Calendar.Save(nil, f.uid, req)
		require.Error(t, err, id)
	}
	req := base
	req.Kind = "deposit"
	_, err = Calendar.Save(nil, f.uid, req)
	require.Error(t, err)
	otherBook, err := Books.Create(nil, f.uid+1, models.BookCreateRequest{Name: "other"})
	require.NoError(t, err)
	req = base
	req.BookId = otherBook.Id
	_, err = Calendar.Save(nil, f.uid, req)
	require.Error(t, err)
	req = base
	req.Id = saved.Id
	_, err = Calendar.Save(nil, f.uid+1, req)
	require.Error(t, err)
	_, err = Calendar.SetCompleted(nil, f.uid+1, saved.Id, true)
	require.Error(t, err)
	_, err = Calendar.Delete(nil, f.uid+1, saved.Id)
	require.Error(t, err)
	other, err := Calendar.List(nil, f.uid+1, "2026-10")
	require.NoError(t, err)
	require.Empty(t, other)
	rows, err := Calendar.List(nil, f.uid, "2026-10")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.False(t, rows[0].Completed)
	req = base
	req.BookId = otherBook.Id
	req.AccountId = "2002"
	_, err = Calendar.Save(nil, f.uid+1, req)
	require.NoError(t, err)
	require.NoError(t, Calendar.DeleteAll(nil, f.uid))
	rows, err = Calendar.List(nil, f.uid, "2026-10")
	require.NoError(t, err)
	require.Empty(t, rows)
	other, err = Calendar.List(nil, f.uid+1, "2026-10")
	require.NoError(t, err)
	require.Len(t, other, 1)
}
