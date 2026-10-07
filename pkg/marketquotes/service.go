// Package marketquotes maintains a shared cache of public reference prices.
// It never receives positions, account names, or other private ledger data.
package marketquotes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

const (
	StateLive        = "live"
	StateDelayed     = "delayed"
	StateStale       = "stale"
	StateUnavailable = "unavailable"

	SourceCoinbaseWS   = "Coinbase WebSocket"
	SourceCoinbaseREST = "Coinbase REST"
	SourceCoinGecko    = "CoinGecko"
	SourceECB          = "ECB via Frankfurter"
)

// Quote preserves the supplier's price time separately from our retrieval time.
// SourceTime is zero only if the supplier did not provide a time. Empty Price
// means unavailable; it is never a synthetic zero or a fixed stablecoin peg.
type Quote struct {
	InstrumentID  string  `json:"instrumentId"`
	Price         string  `json:"price"`
	Currency      string  `json:"currency"`
	Source        string  `json:"source"`
	SourceTime    int64   `json:"sourceTime"`
	ReceivedAt    int64   `json:"receivedAt"`
	State         string  `json:"state"`
	Connected     bool    `json:"connected"`
	ChangePercent *string `json:"changePercent,omitempty"`
	ChangePeriod  string  `json:"changePeriod,omitempty"`
}

// FXRate is a daily reference rate, not an executable intraday FX quote.
type FXRate struct {
	Base       string `json:"base"`
	Quote      string `json:"quote"`
	Rate       string `json:"rate"`
	Date       string `json:"date"`
	Source     string `json:"source"`
	ReceivedAt int64  `json:"receivedAt"`
	State      string `json:"state"`
}

// Config permits isolated local fake servers in tests. Production defaults only
// query public instrument identifiers. API keys remain in the backend process.
type Config struct {
	HTTPClient          *http.Client
	CoinbaseRESTURL     string
	CoinbaseExchangeURL string
	CoinbaseWSURL       string
	CoinGeckoURL        string
	CoinGeckoAPIKey     string
	FXURL               string
	RESTInterval        time.Duration
	CoinGeckoInterval   time.Duration
	FXInterval          time.Duration
	FXRetryInterval     time.Duration
	ReadTimeout         time.Duration
	ReconnectMin        time.Duration
	ReconnectMax        time.Duration
	StaleAfter          time.Duration
	Now                 func() time.Time
	TencentURL          string
	TencentSearchURL    string
	FundSearchURL       string
	FundNAVURL          string
	CoinGeckoSearchURL  string
	CoinGeckoCoinURL    string
	HKDFXURL            string
}

type instrument struct {
	id        string
	base      string
	productID string
	geckoID   string
}

// These are candidates, not claims of supplier support. Coinbase must confirm
// their exact product, base, and quote identifiers before a subscription.
var instruments = []instrument{
	{"crypto:bitcoin", "BTC", "BTC-USD", "bitcoin"},
	{"crypto:ethereum", "ETH", "ETH-USD", "ethereum"},
	{"crypto:solana", "SOL", "SOL-USD", "solana"},
	{"crypto:tether", "USDT", "USDT-USD", "tether"},
	{"crypto:usd-coin", "USDC", "USDC-USD", "usd-coin"},
}

type cachedQuote struct {
	quote      Quote
	sourceTime time.Time
	restored   bool
}

// Service can be safely shared by all users and HTTP handlers.
type Service struct {
	config               Config
	fundHTTPClient       *http.Client
	once                 sync.Once
	mu                   sync.RWMutex
	quotes               map[string]cachedQuote
	fx                   FXRate
	fxRestored           bool
	products             map[string]string // verified Coinbase product -> instrument ID
	productsVerifiedAt   time.Time
	connected            bool
	heartbeatAt          time.Time
	lastRESTAttempt      time.Time
	lastGeckoAttempt     time.Time
	activeUntil          time.Time
	references           map[string]Binding
	lastReferenceAttempt map[string]time.Time
	searchCache          map[string]searchEntry
	searchMu             sync.Mutex
	monetarySearchMu     sync.Mutex
	monetarySearchCache  map[string]searchEntry
	conversionMu         sync.Mutex
	hkdFX                FXRate
	hkdFXRestored        bool
	dayMu                sync.Mutex
	days                 map[string]*dayEntry
}

// Default is one cache and one public subscription shared by the application.
var Default = New(Config{CoinGeckoAPIKey: os.Getenv("CYLEDGER_COINGECKO_DEMO_API_KEY")})

func New(config Config) *Service {
	var fundHTTPClient *http.Client
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 0 && req.URL.Host != via[0].URL.Host {
				return http.ErrUseLastResponse
			}
			if len(via) >= 5 {
				return errors.New("too many market data redirects")
			}
			return nil
		}}
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.DialContext = fundDNSDial(transport.DialContext, fundResolver.lookup)
		// Some VPN paths take over ten seconds to complete the fund provider's
		// TLS handshake. Keep a single total request budget and normal TLS checks.
		transport.TLSHandshakeTimeout = 15 * time.Second
		fundClient := *config.HTTPClient
		fundClient.Transport = &fundTransport{primary: transport, fallback: http.DefaultTransport.(*http.Transport).Clone(), delay: 750 * time.Millisecond}
		fundClient.Timeout = 20 * time.Second
		fundHTTPClient = &fundClient
	}
	if config.CoinbaseRESTURL == "" {
		config.CoinbaseRESTURL = "https://api.coinbase.com/api/v3/brokerage/market"
	}
	if config.CoinbaseExchangeURL == "" {
		config.CoinbaseExchangeURL = "https://api.exchange.coinbase.com"
	}
	if config.CoinbaseWSURL == "" {
		config.CoinbaseWSURL = "wss://advanced-trade-ws.coinbase.com"
	}
	if config.CoinGeckoURL == "" {
		config.CoinGeckoURL = "https://api.coingecko.com/api/v3/simple/price"
	}
	if config.FXURL == "" {
		config.FXURL = "https://api.frankfurter.dev/v2/providers/ecb/rate/usd/cny"
	}
	if config.RESTInterval <= 0 {
		config.RESTInterval = time.Minute
	}
	// A configuration mistake must not spend the monthly Demo budget. Tests use
	// an injected clock to exercise the same fifteen-minute lower bound.
	if config.CoinGeckoInterval < 15*time.Minute {
		config.CoinGeckoInterval = 15 * time.Minute
	}
	if config.FXInterval <= 0 {
		config.FXInterval = 24 * time.Hour
	}
	if config.FXRetryInterval <= 0 {
		config.FXRetryInterval = 15 * time.Minute
	}
	if config.ReadTimeout <= 0 {
		config.ReadTimeout = 15 * time.Second
	}
	if config.ReconnectMin <= 0 {
		config.ReconnectMin = time.Second
	}
	if config.ReconnectMax <= 0 {
		config.ReconnectMax = time.Minute
	}
	if config.ReconnectMax < config.ReconnectMin {
		config.ReconnectMax = config.ReconnectMin
	}
	if config.StaleAfter <= 0 {
		config.StaleAfter = 2 * time.Minute
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	config.CoinbaseRESTURL = strings.TrimRight(config.CoinbaseRESTURL, "/")
	config.CoinbaseExchangeURL = strings.TrimRight(config.CoinbaseExchangeURL, "/")
	configureReferenceSources(&config)
	return &Service{config: config, fundHTTPClient: fundHTTPClient, quotes: make(map[string]cachedQuote), products: make(map[string]string), references: make(map[string]Binding), lastReferenceAttempt: make(map[string]time.Time), searchCache: make(map[string]searchEntry), monetarySearchCache: make(map[string]searchEntry), days: make(map[string]*dayEntry)}
}

// Foreground reads lease faster public fallback updates without changing prices.
// Idle servers retain the conservative background polling budget.
func (s *Service) MarkActive() {
	s.mu.Lock()
	s.activeUntil = s.config.Now().Add(90 * time.Second)
	s.mu.Unlock()
}

// Start starts background refreshes once and returns immediately. Cancelling ctx
// closes the public socket and all refresh loops. Call once with server lifetime.
func (s *Service) Start(ctx context.Context) {
	s.once.Do(func() {
		go s.coinbaseLoop(ctx)
		go s.restLoop(ctx)
		go s.coinGeckoLoop(ctx)
		go s.fxLoop(ctx)
		go s.referenceLoop(ctx)
		go s.hkdFXLoop(ctx)
	})
}

// Quotes includes missing candidates so clients can explain unavailable prices.
func (s *Service) Quotes() []Quote {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Quote, 0, len(instruments))
	for _, candidate := range instruments {
		if cached, ok := s.quotes[candidate.id]; ok {
			result = append(result, s.quoteViewLocked(cached))
		} else {
			result = append(result, Quote{InstrumentID: candidate.id, Currency: "USD", State: StateUnavailable})
		}
	}
	ids := make([]string, 0, len(s.references))
	for id := range s.references {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if cached, ok := s.quotes[id]; ok {
			result = append(result, s.quoteViewLocked(cached))
		} else {
			result = append(result, Quote{InstrumentID: id, Currency: s.references[id].Currency, State: StateUnavailable})
		}
	}
	return result
}

// Get reports whether an actual price exists. A stale cached price still exists
// and must retain its stale label wherever it is used for reference valuation.
func (s *Service) Get(instrumentID string) (Quote, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cached, ok := s.quotes[instrumentID]
	if !ok {
		return Quote{}, false
	}
	return s.quoteViewLocked(cached), true
}

func (s *Service) FX() []FXRate {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.fx.Rate == "" {
		return []FXRate{{Base: "USD", Quote: "CNY", Source: SourceECB, State: StateUnavailable}, s.hkdFXViewLocked()}
	}
	fx := s.fx
	date, err := time.Parse("2006-01-02", fx.Date)
	now := s.config.Now()
	// Weekends and long central-bank holiday closures are normal for a daily
	// reference rate. Its calendar date and retrieval time remain visible.
	if s.fxRestored || err != nil || now.Sub(date) > 7*24*time.Hour || now.Sub(time.Unix(fx.ReceivedAt, 0)) > 36*time.Hour {
		fx.State = StateStale
	} else {
		fx.State = StateDelayed
	}
	return []FXRate{fx, s.hkdFXViewLocked()}
}

// Restore accepts only known public sources and preserves original timestamps.
// Restored data stays stale until that provider successfully refreshes it.
func (s *Service) Restore(quotes []Quote, rates []FXRate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.config.Now()
	for _, quote := range quotes {
		price, valid := positiveDecimal(quote.Price)
		if !s.validQuoteIdentityLocked(quote) || !valid || quote.SourceTime <= 0 || quote.ReceivedAt <= 0 || quote.SourceTime > now.Add(2*time.Minute).Unix() || quote.ReceivedAt > now.Add(2*time.Minute).Unix() {
			continue
		}
		if existing, ok := s.quotes[quote.InstrumentID]; ok && existing.quote.SourceTime >= quote.SourceTime {
			continue
		}
		quote.Price, quote.State, quote.Connected = price, StateStale, false
		normalizeQuoteChange(&quote)
		s.quotes[quote.InstrumentID] = cachedQuote{quote: quote, sourceTime: time.Unix(quote.SourceTime, 0), restored: true}
	}
	for _, rate := range rates {
		value, valid := positiveDecimal(rate.Rate)
		date, err := time.Parse("2006-01-02", rate.Date)
		if (rate.Base != "USD" && rate.Base != "HKD") || rate.Quote != "CNY" || rate.Source != SourceECB || !valid || err != nil || date.After(now) || rate.ReceivedAt <= 0 || rate.ReceivedAt > now.Add(2*time.Minute).Unix() {
			continue
		}
		if rate.Base == "HKD" {
			if s.hkdFX.Rate == "" || s.hkdFX.Date < rate.Date {
				rate.Rate, rate.State = value, StateStale
				s.hkdFX, s.hkdFXRestored = rate, true
			}
			continue
		}
		if s.fx.Rate != "" && s.fx.Date >= rate.Date {
			continue
		}
		rate.Rate, rate.State = value, StateStale
		s.fx, s.fxRestored = rate, true
	}
}

func (s *Service) quoteViewLocked(cached cachedQuote) Quote {
	quote := cached.quote
	if quote.ChangePercent != nil {
		value := *quote.ChangePercent
		quote.ChangePercent = &value
	}
	now := s.config.Now()
	quote.Connected = quote.Source == SourceCoinbaseWS && s.connected && now.Sub(s.heartbeatAt) < s.config.ReadTimeout && !cached.restored
	if quote.Connected {
		mapped := false
		for _, id := range s.products {
			if id == quote.InstrumentID {
				mapped = true
				break
			}
		}
		quote.Connected = mapped
	}
	if cached.restored {
		quote.State = StateStale
		return quote
	}
	if quote.Source == SourceTencent || quote.Source == SourceFundNAV {
		quote.State = StateDelayed
		maxReceiptAge := 10 * time.Minute
		if quote.Source == SourceFundNAV {
			maxReceiptAge = 48 * time.Hour
		}
		// Closed sessions and fund publication delays retain their actual source
		// date; they do not become a live intraday quote on each HTTP refresh.
		if now.Sub(time.Unix(quote.ReceivedAt, 0)) > maxReceiptAge || now.Sub(cached.sourceTime) > 10*24*time.Hour {
			quote.State = StateStale
		}
		return quote
	}
	// ticker_batch only reports changed prices. A current heartbeat means that
	// an unchanged streamed price remains the latest reference, without changing
	// its actual sourceTime or receivedAt.
	if quote.Connected && quote.State == StateLive {
		return quote
	}
	quote.State = StateDelayed
	maxAge := s.config.StaleAfter
	if quote.Source == SourceCoinGecko {
		maxAge = 2 * s.config.CoinGeckoInterval
	}
	if quote.SourceTime == 0 || now.Sub(cached.sourceTime) > maxAge {
		quote.State = StateStale
	}
	return quote
}

func (s *Service) putQuote(quote Quote, sourceTime time.Time) bool {
	price, valid := positiveDecimal(quote.Price)
	if !valid || !knownInstrument(quote.InstrumentID) || quote.Currency != "USD" || !knownSource(quote.Source) || sourceTime.IsZero() || sourceTime.After(s.config.Now().Add(2*time.Minute)) {
		return false
	}
	quote.Price = price
	quote.SourceTime = sourceTime.Unix()
	quote.Connected = false // computed from current heartbeat on reads
	normalizeQuoteChange(&quote)
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, exists := s.quotes[quote.InstrumentID]; exists {
		// An alive streaming feed takes priority over a slower aggregation feed.
		if quote.Source != SourceCoinbaseWS && s.quoteViewLocked(old).State == StateLive {
			return false
		}
		if sourceTime.Before(old.sourceTime) {
			return false
		}
		if sourceTime.Equal(old.sourceTime) && sourceRank(quote.Source) < sourceRank(old.quote.Source) {
			return false
		}
	}
	s.quotes[quote.InstrumentID] = cachedQuote{quote: quote, sourceTime: sourceTime}
	return true
}

func (s *Service) verifiedProducts() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]string, 0, len(s.products))
	for product := range s.products {
		result = append(result, product)
	}
	sort.Strings(result)
	return result
}

func (s *Service) getJSON(ctx context.Context, address string, headers map[string]string, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "CYLedger/0.1 public-reference-prices")
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	client := s.config.HTTPClient
	if isFundDNSHost(req.URL.Hostname()) && s.fundHTTPClient != nil {
		client = s.fundHTTPClient
	}
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("market data HTTP status %d", response.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 4*1024*1024))
	decoder.UseNumber()
	if err = decoder.Decode(result); err != nil {
		return err
	}
	var trailing interface{}
	if decoder.Decode(&trailing) != io.EOF {
		return errors.New("invalid trailing market response")
	}
	return nil
}

func positiveDecimal(value string) (string, bool) {
	if value == "" || len(value) > 128 {
		return "", false
	}
	number, err := decimal.NewFromString(value)
	// Bound exponent and scale before normalization so malicious public payloads
	// cannot request enormous string allocations.
	if err != nil || number.Exponent() < -36 || number.Exponent() > 36 || !number.IsPositive() {
		return "", false
	}
	return number.String(), true
}

func knownInstrument(id string) bool {
	for _, candidate := range instruments {
		if candidate.id == id {
			return true
		}
	}
	return false
}

func knownSource(source string) bool {
	return source == SourceCoinbaseWS || source == SourceCoinbaseREST || source == SourceCoinGecko
}

func sourceRank(source string) int {
	switch source {
	case SourceCoinbaseWS:
		return 3
	case SourceCoinbaseREST:
		return 2
	case SourceCoinGecko:
		return 1
	}
	return 0
}

func waitContext(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
