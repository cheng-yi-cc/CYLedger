package services

import (
	"strings"
	"testing"

	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestStatisticsSQLiteBudgetsNotesAndOwnership(t *testing.T) {
	f := newInvestmentDBFixture(t)
	s := StatisticsWorkspace
	b, err := s.SaveBudget(nil, f.uid, models.StatisticsBudget{BookId: DefaultBookID(f.uid), Amount: "2000.01", Kind: "monthly", StartDate: "2026-10-01", EndDate: "2026-10-31", Repeat: true})
	require.NoError(t, err)
	require.Equal(t, "2000.01", b.Amount)
	require.Equal(t, int64(2000000), f.balance())
	_, err = s.SaveBudget(nil, f.uid, models.StatisticsBudget{BookId: b.BookId, Amount: "100", Kind: "monthly", StartDate: b.StartDate, EndDate: b.EndDate})
	require.ErrorIs(t, err, ErrStatisticsConflict)
	other, err := Books.Create(nil, f.uid+1, models.BookCreateRequest{Name: "其他人"})
	require.NoError(t, err)
	for _, amount := range []string{"-1", "1e9", "0.001", "NaN", strings.Repeat("9", 1000)} {
		invalid := *b
		invalid.Amount = amount
		_, err := s.SaveBudget(nil, f.uid, invalid)
		require.ErrorIs(t, err, ErrStatisticsInvalid)
	}
	invalid := *b
	invalid.BookId = other.Id
	_, err = s.SaveBudget(nil, f.uid, invalid)
	require.ErrorIs(t, err, ErrBookNotFound)
	invalid = *b
	invalid.EndDate = "2026-10-30"
	_, err = s.SaveBudget(nil, f.uid, invalid)
	require.ErrorIs(t, err, ErrStatisticsInvalid)
	note, err := s.SaveNote(nil, f.uid, models.StatisticsNote{BookId: b.BookId, Period: "2026-10", Content: "本月总结"})
	require.NoError(t, err)
	stale := *note
	note.Content = "修订总结"
	_, err = s.SaveNote(nil, f.uid, *note)
	require.NoError(t, err)
	_, err = s.SaveNote(nil, f.uid, stale)
	require.ErrorIs(t, err, ErrStatisticsConflict)
	items, err := s.Notes(nil, f.uid+1)
	require.NoError(t, err)
	require.Empty(t, items)
	require.ErrorIs(t, s.DeleteBudget(nil, f.uid+1, models.StatisticsDeleteRequest{Id: b.Id, Revision: b.Revision}), ErrStatisticsConflict)
	require.NoError(t, s.DeleteBudget(nil, f.uid, models.StatisticsDeleteRequest{Id: b.Id, Revision: b.Revision}))
}

func TestStatisticsSQLitePreferenceValidationAndRevision(t *testing.T) {
	f := newInvestmentDBFixture(t)
	p, err := StatisticsWorkspace.Preferences(nil, f.uid)
	require.NoError(t, err)
	p.Modules["month"][0].Visible = false
	p.CarrySurplus = true
	saved, err := StatisticsWorkspace.SavePreferences(nil, f.uid, *p)
	require.NoError(t, err)
	_, err = StatisticsWorkspace.SavePreferences(nil, f.uid, *p)
	require.ErrorIs(t, err, ErrStatisticsConflict)
	loaded, err := StatisticsWorkspace.Preferences(nil, f.uid)
	require.NoError(t, err)
	require.Equal(t, saved, loaded)
	loaded.Modules["month"][1].Id = loaded.Modules["month"][0].Id
	_, err = StatisticsWorkspace.SavePreferences(nil, f.uid, *loaded)
	require.ErrorIs(t, err, ErrStatisticsInvalid)
}

func TestStatisticsSQLiteDiscountIsInformationalAndAtomic(t *testing.T) {
	f := newInvestmentDBFixture(t)
	tx := addBookExpense(f, "", 8000, 1)
	tx.DiscountAmount = "20.10"
	require.NoError(t, Transactions.ModifyTransaction(nil, tx, false, 0, nil, nil, nil, nil))
	require.Equal(t, int64(1992000), f.balance())
	stored, err := Transactions.GetTransactionByTransactionId(nil, f.uid, tx.TransactionId)
	require.NoError(t, err)
	require.Equal(t, "20.10", stored.DiscountAmount)
	tx.DiscountAmount = "-1"
	tx.Amount = 5000
	require.ErrorIs(t, Transactions.ModifyTransaction(nil, tx, false, 0, nil, nil, nil, nil), ErrStatisticsInvalid)
	require.Equal(t, int64(1992000), f.balance())
	tx.DiscountAmount = "0"
	tx.Amount = -1000
	require.NoError(t, Transactions.ModifyTransaction(nil, tx, false, 0, nil, nil, nil, nil))
	require.Equal(t, int64(2001000), f.balance())
}

func TestStatisticsSQLiteTagHierarchyCannotCycleCrossUsersOrOrphan(t *testing.T) {
	f := newInvestmentDBFixture(t)
	parent := &models.TransactionTag{Uid: f.uid, Name: "旅行"}
	require.NoError(t, TransactionTags.CreateTag(nil, parent))
	child := &models.TransactionTag{Uid: f.uid, Name: "杭州", ParentTagId: parent.TagId}
	require.NoError(t, TransactionTags.CreateTag(nil, child))
	grandchild := &models.TransactionTag{Uid: f.uid, Name: "西湖", ParentTagId: child.TagId}
	require.ErrorIs(t, TransactionTags.CreateTag(nil, grandchild), ErrTagHierarchy)
	foreign := &models.TransactionTag{Uid: f.uid + 1, Name: "外部", ParentTagId: parent.TagId}
	require.ErrorIs(t, TransactionTags.CreateTag(nil, foreign), ErrTagHierarchy)
	parent.ParentTagId = child.TagId
	require.ErrorIs(t, TransactionTags.ModifyTag(nil, parent, false), ErrTagHierarchy)
	require.ErrorIs(t, TransactionTags.DeleteTag(nil, f.uid, parent.TagId), ErrTagHasChildren)
	child.ParentTagId = 0
	require.NoError(t, TransactionTags.ModifyTag(nil, child, false))
	require.NoError(t, TransactionTags.DeleteTag(nil, f.uid, parent.TagId))
}
