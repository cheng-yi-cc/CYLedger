package services

import (
	"testing"

	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/stretchr/testify/require"
)

// Exercise SQLite JSON persistence, explicit clearing and history lookup behavior.
func TestCategoryBookScopePersistenceAndHistoryVisibility(t *testing.T) {
	f := newInvestmentDBFixture(t)
	book, err := Books.Create(nil, f.uid, models.BookCreateRequest{Name: "旅行账本"})
	require.NoError(t, err)
	category := &models.TransactionCategory{Uid: f.uid, Name: "交通", Type: models.CATEGORY_TYPE_EXPENSE, Icon: 1, Color: "123456", BookIds: []string{book.Id}}
	require.NoError(t, TransactionCategories.CreateCategory(nil, category))
	stored, err := TransactionCategories.GetCategoryByCategoryId(nil, f.uid, category.CategoryId)
	require.NoError(t, err)
	require.Equal(t, []string{book.Id}, stored.BookIds)
	require.NoError(t, TransactionCategories.HideCategory(nil, f.uid, []int64{category.CategoryId}, true))
	stored, err = TransactionCategories.GetCategoryByCategoryId(nil, f.uid, category.CategoryId)
	require.NoError(t, err)
	require.Equal(t, []string{book.Id}, stored.BookIds)
	all, err := TransactionCategories.GetAllCategoriesByUid(nil, f.uid, 0, -1)
	require.NoError(t, err)
	require.Len(t, all, 1, "scope and hidden state must not remove historical category lookup")
	stored.BookIds = []string{}
	require.NoError(t, TransactionCategories.ModifyCategory(nil, stored))
	stored, err = TransactionCategories.GetCategoryByCategoryId(nil, f.uid, category.CategoryId)
	require.NoError(t, err)
	require.Empty(t, stored.BookIds)
	require.NotNil(t, stored.ToTransactionCategoryInfoResponse().BookIds)
}
