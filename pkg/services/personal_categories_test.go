package services

import (
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPersonalCategoriesInitializeOnceWithoutChangingLedger(t *testing.T) {
	f := newInvestmentDBFixture(t)
	balance, transactions := f.balance(), f.count(&models.Transaction{})
	require.NoError(t, TransactionCategories.EnsurePersonalCategories(nil, f.uid))
	all, err := TransactionCategories.GetAllCategoriesByUid(nil, f.uid, 0, -1)
	require.NoError(t, err)
	parents := map[int64]*models.TransactionCategory{}
	for _, category := range all {
		if category.ParentCategoryId == 0 {
			parents[category.CategoryId] = category
		}
	}
	require.Len(t, parents, len(personalCategoryPresets))
	for _, category := range all {
		if category.ParentCategoryId == 0 {
			continue
		}
		require.Contains(t, parents, category.ParentCategoryId)
		require.Equal(t, parents[category.ParentCategoryId].Type, category.Type)
		require.Empty(t, category.BookIds)
	}
	require.NoError(t, TransactionCategories.EnsurePersonalCategories(nil, f.uid))
	require.Equal(t, int64(len(all)), f.count(&models.TransactionCategory{}))
	require.Equal(t, balance, f.balance())
	require.Equal(t, transactions, f.count(&models.Transaction{}))
}

func TestPersonalCategoriesPreserveCustomAndDeletedCategories(t *testing.T) {
	for _, deleted := range []bool{false, true} {
		f := newInvestmentDBFixture(t)
		existing := &models.TransactionCategory{CategoryId: 901, Uid: f.uid, Name: "我的分类", Deleted: deleted}
		_, err := f.engine.Insert(existing)
		require.NoError(t, err)
		require.NoError(t, TransactionCategories.EnsurePersonalCategories(nil, f.uid))
		require.Equal(t, int64(1), f.count(&models.TransactionCategory{}))
		var got models.TransactionCategory
		_, err = f.engine.ID(existing.CategoryId).Get(&got)
		require.NoError(t, err)
		require.Equal(t, existing.Name, got.Name)
		require.Equal(t, deleted, got.Deleted)
	}
}
