package marketquotes

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMonetaryPublicSourceSeparatesYieldFromNAV(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/search" {
			fmt.Fprint(w, `{"ErrCode":0,"Datas":[{"CODE":"000198","NAME":"余额宝货币","CATEGORY":700,"FundBaseInfo":{"FCODE":"000198","FUNDTYPE":"005"}},{"CODE":"000001","NAME":"普通基金","CATEGORY":700,"FundBaseInfo":{"FCODE":"000001","FUNDTYPE":"001"}}]}`)
			return
		}
		require.Equal(t, "000198", r.URL.Query().Get("fundCode"))
		require.Equal(t, "https://fundf10.eastmoney.com/", r.Header.Get("Referer"))
		fmt.Fprint(w, `{"ErrCode":0,"Data":{"FundType":"005","LSJZList":[{"FSRQ":"2026-09-30","DWJZ":"0.2253"},{"FSRQ":"2026-09-29","DWJZ":"--"},{"FSRQ":"2026-09-28","DWJZ":"0"}]}}`)
	}))
	defer server.Close()
	s := New(Config{FundSearchURL: server.URL + "/search", FundNAVURL: server.URL + "/yield"})
	candidates, err := s.SearchMonetaryFunds(context.Background(), "000198")
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, "MONETARY_FUND", candidates[0].Type)
	ordinary, err := s.searchFunds(context.Background(), "000198")
	require.NoError(t, err)
	require.Len(t, ordinary, 1)
	require.Equal(t, "000001", ordinary[0].Symbol)
	rows, err := s.MonetaryYields(context.Background(), "000198", "2026-09-28", "2026-09-30")
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, "0", rows[0].PerTenThousand)
	require.Equal(t, "0.2253", rows[1].PerTenThousand)
	_, err = s.fetchFund(context.Background(), candidates[0].Binding)
	require.Error(t, err)
}

func TestMonetaryCatalogueCacheSupportsSelectionAndSave(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	requests, unavailable := 0, false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if unavailable {
			http.Error(w, "temporary outage", http.StatusBadGateway)
			return
		}
		fmt.Fprint(w, `{"ErrCode":0,"Datas":[{"CODE":"000505","NAME":"国寿安保货币A","CATEGORY":700,"FundBaseInfo":{"FCODE":"000505","FUNDTYPE":"005"}},{"CODE":"000001","NAME":"普通基金","CATEGORY":700,"FundBaseInfo":{"FCODE":"000001","FUNDTYPE":"001"}}]}`)
	}))
	defer server.Close()
	s := New(Config{FundSearchURL: server.URL, Now: func() time.Time { return now }})
	selected, err := s.SearchMonetaryFunds(context.Background(), "国寿安保货币A")
	require.NoError(t, err)
	require.Len(t, selected, 1)
	selected[0].Name = "caller changed the name"
	unavailable = true
	verified, err := s.SearchMonetaryFunds(context.Background(), "000505")
	require.NoError(t, err)
	require.Equal(t, "国寿安保货币A", verified[0].Name)
	require.Equal(t, 1, requests)
	_, err = s.SearchMonetaryFunds(context.Background(), "000001")
	require.Error(t, err) // Ordinary funds never enter the verified money-fund cache.
	now = now.Add(11 * time.Minute)
	_, err = s.SearchMonetaryFunds(context.Background(), "000505")
	require.Error(t, err) // An expired identity is not treated as freshly verified.
	unavailable = false
	_, err = s.SearchMonetaryFunds(context.Background(), "000505")
	require.NoError(t, err) // Failed lookups are not cached.
	require.Equal(t, 4, requests)
}

func TestMonetaryRejectsWrongTypeConflictingRowsAndBadDecimals(t *testing.T) {
	for _, body := range []string{
		`{"ErrCode":0,"Data":{"FundType":"001","LSJZList":[]}}`,
		`{"ErrCode":0,"Data":{"FundType":"005","LSJZList":[{"FSRQ":"2026-09-30","DWJZ":"1e100"}]}}`,
		`{"ErrCode":0,"Data":{"FundType":"005","LSJZList":[{"FSRQ":"2026-09-30","DWJZ":"1"},{"FSRQ":"2026-09-30","DWJZ":"2"}]}}`,
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
		_, err := New(Config{FundNAVURL: srv.URL}).MonetaryYields(context.Background(), "000198", "2026-09-30", "2026-09-30")
		require.Error(t, err)
		srv.Close()
	}
}
