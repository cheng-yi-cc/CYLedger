package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/investments"
	"github.com/mayswind/ezbookkeeping/pkg/marketquotes"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestInvestmentBindingManualPriorityAndUserIsolation(t *testing.T) {
	f := newInvestmentDBFixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/search" {
			_, _ = w.Write([]byte(`v_hint="hk~00700~Tencent~tx~GP"`))
			return
		}
		fields := make([]string, 88)
		fields[0], fields[1], fields[2], fields[3], fields[30], fields[32] = "100", "Tencent", "00700", "430", time.Now().UTC().Add(-2*time.Hour).Format("2006/01/02 15:04:05"), "2.63"
		fields[75] = "HKD"
		// Source time uses Hong Kong, independent of the user's accounting zone.
		zone, _ := time.LoadLocation("Asia/Hong_Kong")
		fields[30] = time.Now().In(zone).Add(-time.Minute).Format("2006/01/02 15:04:05")
		body, _ := json.Marshal(strings.Join(fields, "~"))
		_, _ = w.Write([]byte("v_hk00700=" + string(body) + ";"))
	}))
	defer server.Close()
	marketquotes.Default = marketquotes.New(marketquotes.Config{TencentURL: server.URL + "/quote", TencentSearchURL: server.URL + "/search"})
	asset, err := f.s.CreateInstrument(nil, f.uid, models.InvestmentInstrument{Type: "STOCK", Name: "My holding", Symbol: "00700", Market: "HK", Provider: "tencent", ProviderID: "hk00700", Currency: "HKD"})
	require.NoError(t, err)
	f.create(InvestmentEvent{Event: investments.Event{Type: investments.Opening, AccountID: f.portfolioID, InstrumentID: asset.Id, Quantity: "10", OccurredAt: f.at + 1}}, "bound-holding-opening")
	unpriced := f.summary(false)
	require.Nil(t, unpriced.Positions[0].MarketValue, "missing HKD FX must remain unknown")
	marketquotes.Default.Restore(nil, []marketquotes.FXRate{{Base: "HKD", Quote: "CNY", Rate: "0.85", Date: time.Now().UTC().Format("2006-01-02"), Source: marketquotes.SourceECB, ReceivedAt: time.Now().Unix()}})
	priced := f.summary(false)
	requireMoney(t, "3655", priced.Positions[0].MarketValue)
	require.Equal(t, "HKD", priced.Positions[0].Quote.Currency)
	_, err = f.s.ManualQuote(nil, f.uid, asset.Id, "400", time.Now().Unix())
	require.NoError(t, err)
	manual := f.summary(false)
	requireMoney(t, "4000", manual.Positions[0].MarketValue)
	require.Equal(t, "manual", manual.Positions[0].Quote.State)
	require.NoError(t, f.s.RemoveManualQuote(nil, f.uid, asset.Id))
	requireMoney(t, "3655", f.summary(false).Positions[0].MarketValue)
	other, err := f.s.Quotes(nil, 202)
	require.NoError(t, err)
	for _, q := range other {
		require.NotEqual(t, asset.Id, q.InstrumentID)
		require.False(t, strings.HasPrefix(q.InstrumentID, "market:"), "shared provider choices must not leak through another user's API")
	}
	_, err = f.s.BindInstrument(nil, 202, asset.Id, marketquotes.Binding{Market: "HK", Provider: "tencent", ProviderID: "hk00700", Currency: "HKD"})
	require.Error(t, err)
	var stored models.InvestmentInstrument
	has, err := f.engine.ID(asset.Id).Get(&stored)
	require.NoError(t, err)
	require.True(t, has)
	require.Equal(t, "hk00700", stored.ProviderID)
	// The database identity is sufficient to restore the shared reference cache.
	previous := marketquotes.Default
	restored := marketquotes.New(marketquotes.Config{})
	require.NoError(t, restored.Register(instrumentBinding(stored)))
	q, ok := previous.Get(instrumentBinding(stored).Key())
	require.True(t, ok)
	restored.Restore([]marketquotes.Quote{q}, previous.FX())
	marketquotes.Default = restored
	requireMoney(t, "3655", f.summary(false).Positions[0].MarketValue)
}
