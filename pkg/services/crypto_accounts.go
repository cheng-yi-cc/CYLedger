package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/investments"
	"github.com/mayswind/ezbookkeeping/pkg/marketquotes"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/shopspring/decimal"
	"xorm.io/xorm"
)

var cryptoPlatforms = map[string]string{
	"binance": "EXCHANGE", "okx": "EXCHANGE", "coinbase": "EXCHANGE", "kraken": "EXCHANGE", "bybit": "EXCHANGE", "bitget": "EXCHANGE",
	"metamask": "WALLET", "trust": "WALLET", "phantom": "WALLET", "rabby": "WALLET", "ledger": "WALLET", "trezor": "WALLET",
}

type CryptoHoldingInput struct {
	InstrumentID string `json:"instrumentId"`
	Quantity     string `json:"quantity"`
}
type CryptoAccountInput struct {
	Name     string               `json:"name"`
	Kind     string               `json:"kind"`
	Platform string               `json:"platform"`
	BookID   string               `json:"bookId"`
	Holdings []CryptoHoldingInput `json:"holdings"`
}

// UpdatePortfolioAccount edits account metadata without manufacturing or revising holdings.
func (s *InvestmentService) UpdatePortfolioAccount(c core.Context, uid int64, input models.PortfolioAccount) (*models.PortfolioAccount, error) {
	input.Name = strings.TrimSpace(input.Name)
	if uid <= 0 || len(input.Id) == 0 || len(input.Id) > 64 || len([]rune(input.Name)) == 0 || len([]rune(input.Name)) > 64 || len(input.Instruments) > 100 || len(input.Platform) > 32 {
		return nil, investmentError("请填写有效的账户信息")
	}
	defer s.lock(uid)()
	var result models.PortfolioAccount
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		if _, err := sess.Where("uid=?", uid).Incr("revision", 1).Update(&models.InvestmentSettings{}); err != nil {
			return err
		}
		has, err := sess.Where("uid=? AND id=?", uid, input.Id).Get(&result)
		if err != nil {
			return err
		}
		if !has {
			return investmentError("找不到这个投资账户")
		}
		if input.Kind != result.Kind {
			return investmentError("已有账户的类型不能更改")
		}
		result.Name = input.Name
		if result.Kind == "EXCHANGE" || result.Kind == "WALLET" {
			if cryptoPlatforms[input.Platform] != result.Kind {
				return investmentError("请选择对应的交易所或钱包")
			}
			known := map[string]bool{}
			for _, v := range investmentPresets {
				known[v.Id] = v.Type == "CRYPTO"
			}
			var own []models.InvestmentInstrument
			if err = sess.Where("uid=?", uid).Find(&own); err != nil {
				return err
			}
			for _, v := range own {
				known[v.Id] = v.Type == "CRYPTO"
			}
			seen := map[string]bool{}
			for _, id := range input.Instruments {
				if len(id) > 100 || !known[id] || seen[id] {
					return investmentError("请选择不重复的加密货币")
				}
				seen[id] = true
			}
			events, err := readInvestmentEvents(sess, uid)
			if err != nil {
				return err
			}
			positions, err := replayInvestments(events)
			if err != nil {
				return err
			}
			for _, p := range positions.Positions {
				quantity, err := decimal.NewFromString(p.Quantity)
				if err != nil {
					return err
				}
				if p.AccountID == result.Id && quantity.IsPositive() && !seen[p.InstrumentID] {
					return investmentError("持有中的币种不能移除，请先处理持仓")
				}
			}
			result.Platform, result.Instruments = input.Platform, input.Instruments
		}
		_, err = sess.Where("uid=? AND id=?", uid, result.Id).Cols("name", "platform", "instruments").Update(&result)
		return err
	})
	return &result, err
}

// CreateCryptoAccount saves the account, selected coins and all initial quantities together.
// Empty/zero quantities select a coin without fabricating an opening transaction or cost.
func (s *InvestmentService) CreateCryptoAccount(c core.Context, uid int64, input CryptoAccountInput, key string) (*models.PortfolioAccount, error) {
	if uid <= 0 || len(key) < 8 || len(key) > 128 {
		return nil, investmentError("请求缺少有效的 Idempotency-Key")
	}
	input.Name = strings.TrimSpace(input.Name)
	if len([]rune(input.Name)) == 0 || len([]rune(input.Name)) > 64 || len(input.Holdings) > 100 || (input.Kind != "EXCHANGE" && input.Kind != "WALLET") || cryptoPlatforms[input.Platform] != input.Kind {
		return nil, investmentError("请选择交易所或钱包，并填写账户名称")
	}
	raw, _ := json.Marshal(input)
	hash := sha256.Sum256(append([]byte("crypto-account:"), raw...))
	digest := hex.EncodeToString(hash[:])
	defer s.lock(uid)()
	var result *models.PortfolioAccount
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		// Serialize with investment mutations before reading any facts.
		if _, err := sess.Where("uid=?", uid).Incr("revision", 1).Update(&models.InvestmentSettings{}); err != nil {
			return err
		}
		previous := new(models.InvestmentIdempotency)
		has, err := sess.Where("uid=? AND request_key=?", uid, key).Get(previous)
		if err != nil {
			return err
		}
		if has {
			if previous.Digest != digest {
				return ErrInvestmentConflict
			}
			result = new(models.PortfolioAccount)
			return json.Unmarshal([]byte(previous.Response), result)
		}
		bookID, err := Books.ResolveInSession(sess, uid, input.BookID, false)
		if err != nil {
			return err
		}
		known := map[string]bool{}
		for _, v := range investmentPresets {
			known[v.Id] = v.Type == "CRYPTO"
		}
		var own []models.InvestmentInstrument
		if err = sess.Where("uid=?", uid).Find(&own); err != nil {
			return err
		}
		for _, v := range own {
			known[v.Id] = v.Type == "CRYPTO"
		}
		result = &models.PortfolioAccount{Id: investmentID(), Uid: uid, Name: input.Name, Kind: input.Kind, Platform: input.Platform, Instruments: []string{}}
		seen := map[string]bool{}
		for _, h := range input.Holdings {
			if !known[h.InstrumentID] || seen[h.InstrumentID] {
				return investmentError("请选择不重复的加密货币")
			}
			seen[h.InstrumentID] = true
			result.Instruments = append(result.Instruments, h.InstrumentID)
		}
		if _, err = sess.Insert(result); err != nil {
			return err
		}
		events, err := readInvestmentEvents(sess, uid)
		if err != nil {
			return err
		}
		now := time.Now().Unix()
		for _, h := range input.Holdings {
			if h.Quantity == "" {
				continue
			}
			if err = investments.ValidateDecimal(h.Quantity); err != nil {
				return investmentError("数量格式无效，最多支持 18 位小数")
			}
			q, _ := decimal.NewFromString(h.Quantity)
			if q.IsNegative() {
				return investmentError("持有数量不能为负数")
			}
			if q.IsZero() {
				continue
			}
			event := InvestmentEvent{Event: investments.Event{ID: fmt.Sprintf("%020d-%s", time.Now().UnixNano(), investmentID()), Version: 1, Type: "OPENING", AccountID: result.Id, InstrumentID: h.InstrumentID, Quantity: q.String(), Amount: "0", Fee: "0", ExchangeRate: "1", OccurredAt: now, Note: "录入已有加密货币持仓"}, BookID: bookID}
			if err = s.validateEvent(sess, uid, &event); err != nil {
				return err
			}
			events = append(events, event)
			payload, _ := json.Marshal(event)
			if _, err = sess.Insert(&models.InvestmentEventRecord{Id: event.ID, Uid: uid, OccurredAt: now, Version: 1, Payload: string(payload)}); err != nil {
				return err
			}
			if _, err = sess.Insert(&models.InvestmentEventRevision{Id: investmentID(), Uid: uid, EventId: event.ID, Version: 1, RecordedAt: now, Payload: string(payload)}); err != nil {
				return err
			}
		}
		if _, err = replayInvestments(events); err != nil {
			return investmentError(err.Error())
		}
		if err = InvalidateWealthSnapshots(sess, uid, now); err != nil {
			return err
		}
		response, _ := json.Marshal(result)
		_, err = sess.Insert(&models.InvestmentIdempotency{Id: investmentID(), Uid: uid, RequestKey: key, Digest: digest, Response: string(response)})
		return err
	})
	return result, err
}

type ConversionInput struct {
	FromInstrumentID string `json:"fromInstrumentId"` // Empty denotes CNY cash.
	ToInstrumentID   string `json:"toInstrumentId"`   // Empty denotes CNY cash.
	FromQuantity     string `json:"fromQuantity"`
}
type InvestmentConversion struct {
	ConversionInput
	ToQuantity string                    `json:"toQuantity"`
	FromPrice  string                    `json:"fromPrice"`
	ToPrice    string                    `json:"toPrice"`
	ObservedAt int64                     `json:"observedAt"`
	ExpiresAt  int64                     `json:"expiresAt"`
	FromQuote  *InvestmentValuationQuote `json:"fromQuote,omitempty"`
	ToQuote    *InvestmentValuationQuote `json:"toQuote,omitempty"`
}

func conversionFromQuotes(input ConversionInput, quotes []InvestmentValuationQuote, now int64) (*InvestmentConversion, error) {
	if input.FromInstrumentID == input.ToInstrumentID {
		return nil, investmentError("请选择不同的转出和转入币种")
	}
	if err := investments.ValidateDecimal(input.FromQuantity); err != nil {
		return nil, investmentError("请填写有效的转出金额或数量")
	}
	quantity, _ := decimal.NewFromString(input.FromQuantity)
	if !quantity.IsPositive() {
		return nil, investmentError("转出金额或数量必须大于零")
	}
	if input.FromInstrumentID == "" && quantity.Exponent() < -2 {
		return nil, investmentError("人民币金额最多支持两位小数")
	}
	lookup := func(id string) (decimal.Decimal, *InvestmentValuationQuote, error) {
		if id == "" {
			return decimal.NewFromInt(1), nil, nil
		}
		for _, q := range quotes {
			if q.InstrumentID != id {
				continue
			}
			if (q.State != marketquotes.StateLive && q.State != marketquotes.StateDelayed) || q.SourceTime <= 0 || q.SourceTime > now+120 || (q.State != marketquotes.StateLive && now-q.SourceTime > 300) || q.FXState == marketquotes.StateStale {
				break
			}
			price, e := decimal.NewFromString(q.Price)
			if e != nil || !price.IsPositive() {
				break
			}
			fx, e := decimal.NewFromString(q.FXRate)
			if e != nil || !fx.IsPositive() {
				break
			}
			if q.Currency != "CNY" && (q.FXDate == "" || q.FXSource == "" || q.FXState == marketquotes.StateUnavailable) {
				break
			}
			return price.Mul(fx), &q, nil
		}
		return decimal.Zero, nil, investmentError("暂时没有可用的最新行情或人民币汇率，请稍后重试")
	}
	from, fq, err := lookup(input.FromInstrumentID)
	if err != nil {
		return nil, err
	}
	to, tq, err := lookup(input.ToInstrumentID)
	if err != nil {
		return nil, err
	}
	precision := int32(18)
	if input.ToInstrumentID == "" {
		precision = 2
	}
	received := quantity.Mul(from).DivRound(to, 36).Truncate(precision)
	if !received.IsPositive() {
		return nil, investmentError("兑换数量过小，请增加转出金额")
	}
	return &InvestmentConversion{ConversionInput: input, ToQuantity: received.String(), FromPrice: from.Round(18).String(), ToPrice: to.Round(18).String(), ObservedAt: now, ExpiresAt: now + 120, FromQuote: fq, ToQuote: tq}, nil
}
func (s *InvestmentService) Conversion(c core.Context, uid int64, input ConversionInput) (*InvestmentConversion, error) {
	items, err := s.Instruments(c, uid)
	if err != nil {
		return nil, err
	}
	known := map[string]bool{"": true}
	for _, item := range items {
		known[item.Id] = item.Type == "CRYPTO"
	}
	if !known[input.FromInstrumentID] || !known[input.ToInstrumentID] {
		return nil, investmentError("请选择当前账本中的加密货币")
	}
	ids := []string{}
	bindings := []marketquotes.Binding{}
	for _, id := range []string{input.FromInstrumentID, input.ToInstrumentID} {
		if id == "" {
			continue
		}
		key := id
		for _, item := range items {
			if item.Id == id && item.Provider != "" {
				b := instrumentBinding(item)
				bindings = append(bindings, b)
				key = b.Key()
			}
		}
		ids = append(ids, key)
	}
	marketquotes.Default.RefreshCryptoConversion(investmentContext(c), ids, bindings)
	quotes, err := s.Quotes(c, uid)
	if err != nil {
		return nil, err
	}
	return conversionFromQuotes(input, quotes, time.Now().Unix())
}
