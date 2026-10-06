package marketquotes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shopspring/decimal"
	"golang.org/x/text/encoding/simplifiedchinese"
)

const (
	SourceTencent = "腾讯财经公开参考行情"
	SourceFundNAV = "天天基金已公布净值"
)

// Binding contains public identifiers only. A ledger's private instrument ID,
// account, position and cost never enter this package or an upstream request.
type Binding struct {
	Market     string `json:"market"`
	Provider   string `json:"provider"`
	ProviderID string `json:"providerId"`
	Currency   string `json:"currency"`
}

type Candidate struct {
	Binding
	Name   string `json:"name"`
	Symbol string `json:"symbol"`
	Type   string `json:"type"`
}

type searchEntry struct {
	items []Candidate
	at    time.Time
}

func (b Binding) Key() string { return "market:" + b.Provider + ":" + b.ProviderID }

var sixDigits = regexp.MustCompile(`^[0-9]{6}$`)
var publicSymbol = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
var decimalText = regexp.MustCompile(`^-?[0-9]{1,36}(\.[0-9]{1,36})?$`)

func configureReferenceSources(c *Config) {
	if c.TencentURL == "" {
		c.TencentURL = "https://qt.gtimg.cn/"
	}
	if c.TencentSearchURL == "" {
		c.TencentSearchURL = "https://smartbox.gtimg.cn/s3/"
	}
	if c.FundSearchURL == "" {
		c.FundSearchURL = "https://fundsuggest.eastmoney.com/FundSearch/api/FundSearchAPI.ashx"
	}
	if c.FundNAVURL == "" {
		c.FundNAVURL = "https://api.fund.eastmoney.com/f10/lsjz"
	}
	if c.CoinGeckoSearchURL == "" {
		c.CoinGeckoSearchURL = "https://api.coingecko.com/api/v3/search"
	}
	if c.CoinGeckoCoinURL == "" {
		c.CoinGeckoCoinURL = "https://api.coingecko.com/api/v3/coins"
	}
	if c.HKDFXURL == "" {
		c.HKDFXURL = "https://api.frankfurter.dev/v2/providers/ecb/rate/hkd/cny"
	}
}

func (b Binding) Valid() bool {
	if !publicSymbol.MatchString(b.ProviderID) {
		return false
	}
	switch b.Provider {
	case "coinbase":
		return b.Market == "CRYPTO" && b.Currency == "USD" && strings.HasSuffix(b.ProviderID, "-USD")
	case "coingecko":
		return b.Market == "CRYPTO" && b.Currency == "USD" && b.ProviderID == strings.ToLower(b.ProviderID)
	case "eastmoney":
		return b.Market == "CN_FUND" && b.Currency == "CNY" && sixDigits.MatchString(b.ProviderID)
	case "tencent":
		switch b.Market {
		case "CN_SH":
			return b.Currency == "CNY" && strings.HasPrefix(b.ProviderID, "sh") && sixDigits.MatchString(strings.TrimPrefix(b.ProviderID, "sh"))
		case "CN_SZ":
			return b.Currency == "CNY" && strings.HasPrefix(b.ProviderID, "sz") && sixDigits.MatchString(strings.TrimPrefix(b.ProviderID, "sz"))
		case "CN_BJ":
			return b.Currency == "CNY" && strings.HasPrefix(b.ProviderID, "bj") && sixDigits.MatchString(strings.TrimPrefix(b.ProviderID, "bj"))
		case "HK":
			return (b.Currency == "HKD" || b.Currency == "CNY" || b.Currency == "USD") && regexp.MustCompile(`^hk[0-9]{5}$`).MatchString(b.ProviderID)
		case "US":
			return b.Currency == "USD" && regexp.MustCompile(`^us[A-Z][A-Z0-9.-]{0,20}$`).MatchString(b.ProviderID)
		}
	}
	return false
}

// Register restores or activates an already confirmed public mapping. It makes
// no network request; refreshes remain independent of browser read frequency.
func (s *Service) Register(b Binding) error {
	if !b.Valid() {
		return errors.New("行情绑定标识无效")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, exists := s.references[b.Key()]; exists && old != b {
		return errors.New("行情绑定的市场或币种不一致")
	}
	if len(s.references) >= 500 {
		if _, exists := s.references[b.Key()]; !exists {
			return errors.New("自动行情资产数量已达上限")
		}
	}
	s.references[b.Key()] = b
	return nil
}

func (s *Service) validQuoteIdentityLocked(q Quote) bool {
	if knownInstrument(q.InstrumentID) {
		return q.Currency == "USD" && knownSource(q.Source)
	}
	b, ok := s.references[q.InstrumentID]
	if !ok || b.Currency != q.Currency {
		return false
	}
	return b.Provider == "tencent" && q.Source == SourceTencent || b.Provider == "eastmoney" && q.Source == SourceFundNAV || b.Provider == "coinbase" && q.Source == SourceCoinbaseREST || b.Provider == "coingecko" && q.Source == SourceCoinGecko
}

func signedPercent(value string) *string {
	value = strings.TrimSuffix(strings.TrimSpace(value), "%")
	if !decimalText.MatchString(value) {
		return nil
	}
	d, err := decimal.NewFromString(value)
	if err != nil {
		return nil
	}
	out := d.String()
	return &out
}

func normalizeQuoteChange(q *Quote) {
	if q.ChangePercent != nil {
		q.ChangePercent = signedPercent(*q.ChangePercent)
	}
	expectedPeriod := "24h"
	if q.Source == SourceTencent {
		expectedPeriod = "session"
	} else if q.Source == SourceFundNAV {
		expectedPeriod = "nav"
	}
	if q.ChangePeriod != expectedPeriod || q.ChangePercent == nil {
		q.ChangePercent, q.ChangePeriod = nil, ""
	}
}

func (s *Service) putReferenceQuote(q Quote) bool {
	price, valid := positiveDecimal(q.Price)
	now := s.config.Now()
	if !valid || q.SourceTime <= 0 || q.SourceTime > now.Add(2*time.Minute).Unix() || q.ReceivedAt <= 0 || q.ReceivedAt > now.Add(2*time.Minute).Unix() {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.validQuoteIdentityLocked(q) {
		return false
	}
	if old, ok := s.quotes[q.InstrumentID]; ok && (old.quote.SourceTime > q.SourceTime || old.quote.ReceivedAt > q.ReceivedAt) {
		return false
	}
	q.Price, q.State, q.Connected = price, StateDelayed, false
	normalizeQuoteChange(&q)
	s.quotes[q.InstrumentID] = cachedQuote{quote: q, sourceTime: time.Unix(q.SourceTime, 0)}
	return true
}

func queryURL(address string, args url.Values) string {
	u, err := url.Parse(address)
	if err != nil {
		return address
	}
	q := u.Query()
	for k, values := range args {
		q[k] = values
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func (s *Service) getText(ctx context.Context, address string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "CYLedger/0.1 public-reference-prices")
	res, err := s.config.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("market data HTTP status %d", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, 1024*1024+1))
	if err != nil {
		return "", err
	}
	if len(data) > 1024*1024 {
		return "", errors.New("market response too large")
	}
	if !utf8.Valid(data) {
		data, err = simplifiedchinese.GBK.NewDecoder().Bytes(data)
	}
	return string(data), err
}

// Search returns provider identities for explicit user selection. Coin symbols
// and securities with the same code are never merged by their display name.
func (s *Service) Search(ctx context.Context, query, market string) ([]Candidate, error) {
	query = strings.TrimSpace(query)
	market = strings.ToUpper(strings.TrimSpace(market))
	if len([]rune(query)) < 1 || len([]rune(query)) > 64 {
		return nil, errors.New("请输入 1 至 64 个字符的资产名称或代码")
	}
	if market != "" && market != "CN" && market != "CN_SH" && market != "CN_SZ" && market != "CN_BJ" && market != "HK" && market != "US" && market != "CN_FUND" && market != "CRYPTO" {
		return nil, errors.New("市场无效")
	}
	key := market + ":" + strings.ToLower(query)
	// Coalesce duplicate searches and bound public-provider pressure globally.
	s.searchMu.Lock()
	defer s.searchMu.Unlock()
	if cached, ok := s.searchCache[key]; ok && s.config.Now().Sub(cached.at) < 10*time.Minute {
		return append([]Candidate{}, cached.items...), nil
	}
	var items []Candidate
	var failures []error
	if market == "" || market == "CRYPTO" {
		coins, err := s.searchCrypto(ctx, query)
		items = append(items, coins...)
		if err != nil {
			failures = append(failures, err)
		}
	}
	if market != "CRYPTO" && market != "CN_FUND" {
		securities, err := s.searchTencent(ctx, query, market)
		items = append(items, securities...)
		if err != nil {
			failures = append(failures, err)
		}
	}
	if market == "" || market == "CN_FUND" {
		funds, err := s.searchFunds(ctx, query)
		items = append(items, funds...)
		if err != nil {
			failures = append(failures, err)
		}
	}
	if len(items) == 0 && len(failures) > 0 {
		return nil, errors.New("公开行情搜索暂不可用，请稍后重试或创建手动估值资产")
	}
	if len(items) > 50 {
		items = items[:50]
	}
	if len(s.searchCache) >= 200 {
		s.searchCache = make(map[string]searchEntry)
	}
	s.searchCache[key] = searchEntry{items: append([]Candidate{}, items...), at: s.config.Now()}
	return append([]Candidate{}, items...), nil
}

func (s *Service) searchTencent(ctx context.Context, query, market string) ([]Candidate, error) {
	body, err := s.getText(ctx, queryURL(s.config.TencentSearchURL, url.Values{"q": {query}, "t": {"all"}}))
	if err != nil {
		return nil, err
	}
	value := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(body), ";"))
	if !strings.HasPrefix(value, "v_hint=") {
		return nil, errors.New("invalid security search response")
	}
	var decoded string
	if err = json.Unmarshal([]byte(strings.TrimPrefix(value, "v_hint=")), &decoded); err != nil {
		return nil, err
	}
	items := make([]Candidate, 0)
	for _, record := range strings.Split(decoded, "^") {
		parts := strings.Split(record, "~")
		if len(parts) < 5 {
			continue
		}
		kind := strings.ToUpper(parts[4])
		if kind != "GP" && kind != "GP-A" && kind != "ETF" && kind != "LOF" && kind != "FUND" {
			continue
		}
		b := Binding{Provider: "tencent", Currency: "CNY"}
		code := parts[1]
		switch parts[0] {
		case "sh":
			b.Market = "CN_SH"
		case "sz":
			b.Market = "CN_SZ"
		case "bj":
			b.Market = "CN_BJ"
		case "hk":
			b.Market, b.Currency = "HK", "HKD"
		case "us":
			b.Market, b.Currency, code = "US", "USD", usTicker(code)
		default:
			continue
		}
		b.ProviderID = parts[0] + code
		if !b.Valid() || market != "" && market != b.Market && !(market == "CN" && strings.HasPrefix(b.Market, "CN_")) {
			continue
		}
		typ := "STOCK"
		if kind == "ETF" || kind == "LOF" || kind == "FUND" {
			typ = "FUND"
		}
		if parts[2] == "" || len([]rune(parts[2])) > 128 {
			continue
		}
		items = append(items, Candidate{Binding: b, Name: parts[2], Symbol: code, Type: typ})
	}
	// Hong Kong has HKD, CNY and USD counters. The search feed does not carry
	// currency, so verify it against each counter's public quote before showing
	// a binding. Inferring HKD merely from the exchange would misvalue CNY/USD.
	hongKong := make([]Binding, 0)
	for _, item := range items {
		if item.Market == "HK" {
			hongKong = append(hongKong, item.Binding)
		}
	}
	if len(hongKong) == 0 {
		return items, nil
	}
	bodies, fetchErr := s.fetchTencentBodies(ctx, hongKong)
	verified := make([]Candidate, 0, len(items))
	for _, item := range items {
		if item.Market != "HK" {
			verified = append(verified, item)
			continue
		}
		body, ok := bodies[item.ProviderID]
		if !ok {
			continue
		}
		fields := strings.Split(body, "~")
		item.Currency = tencentQuoteCurrency(item.Market, fields)
		if len(fields) > 63 && fields[63] == "GP-FUND" {
			item.Type = "FUND"
		}
		if item.Binding.Valid() {
			verified = append(verified, item)
		}
	}
	return verified, fetchErr
}

func usTicker(value string) string {
	value = strings.ToUpper(value)
	for _, suffix := range []string{".OQ", ".N", ".AM", ".PK", ".OB"} {
		if strings.HasSuffix(value, suffix) {
			return strings.TrimSuffix(value, suffix)
		}
	}
	return value
}

type fundSearchResponse struct {
	ErrorCode int `json:"ErrCode"`
	Data      []struct {
		Code     string `json:"CODE"`
		Name     string `json:"NAME"`
		Category int    `json:"CATEGORY"`
		Info     *struct {
			Code string `json:"FCODE"`
			Type string `json:"FUNDTYPE"`
		} `json:"FundBaseInfo"`
	} `json:"Datas"`
}

func (s *Service) searchFunds(ctx context.Context, query string) ([]Candidate, error) {
	var response fundSearchResponse
	if err := s.getJSON(ctx, queryURL(s.config.FundSearchURL, url.Values{"m": {"1"}, "key": {query}}), nil, &response); err != nil {
		return nil, err
	}
	if response.ErrorCode != 0 {
		return nil, errors.New("fund search failed")
	}
	items := make([]Candidate, 0)
	for _, fund := range response.Data {
		if fund.Category != 700 || fund.Info == nil || fund.Code != fund.Info.Code || fund.Info.Type == "005" || !sixDigits.MatchString(fund.Code) || fund.Name == "" {
			continue
		}
		items = append(items, Candidate{Binding: Binding{Market: "CN_FUND", Provider: "eastmoney", ProviderID: fund.Code, Currency: "CNY"}, Name: fund.Name, Symbol: fund.Code, Type: "FUND"})
	}
	return items, nil
}

func (s *Service) searchCrypto(ctx context.Context, query string) ([]Candidate, error) {
	var response struct {
		Coins []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Symbol string `json:"symbol"`
		} `json:"coins"`
	}
	headers := map[string]string{}
	if s.config.CoinGeckoAPIKey != "" {
		headers["x-cg-demo-api-key"] = s.config.CoinGeckoAPIKey
	}
	geckoErr := s.getJSON(ctx, queryURL(s.config.CoinGeckoSearchURL, url.Values{"query": {query}}), headers, &response)
	items := make([]Candidate, 0)
	if geckoErr == nil {
		for _, coin := range response.Coins {
			b := Binding{Market: "CRYPTO", Provider: "coingecko", ProviderID: coin.ID, Currency: "USD"}
			if b.Valid() && coin.Name != "" && len(coin.Symbol) <= 24 {
				items = append(items, Candidate{Binding: b, Name: coin.Name, Symbol: strings.ToUpper(coin.Symbol), Type: "CRYPTO"})
			}
		}
	}
	// Coinbase is an independently verified alternative, including when the
	// keyless CoinGecko endpoint is unavailable in the deployment region.
	var products struct {
		Products []coinbaseProduct `json:"products"`
	}
	err := s.getJSON(ctx, queryURL(s.config.CoinbaseRESTURL+"/products", url.Values{"limit": {"1000"}, "product_type": {"SPOT"}}), nil, &products)
	if err == nil {
		q := strings.ToLower(query)
		for _, p := range products.Products {
			if p.QuoteCurrency != "USD" || p.ProductType != "SPOT" || p.Status != "online" || p.Disabled || p.TradingDisabled || !strings.Contains(strings.ToLower(p.BaseCurrency+" "+p.BaseName), q) {
				continue
			}
			b := Binding{Market: "CRYPTO", Provider: "coinbase", ProviderID: p.ProductID, Currency: "USD"}
			if b.Valid() {
				name := p.BaseName
				if name == "" {
					name = p.BaseCurrency
				}
				items = append(items, Candidate{Binding: b, Name: name, Symbol: p.BaseCurrency, Type: "CRYPTO"})
			}
		}
	}
	if len(items) == 0 && geckoErr != nil && err != nil {
		return nil, err
	}
	return items, nil
}

// Resolve verifies identity and a usable price before saving a new binding.
// Failure leaves the private asset and its existing manual quote untouched.
func (s *Service) Resolve(ctx context.Context, b Binding) (*Candidate, error) {
	if !b.Valid() {
		return nil, errors.New("行情绑定无效，请从搜索结果选择资产")
	}
	var item *Candidate
	var q Quote
	var err error
	switch b.Provider {
	case "tencent":
		var found map[string]Quote
		found, err = s.fetchTencent(ctx, []Binding{b})
		if err == nil {
			if quote, ok := found[b.Key()]; ok {
				q = quote
			} else {
				err = errors.New("该证券未返回可核验的报价")
			}
		}
		if err == nil {
			candidates, searchErr := s.searchTencent(ctx, strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(b.ProviderID, "sh"), "sz"), "bj"), "hk"), "us"), b.Market)
			if searchErr != nil {
				err = searchErr
			} else {
				for _, c := range candidates {
					if c.Binding == b {
						copy := c
						item = &copy
						break
					}
				}
			}
		}
	case "eastmoney":
		q, err = s.fetchFund(ctx, b)
		if err == nil {
			candidates, searchErr := s.searchFunds(ctx, b.ProviderID)
			if searchErr != nil {
				err = searchErr
			} else {
				for _, c := range candidates {
					if c.Binding == b {
						copy := c
						item = &copy
						break
					}
				}
			}
		}
	case "coinbase":
		item, q, err = s.fetchCoinbaseReference(ctx, b)
	case "coingecko":
		var quotes map[string]Quote
		quotes, err = s.fetchGeckoReferences(ctx, []Binding{b})
		if err == nil {
			if quote, ok := quotes[b.Key()]; ok {
				q = quote
			} else {
				err = errors.New("该加密资产暂未返回有效报价")
			}
		}
		if err == nil {
			var response struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				Symbol string `json:"symbol"`
			}
			headers := map[string]string{}
			if s.config.CoinGeckoAPIKey != "" {
				headers["x-cg-demo-api-key"] = s.config.CoinGeckoAPIKey
			}
			// Search matches names and symbols, not stable IDs (BNB's ID is
			// binancecoin). Verify the exact selected ID through coin metadata.
			address := strings.TrimRight(s.config.CoinGeckoCoinURL, "/") + "/" + url.PathEscape(b.ProviderID)
			err = s.getJSON(ctx, queryURL(address, url.Values{"localization": {"false"}, "tickers": {"false"}, "market_data": {"false"}, "community_data": {"false"}, "developer_data": {"false"}}), headers, &response)
			if response.ID == b.ProviderID && strings.TrimSpace(response.Name) != "" && strings.TrimSpace(response.Symbol) != "" && len(response.Symbol) <= 24 {
				item = &Candidate{Binding: b, Name: response.Name, Symbol: strings.ToUpper(response.Symbol), Type: "CRYPTO"}
			}
		}
	}
	if err != nil {
		return nil, err
	}
	if item == nil || item.Name == "" {
		return nil, errors.New("无法核验资产身份，请重新搜索")
	}
	if err = s.Register(b); err != nil {
		return nil, err
	}
	if !s.putReferenceQuote(q) {
		return nil, errors.New("供应商报价无效或已经过期")
	}
	s.mu.Lock()
	s.lastReferenceAttempt[b.Key()] = s.config.Now()
	s.mu.Unlock()
	return item, nil
}

func (s *Service) referenceLoop(ctx context.Context) {
	for ctx.Err() == nil {
		s.refreshReferences(ctx)
		if !waitContext(ctx, 30*time.Second) {
			return
		}
	}
}

func (s *Service) refreshReferences(ctx context.Context) {
	s.mu.Lock()
	groups := make(map[string][]Binding)
	now := s.config.Now()
	keys := make([]string, 0, len(s.references))
	for k := range s.references {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b := s.references[k]
		// Presets and custom CoinGecko bindings share the same periodic batch
		// and attempt budget in refreshCoinGecko.
		if b.Provider == "coingecko" {
			continue
		}
		interval := time.Minute
		if b.Provider == "eastmoney" {
			interval = 30 * time.Minute
		}
		if previous := s.lastReferenceAttempt[k]; !previous.IsZero() && now.Sub(previous) < interval {
			continue
		}
		s.lastReferenceAttempt[k] = now
		groups[b.Provider] = append(groups[b.Provider], b)
	}
	s.mu.Unlock()
	for provider, bindings := range groups {
		if ctx.Err() != nil {
			return
		}
		switch provider {
		case "tencent":
			for start := 0; start < len(bindings); start += 50 {
				end := start + 50
				if end > len(bindings) {
					end = len(bindings)
				}
				quotes, err := s.fetchTencent(ctx, bindings[start:end])
				if err == nil {
					for _, q := range quotes {
						s.putReferenceQuote(q)
					}
				}
			}
		case "eastmoney", "coinbase":
			for _, b := range bindings {
				if ctx.Err() != nil {
					return
				}
				var q Quote
				var err error
				if provider == "eastmoney" {
					q, err = s.fetchFund(ctx, b)
				} else {
					_, q, err = s.fetchCoinbaseReference(ctx, b)
				}
				if err == nil {
					s.putReferenceQuote(q)
				}
			}
		}
	}
}

func (s *Service) fetchTencent(ctx context.Context, bindings []Binding) (map[string]Quote, error) {
	bodies, err := s.fetchTencentBodies(ctx, bindings)
	if err != nil {
		return nil, err
	}
	result := make(map[string]Quote)
	for _, b := range bindings {
		body, exists := bodies[b.ProviderID]
		if !exists {
			continue
		}
		q, err := parseTencent(b, body, s.config.Now())
		if err == nil {
			result[b.Key()] = q
		}
	}
	return result, nil
}

func (s *Service) fetchTencentBodies(ctx context.Context, bindings []Binding) (map[string]string, error) {
	ids := make([]string, 0, len(bindings))
	expected := make(map[string]Binding)
	for _, b := range bindings {
		ids = append(ids, b.ProviderID)
		expected[b.ProviderID] = b
	}
	body, err := s.getText(ctx, queryURL(s.config.TencentURL, url.Values{"q": {strings.Join(ids, ",")}}))
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, line := range strings.Split(body, ";") {
		line = strings.TrimSpace(line)
		prefix, value, ok := strings.Cut(line, "=")
		if !ok || !strings.HasPrefix(prefix, "v_") {
			continue
		}
		b, exists := expected[strings.TrimPrefix(prefix, "v_")]
		if !exists {
			continue
		}
		var decoded string
		if json.Unmarshal([]byte(value), &decoded) != nil {
			continue
		}
		result[b.ProviderID] = decoded
	}
	return result, nil
}

func tencentQuoteCurrency(market string, fields []string) string {
	index := 82
	if market == "HK" {
		index = 75
	} else if market == "US" {
		index = 35
	}
	if len(fields) <= index {
		return ""
	}
	return fields[index]
}

func parseTencent(b Binding, body string, now time.Time) (Quote, error) {
	parts := strings.Split(body, "~")
	if len(parts) < 36 {
		return Quote{}, errors.New("invalid security quote")
	}
	code := strings.TrimPrefix(b.ProviderID, "us")
	if b.Market != "US" {
		code = b.ProviderID[2:]
	}
	responseCode := parts[2]
	if b.Market == "US" {
		responseCode = usTicker(responseCode)
	}
	if responseCode != code || parts[1] == "" {
		return Quote{}, errors.New("security identity mismatch")
	}
	if tencentQuoteCurrency(b.Market, parts) != b.Currency {
		return Quote{}, errors.New("security currency mismatch")
	}
	location, layout := "Asia/Shanghai", "20060102150405"
	if b.Market == "HK" {
		location, layout = "Asia/Hong_Kong", "2006/01/02 15:04:05"
	}
	if b.Market == "US" {
		location, layout = "America/New_York", "2006-01-02 15:04:05"
	}
	zone, err := time.LoadLocation(location)
	if err != nil {
		return Quote{}, err
	}
	at, err := time.ParseInLocation(layout, parts[30], zone)
	if err != nil {
		return Quote{}, err
	}
	price, valid := positiveDecimal(parts[3])
	if !valid || at.After(now.Add(2*time.Minute)) || now.Sub(at) > 370*24*time.Hour {
		return Quote{}, errors.New("invalid security price or time")
	}
	return Quote{InstrumentID: b.Key(), Price: price, Currency: b.Currency, Source: SourceTencent, SourceTime: at.Unix(), ReceivedAt: now.Unix(), State: StateDelayed, ChangePercent: signedPercent(parts[32]), ChangePeriod: "session"}, nil
}

func (s *Service) fetchFund(ctx context.Context, b Binding) (Quote, error) {
	var response struct {
		ErrorCode int `json:"ErrCode"`
		Data      struct {
			Type      string `json:"FundType"`
			YieldType string `json:"SYType"`
			Rows      []struct {
				Date   string `json:"FSRQ"`
				NAV    string `json:"DWJZ"`
				Change string `json:"JZZZL"`
			} `json:"LSJZList"`
		} `json:"Data"`
	}
	err := s.getJSON(ctx, queryURL(s.config.FundNAVURL, url.Values{"fundCode": {b.ProviderID}, "pageIndex": {"1"}, "pageSize": {"1"}}), map[string]string{"Referer": "https://fundf10.eastmoney.com/"}, &response)
	if err != nil {
		return Quote{}, err
	}
	if response.Data.Type == "005" || response.Data.YieldType != "" {
		return Quote{}, errors.New("货币基金的每万份收益不是单位净值，请使用手动估值")
	}
	if response.ErrorCode != 0 || len(response.Data.Rows) != 1 {
		return Quote{}, errors.New("该基金暂无已公布净值")
	}
	row := response.Data.Rows[0]
	zone, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return Quote{}, err
	}
	date, err := time.ParseInLocation("2006-01-02", row.Date, zone)
	if err != nil {
		return Quote{}, err
	}
	// Publication date is a date, not the moment it was downloaded. Midnight
	// expresses that date without inventing a precise intraday NAV timestamp.
	price, valid := positiveDecimal(row.NAV)
	if !valid || date.After(s.config.Now()) {
		return Quote{}, errors.New("基金净值或日期无效")
	}
	return Quote{InstrumentID: b.Key(), Price: price, Currency: "CNY", Source: SourceFundNAV, SourceTime: date.Unix(), ReceivedAt: s.config.Now().Unix(), State: StateDelayed, ChangePercent: signedPercent(row.Change), ChangePeriod: "nav"}, nil
}

func (s *Service) fetchCoinbaseReference(ctx context.Context, b Binding) (*Candidate, Quote, error) {
	var product coinbaseProduct
	err := s.getJSON(ctx, s.config.CoinbaseRESTURL+"/products/"+url.PathEscape(b.ProviderID), nil, &product)
	if err != nil {
		return nil, Quote{}, err
	}
	if product.ProductID != b.ProviderID || product.BaseCurrency+"-USD" != b.ProviderID || product.QuoteCurrency != "USD" || product.ProductType != "SPOT" || product.Status != "online" || product.Disabled || product.TradingDisabled {
		return nil, Quote{}, errors.New("加密现货产品未通过来源核验")
	}
	var ticker struct {
		Trades []struct {
			ID    string `json:"product_id"`
			Price string `json:"price"`
			Time  string `json:"time"`
		} `json:"trades"`
	}
	err = s.getJSON(ctx, s.config.CoinbaseRESTURL+"/products/"+url.PathEscape(b.ProviderID)+"/ticker?limit=1", nil, &ticker)
	if err != nil {
		return nil, Quote{}, err
	}
	if len(ticker.Trades) == 0 || ticker.Trades[0].ID != b.ProviderID {
		return nil, Quote{}, errors.New("缺少对应现货产品的成交报价")
	}
	trade := ticker.Trades[0]
	at, err := time.Parse(time.RFC3339Nano, trade.Time)
	if err != nil {
		return nil, Quote{}, err
	}
	name := product.BaseName
	if name == "" {
		name = product.BaseCurrency
	}
	return &Candidate{Binding: b, Name: name, Symbol: product.BaseCurrency, Type: "CRYPTO"}, Quote{InstrumentID: b.Key(), Price: trade.Price, Currency: "USD", Source: SourceCoinbaseREST, SourceTime: at.Unix(), ReceivedAt: s.config.Now().Unix(), State: StateDelayed, ChangePercent: signedPercent(product.Change24h), ChangePeriod: "24h"}, nil
}

func (s *Service) fetchGeckoReferences(ctx context.Context, bindings []Binding) (map[string]Quote, error) {
	ids := make([]string, 0, len(bindings))
	for _, b := range bindings {
		ids = append(ids, b.ProviderID)
	}
	var response map[string]struct {
		Price  json.Number `json:"usd"`
		Change json.Number `json:"usd_24h_change"`
		Time   int64       `json:"last_updated_at"`
	}
	headers := map[string]string{}
	if s.config.CoinGeckoAPIKey != "" {
		headers["x-cg-demo-api-key"] = s.config.CoinGeckoAPIKey
	}
	err := s.getJSON(ctx, queryURL(s.config.CoinGeckoURL, url.Values{"ids": {strings.Join(ids, ",")}, "vs_currencies": {"usd"}, "include_24hr_change": {"true"}, "include_last_updated_at": {"true"}, "precision": {"full"}}), headers, &response)
	if err != nil {
		return nil, err
	}
	result := make(map[string]Quote)
	for _, b := range bindings {
		p, ok := response[b.ProviderID]
		if !ok {
			continue
		}
		if _, valid := positiveDecimal(string(p.Price)); !valid || p.Time <= 0 {
			continue
		}
		result[b.Key()] = Quote{InstrumentID: b.Key(), Price: string(p.Price), Currency: "USD", Source: SourceCoinGecko, SourceTime: p.Time, ReceivedAt: s.config.Now().Unix(), State: StateDelayed, ChangePercent: signedPercent(string(p.Change)), ChangePeriod: "24h"}
	}
	return result, nil
}

func (s *Service) hkdFXViewLocked() FXRate {
	rate := s.hkdFX
	if rate.Rate == "" {
		return FXRate{Base: "HKD", Quote: "CNY", Source: SourceECB, State: StateUnavailable}
	}
	date, err := time.Parse("2006-01-02", rate.Date)
	rate.State = StateDelayed
	if s.hkdFXRestored || err != nil || s.config.Now().Sub(date) > 7*24*time.Hour || s.config.Now().Sub(time.Unix(rate.ReceivedAt, 0)) > 36*time.Hour {
		rate.State = StateStale
	}
	return rate
}

func (s *Service) hkdFXLoop(ctx context.Context) {
	for ctx.Err() == nil {
		delay := s.config.FXInterval
		if err := s.refreshHKDFX(ctx); err != nil {
			delay = s.config.FXRetryInterval
		}
		if !waitContext(ctx, delay) {
			return
		}
	}
}

func (s *Service) refreshHKDFX(ctx context.Context) error {
	var response struct {
		Date  string      `json:"date"`
		Base  string      `json:"base"`
		Quote string      `json:"quote"`
		Rate  json.Number `json:"rate"`
	}
	if err := s.getJSON(ctx, s.config.HKDFXURL, nil, &response); err != nil {
		return err
	}
	price, valid := positiveDecimal(string(response.Rate))
	date, err := time.Parse("2006-01-02", response.Date)
	if !valid || err != nil || date.After(s.config.Now()) || response.Base != "HKD" || response.Quote != "CNY" {
		return errors.New("invalid ECB HKD reference rate")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hkdFX.Date > response.Date {
		return errors.New("older ECB HKD reference rate")
	}
	s.hkdFX = FXRate{Base: "HKD", Quote: "CNY", Rate: price, Date: response.Date, Source: SourceECB, ReceivedAt: s.config.Now().Unix(), State: StateDelayed}
	s.hkdFXRestored = false
	return nil
}
