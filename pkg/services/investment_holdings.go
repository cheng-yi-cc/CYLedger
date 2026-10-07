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

type HoldingSetupInput struct {
	Profile          models.InvestmentHoldingProfile `json:"profile"`
	Instrument       models.InvestmentInstrument     `json:"instrument"`
	Quantity         string                          `json:"quantity"`
	Cost             *string                         `json:"cost"`
	Price            string                          `json:"price"`
	BookId           string                          `json:"bookId"`
	OccurredAt       int64                           `json:"occurredAt"`
	ExpectedQuantity string                          `json:"expectedQuantity"`
	ExpectedCost     *string                         `json:"expectedCost"`
}

func (s *InvestmentService) UpdateHolding(c core.Context, uid int64, input HoldingSetupInput, key string) (*models.InvestmentHoldingProfile, error) {
	defer s.lock(uid)()
	p := input.Profile
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		if err := validateHoldingProfile(sess, uid, &p); err != nil {
			return err
		}
		old := new(models.InvestmentHoldingProfile)
		has, err := sess.Where("uid=? AND account_id=? AND instrument_id=?", uid, p.AccountId, p.InstrumentId).Get(old)
		if err != nil {
			return err
		}
		if has && old.Version != p.Version || !has && p.Version != 0 {
			return ErrInvestmentConflict
		}
		events, err := readInvestmentEvents(sess, uid)
		if err != nil {
			return err
		}
		result, err := replayInvestments(events)
		if err != nil {
			return err
		}
		quantity := "0"
		cost := decimalPointer(decimal.Zero)
		for _, pos := range result.Positions {
			if pos.AccountID == p.AccountId && pos.InstrumentID == p.InstrumentId {
				quantity = pos.Quantity
				cost = pos.Cost
			}
		}
		equalCost := func(a, b *string) bool {
			if a == nil || b == nil {
				return a == nil && b == nil
			}
			// 成本回放可能返回36位小数；原样回传的并发校验值不应被18位输入限制截断。
			if *a == *b {
				return true
			}
			x, e1 := investmentDecimal(*a, "成本", true)
			y, e2 := investmentDecimal(*b, "成本", true)
			return e1 == nil && e2 == nil && x.Equal(y)
		}
		if quantity != input.ExpectedQuantity || !equalCost(cost, input.ExpectedCost) {
			return ErrInvestmentConflict
		}
		q, err := investmentDecimal(input.Quantity, "份额", true)
		if err != nil {
			return err
		}
		if input.Cost != nil && !equalCost(cost, input.Cost) {
			if _, err = investmentDecimal(*input.Cost, "成本", true); err != nil {
				return err
			}
		}
		e := InvestmentEvent{Event: investments.Event{Type: investments.Adjust, AccountID: p.AccountId, InstrumentID: p.InstrumentId, Quantity: q.String(), Cost: input.Cost, Amount: "0", Fee: "0", ExchangeRate: "1", OccurredAt: input.OccurredAt, Note: "编辑理财持仓"}, BookID: input.BookId}
		if err = s.validateEvent(sess, uid, &e); err != nil {
			return err
		}
		if q.String() != quantity || !equalCost(cost, input.Cost) {
			if _, err = s.mutateInvestmentInSession(c, sess, uid, e, key, "create", false, nil); err != nil {
				return err
			}
		}
		p.Uid = uid
		p.Version++
		if has {
			p.Id = old.Id
			_, err = sess.Where("uid=? AND id=? AND version=?", uid, old.Id, old.Version).AllCols().Update(&p)
		} else {
			p.Id = investmentID()
			_, err = sess.Insert(&p)
		}
		return err
	})
	return &p, err
}

func investmentDecimal(raw, label string, nonnegative bool) (decimal.Decimal, error) {
	if err := investments.ValidateDecimal(raw); err != nil {
		return decimal.Zero, investmentError(label + "格式无效，最多18位小数")
	}
	d, err := decimal.NewFromString(raw)
	if err != nil || (nonnegative && d.IsNegative()) {
		return decimal.Zero, investmentError(label + "不能为负数")
	}
	return d, nil
}

func validateHoldingProfile(sess *xorm.Session, uid int64, p *models.InvestmentHoldingProfile) error {
	p.Name, p.Group = strings.TrimSpace(p.Name), strings.TrimSpace(p.Group)
	if len([]rune(p.Name)) > 64 || len([]rune(p.Group)) > 32 || len(p.Note) > 1000 || len(p.BookIds) > 100 {
		return investmentError("名称、分组或备注过长")
	}
	if p.ProfitOffset == "" {
		p.ProfitOffset = "0"
	}
	if _, err := investmentDecimal(p.ProfitOffset, "盈亏偏差", false); err != nil {
		return err
	}
	for _, id := range p.BookIds {
		if _, err := Books.ResolveInSession(sess, uid, id, true); err != nil {
			return err
		}
	}
	return nil
}

func (s *InvestmentService) HoldingProfiles(c core.Context, uid int64) ([]models.InvestmentHoldingProfile, error) {
	items := []models.InvestmentHoldingProfile{}
	err := s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).Find(&items)
	return items, err
}

func applyHoldingProfiles(out *WealthSummary, profiles []models.InvestmentHoldingProfile) {
	byKey := map[string]*models.InvestmentHoldingProfile{}
	for i := range profiles {
		p := &profiles[i]
		byKey[p.AccountId+":"+p.InstrumentId] = p
	}
	missing, stale := 0, 0
	total := decimal.Zero
	for _, cash := range out.CashAccounts {
		if cash.ExcludedFromTotal {
			continue
		}
		if cash.Value == nil {
			missing++
		}
		if cash.FX != nil && cash.FX.State == "stale" && cash.Balance != "0" {
			stale++
		}
	}
	for i := range out.Positions {
		p := &out.Positions[i]
		p.Profile = byKey[p.AccountID+":"+p.InstrumentID]
		p.TotalValue = p.MarketValue
		if p.Profile != nil && p.Profile.ExcludeFromTotal {
			p.TotalValue = decimalPointer(decimal.Zero)
			continue
		}
		if p.Profile != nil && p.Profile.ExcludeProfit {
			p.TotalValue = p.Cost
		} else if p.Quote != nil && (p.Quote.State == "stale" || p.Quote.FXState == "stale") && p.Quantity != "0" {
			stale++
		}
		if p.TotalValue == nil {
			missing++
		} else {
			d, _ := decimal.NewFromString(*p.TotalValue)
			total = total.Add(d)
		}
	}
	out.InvestmentValue = total.String()
	cash, _ := decimal.NewFromString(out.CashAssets)
	debt, _ := decimal.NewFromString(out.Liabilities)
	value := cash.Sub(debt).Add(total)
	out.ValuedAssets = value.String()
	out.NetAssets = nil
	out.MissingPrices = missing
	out.StalePrices = stale
	if missing == 0 {
		out.NetAssets = decimalPointer(value)
	}
}

func (s *InvestmentService) SaveHoldingProfile(c core.Context, uid int64, p models.InvestmentHoldingProfile) (*models.InvestmentHoldingProfile, error) {
	defer s.lock(uid)()
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		if err := validateHoldingProfile(sess, uid, &p); err != nil {
			return err
		}
		e := InvestmentEvent{Event: investments.Event{AccountID: p.AccountId, InstrumentID: p.InstrumentId, Type: investments.Opening, OccurredAt: time.Now().Unix()}}
		if err := s.validateEvent(sess, uid, &e); err != nil {
			return err
		}
		old := new(models.InvestmentHoldingProfile)
		has, err := sess.Where("uid=? AND account_id=? AND instrument_id=?", uid, p.AccountId, p.InstrumentId).Get(old)
		if err != nil {
			return err
		}
		if has && old.Version != p.Version || !has && p.Version != 0 {
			return ErrInvestmentConflict
		}
		p.Uid = uid
		p.Version++
		if has {
			p.Id = old.Id
			_, err = sess.Where("uid=? AND id=? AND version=?", uid, old.Id, old.Version).AllCols().Update(&p)
		} else {
			p.Id = investmentID()
			_, err = sess.Insert(&p)
		}
		return err
	})
	return &p, err
}

// 一次新增直接保存资产、账户、展示规则和期初事实，任一失败整体回滚。
func (s *InvestmentService) SetupHolding(c core.Context, uid int64, input HoldingSetupInput, key string) (*models.InvestmentHoldingProfile, error) {
	if uid <= 0 || len(key) < 8 || len(key) > 128 {
		return nil, investmentError("请求缺少有效的 Idempotency-Key")
	}
	raw, _ := json.Marshal(input)
	hash := sha256.Sum256(append([]byte("holding:"), raw...))
	digest := hex.EncodeToString(hash[:])
	// 网络身份核验在持有数据库事务前完成。
	asset := input.Instrument
	if input.Profile.InstrumentId == "" {
		asset.Name, asset.Symbol = strings.TrimSpace(asset.Name), strings.TrimSpace(asset.Symbol)
		if asset.Symbol == "" && asset.Type == "OTHER" {
			asset.Symbol = "自定义"
		}
		if len([]rune(asset.Name)) < 1 || len([]rune(asset.Name)) > 64 || len(asset.Symbol) < 1 || len(asset.Symbol) > 24 || (asset.Type != "FUND" && asset.Type != "STOCK" && asset.Type != "OTHER" && asset.Type != "CRYPTO") {
			return nil, investmentError("请填写有效的资产名称、代码和类型")
		}
		if asset.Provider != "" || asset.ProviderID != "" || asset.Market != "" {
			confirmed, err := marketquotes.Default.Resolve(investmentContext(c), instrumentBinding(asset))
			if err != nil {
				return nil, investmentError(err.Error())
			}
			if asset.Type != confirmed.Type && !(asset.Provider == "tencent" && (asset.Type == "STOCK" || asset.Type == "FUND")) {
				return nil, investmentError("资产类型与行情不一致")
			}
		} else if asset.Currency != "" && asset.Currency != "CNY" && asset.Currency != "USD" && asset.Currency != "HKD" {
			return nil, investmentError("请选择人民币、美元或港元")
		}
	}
	q, err := investmentDecimal(input.Quantity, "持有数量", true)
	if err != nil {
		return nil, err
	}
	if input.Cost != nil {
		if _, err = investmentDecimal(*input.Cost, "持仓成本", true); err != nil {
			return nil, err
		}
	}
	if input.OccurredAt <= 0 || input.OccurredAt > time.Now().Unix()+60 {
		return nil, investmentError("请选择有效的持仓日期")
	}
	defer s.lock(uid)()
	var result *models.InvestmentHoldingProfile
	err = s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		old := new(models.InvestmentIdempotency)
		has, err := sess.Where("uid=? AND request_key=?", uid, key).Get(old)
		if err != nil {
			return err
		}
		if has {
			if old.Digest != digest {
				return ErrInvestmentConflict
			}
			result = new(models.InvestmentHoldingProfile)
			return json.Unmarshal([]byte(old.Response), result)
		}
		p := input.Profile
		p.Id = investmentID()
		p.Uid = uid
		p.Version = 1
		if err = validateHoldingProfile(sess, uid, &p); err != nil {
			return err
		}
		if p.InstrumentId == "" {
			asset.Id = "custom:" + investmentID()
			asset.Uid = uid
			asset.Precision = 18
			if _, err = sess.Insert(&asset); err != nil {
				return err
			}
			p.InstrumentId = asset.Id
		} else {
			found := false
			for _, preset := range investmentPresets {
				if preset.Id == p.InstrumentId {
					asset = preset
					found = true
					break
				}
			}
			if !found {
				has, err = sess.Where("uid=? AND id=?", uid, p.InstrumentId).Get(&asset)
				if err != nil {
					return err
				}
				if !has {
					return investmentError("找不到所选资产")
				}
			}
		}
		if p.Name == "" {
			p.Name = asset.Name
		}
		if p.AccountId == "" {
			a := models.PortfolioAccount{Id: investmentID(), Uid: uid, Name: p.Name, Kind: "BROKER", Currency: "CNY", Instruments: []string{p.InstrumentId}}
			if asset.Type == "OTHER" {
				a.Kind = "OTHER"
			}
			if _, err = sess.Insert(&a); err != nil {
				return err
			}
			p.AccountId = a.Id
		}
		e := InvestmentEvent{Event: investments.Event{Type: investments.Opening, AccountID: p.AccountId, InstrumentID: p.InstrumentId, Quantity: q.String(), Cost: input.Cost, Amount: "0", Fee: "0", ExchangeRate: "1", OccurredAt: input.OccurredAt, Note: input.Profile.Note}, BookID: input.BookId}
		if err = s.validateEvent(sess, uid, &e); err != nil {
			return err
		}
		has, err = sess.Where("uid=? AND account_id=? AND instrument_id=?", uid, p.AccountId, p.InstrumentId).Exist(&models.InvestmentHoldingProfile{})
		if err != nil {
			return err
		}
		if has {
			return investmentError("这个账户已有该项理财，请从详情页买入或编辑")
		}
		if q.IsPositive() {
			if _, err = s.mutateInvestmentInSession(c, sess, uid, e, "holding-opening:"+p.Id, "create", false, nil); err != nil {
				return err
			}
		}
		if input.Price != "" {
			price, err := investmentDecimal(input.Price, "价格", true)
			if err != nil {
				return err
			}
			if !price.IsPositive() {
				return investmentError("价格必须大于零")
			}
			currency := asset.Currency
			if currency == "" {
				currency = "CNY"
			}
			quote := InvestmentValuationQuote{Quote: marketquotes.Quote{InstrumentID: p.InstrumentId, Price: price.String(), Currency: currency, Source: "手动估值", SourceTime: input.OccurredAt, ReceivedAt: time.Now().Unix(), State: "manual"}}
			if currency == "CNY" {
				quote.FXRate = "1"
			}
			payload, _ := json.Marshal(quote)
			if _, err = sess.Insert(&models.InvestmentQuote{Id: fmt.Sprintf("%d:%s", uid, p.InstrumentId), Uid: uid, InstrumentId: p.InstrumentId, Payload: string(payload)}); err != nil {
				return err
			}
		}
		if _, err = sess.Insert(&p); err != nil {
			return err
		}
		result = &p
		response, _ := json.Marshal(p)
		_, err = sess.Insert(&models.InvestmentIdempotency{Id: investmentID(), Uid: uid, RequestKey: key, Digest: digest, Response: string(response)})
		return err
	})
	return result, err
}
