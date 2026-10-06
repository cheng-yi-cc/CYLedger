package services

import (
	"github.com/mayswind/ezbookkeeping/pkg/investments"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPortfolioAccountEditPreservesFactsAndHeldCoins(t *testing.T) {
	f := newInvestmentDBFixture(t)
	f.create(f.event(investments.Buy, "0.01", "100", "0", 1), "edit-metadata-buy")
	before, err := f.s.Events(nil, f.uid)
	require.NoError(t, err)
	balance := f.balance()
	input := models.PortfolioAccount{Id: f.portfolioID, Name: "我的钱包", Kind: "WALLET", Platform: "metamask", Instruments: []string{"crypto:bitcoin", "crypto:ethereum"}}
	result, err := f.s.UpdatePortfolioAccount(nil, f.uid, input)
	require.NoError(t, err)
	require.Equal(t, "我的钱包", result.Name)
	after, err := f.s.Events(nil, f.uid)
	require.NoError(t, err)
	require.Equal(t, before, after)
	require.Equal(t, balance, f.balance())
	for _, platform := range []string{"bitget-wallet", ""} {
		input.Platform = platform
		result, err = f.s.UpdatePortfolioAccount(nil, f.uid, input)
		require.NoError(t, err)
		require.Equal(t, platform, result.Platform)
		after, err = f.s.Events(nil, f.uid)
		require.NoError(t, err)
		require.Equal(t, before, after)
		require.Equal(t, balance, f.balance())
	}
	for _, mutate := range []func(*models.PortfolioAccount){func(a *models.PortfolioAccount) { a.Instruments = []string{"crypto:ethereum"} }, func(a *models.PortfolioAccount) { a.Kind = "EXCHANGE" }, func(a *models.PortfolioAccount) { a.Platform = "binance" }, func(a *models.PortfolioAccount) { a.Instruments = []string{"private:other-user"} }, func(a *models.PortfolioAccount) { a.Name = "" }} {
		bad := input
		mutate(&bad)
		_, err = f.s.UpdatePortfolioAccount(nil, f.uid, bad)
		require.Error(t, err)
	}
	_, err = f.s.UpdatePortfolioAccount(nil, f.uid+1, input)
	require.Error(t, err)
	items, err := f.s.Accounts(nil, f.uid)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, *result, items[0])
	after, err = f.s.Events(nil, f.uid)
	require.NoError(t, err)
	require.Equal(t, before, after)
}
