package marketquotes

import (
	"context"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFundHistoryRejectsMonetaryAndConflictingDates(t *testing.T) {
	response := `{"ErrCode":0,"Data":{"FundType":"005","LSJZList":[{"FSRQ":"2026-01-02","DWJZ":"0.4"}]}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()
	s := New(Config{FundNAVURL: server.URL})
	_, err := s.FundHistory(context.Background(), "000001", "2026-01-01", "2026-01-10")
	require.Error(t, err)
	response = `{"ErrCode":0,"Data":{"FundType":"001","LSJZList":[{"FSRQ":"2026-01-03","DWJZ":"1.5"},{"FSRQ":"2026-01-02","DWJZ":"1.4"}]}}`
	rows, err := s.FundHistory(context.Background(), "000001", "2026-01-01", "2026-01-10")
	require.NoError(t, err)
	require.Equal(t, "2026-01-02", rows[0].Date)
	response = `{"ErrCode":0,"Data":{"FundType":"001","LSJZList":[{"FSRQ":"2026-01-02","DWJZ":"1.5"},{"FSRQ":"2026-01-02","DWJZ":"1.4"}]}}`
	_, err = s.FundHistory(context.Background(), "000001", "2026-01-01", "2026-01-10")
	require.Error(t, err)
}
