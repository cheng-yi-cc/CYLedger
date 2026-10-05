package models

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCategoryScopeLegacyAndExplicitGlobalRequests(t *testing.T) {
	var legacy, global TransactionCategoryModifyRequest
	assert.NoError(t, json.Unmarshal([]byte(`{"id":"1"}`), &legacy))
	assert.NoError(t, json.Unmarshal([]byte(`{"id":"1","bookIds":[]}`), &global))
	assert.Nil(t, legacy.BookIds)
	assert.NotNil(t, global.BookIds)
	response, err := json.Marshal((&TransactionCategory{CategoryId: 1}).ToTransactionCategoryInfoResponse())
	assert.NoError(t, err)
	assert.Contains(t, string(response), `"bookIds":[]`)
}

func TestTransactionCategoryInfoResponseSliceLess(t *testing.T) {
	var transactionCategoryRespSlice TransactionCategoryInfoResponseSlice
	transactionCategoryRespSlice = append(transactionCategoryRespSlice, &TransactionCategoryInfoResponse{
		Id:           1,
		DisplayOrder: 3,
	})
	transactionCategoryRespSlice = append(transactionCategoryRespSlice, &TransactionCategoryInfoResponse{
		Id:           2,
		DisplayOrder: 1,
	})
	transactionCategoryRespSlice = append(transactionCategoryRespSlice, &TransactionCategoryInfoResponse{
		Id:           3,
		DisplayOrder: 2,
	})

	sort.Sort(transactionCategoryRespSlice)

	assert.Equal(t, int64(2), transactionCategoryRespSlice[0].Id)
	assert.Equal(t, int64(3), transactionCategoryRespSlice[1].Id)
	assert.Equal(t, int64(1), transactionCategoryRespSlice[2].Id)
}
