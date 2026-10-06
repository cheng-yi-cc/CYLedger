package services

import (
	"fmt"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDebtReminderTracksFactAndRejectsIndependentChanges(t *testing.T) {
	f, tx := feeFixture(t)
	tx.TransferFeeAmount = "0"
	tx.TransferFeeCategoryId = 0
	_, err := f.engine.ID(tx.AccountId).Cols("category").Update(&models.Account{Category: models.ACCOUNT_CATEGORY_DEBT})
	require.NoError(t, err)
	tx.DebtDueDate = "2026-11-20"
	tx.LocationName = "测试地点"
	require.NoError(t, Transactions.CreateTransaction(nil, tx, nil, nil))
	id := fmt.Sprintf("debt-transaction-%d", tx.TransactionId)
	read := func() models.CalendarEvent {
		var item models.CalendarEvent
		ok, e := f.engine.ID(id).Get(&item)
		require.NoError(t, e)
		require.True(t, ok)
		return item
	}
	item := read()
	require.Equal(t, tx.BookId, item.BookId)
	require.Equal(t, "100.00", item.Amount)
	balance := f.balance()
	_, err = Calendar.Save(nil, f.uid, models.CalendarSaveRequest{Id: id, BookId: tx.BookId, AccountId: item.AccountId, Kind: "repayment", Date: "2026-12-01", Amount: "100"})
	require.Error(t, err)
	_, err = Calendar.Delete(nil, f.uid, id)
	require.Error(t, err)
	require.Equal(t, balance, f.balance())
	tx.DebtDueDate = "2026-12-10"
	tx.Amount = 20000
	tx.RelatedAccountAmount = 20000
	require.NoError(t, Transactions.ModifyTransaction(nil, tx, false, 0, nil, nil, nil, nil))
	require.Equal(t, "2026-12-10", read().Date)
	require.Equal(t, "200.00", read().Amount)
	book, err := Books.Create(nil, f.uid, models.BookCreateRequest{Name: "另一账本"})
	require.NoError(t, err)
	require.NoError(t, Books.MoveTransactions(nil, f.uid, []int64{tx.TransactionId}, book.Id))
	require.Equal(t, book.Id, read().BookId)
	tx.BookId = book.Id
	tx.DebtDueDate = "2026-02-30"
	before := f.balance()
	require.Error(t, Transactions.ModifyTransaction(nil, tx, false, 0, nil, nil, nil, nil))
	require.Equal(t, before, f.balance())
	require.Equal(t, "2026-12-10", read().Date)
	tx.DebtDueDate = ""
	require.NoError(t, Transactions.ModifyTransaction(nil, tx, false, 0, nil, nil, nil, nil))
	exists, err := f.engine.ID(id).Exist(&models.CalendarEvent{})
	require.NoError(t, err)
	require.False(t, exists)
	tx.DebtDueDate = "2027-01-10"
	require.NoError(t, Transactions.ModifyTransaction(nil, tx, false, 0, nil, nil, nil, nil))
	require.NoError(t, Transactions.DeleteTransaction(nil, f.uid, tx.TransactionId))
	exists, err = f.engine.ID(id).Exist(&models.CalendarEvent{})
	require.NoError(t, err)
	require.False(t, exists)
	require.Equal(t, int64(2000000), f.balance())
}
