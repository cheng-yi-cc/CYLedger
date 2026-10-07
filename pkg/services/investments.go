package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/investments"
	"github.com/mayswind/ezbookkeeping/pkg/marketquotes"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
	"github.com/shopspring/decimal"
	"xorm.io/xorm"
)

const InvestmentSettlementRole = "investment_settlement"

var ErrInvestmentLinked = errs.NewNormalError(21, 1, 409, "此流水属于投资结算，请在资产页修订或撤销对应投资记录")
var ErrInvestmentConflict = errs.NewNormalError(21, 2, 409, "记录已变更，或重复请求的内容不同，请刷新后重试")

type InvestmentEvent struct {
	investments.Event
	Wallet        *WalletEntry          `json:"wallet,omitempty"`
	CashAccountID string                `json:"cashAccountId"`
	BookID        string                `json:"bookId,omitempty"`
	Conversion    *InvestmentConversion `json:"conversion,omitempty"`
	DCA           *CryptoDCAInfo        `json:"dca,omitempty"`
}
type InvestmentPreview struct {
	Event     InvestmentEvent           `json:"event"`
	Positions []investments.Position    `json:"positions"`
	Effects   []investments.EventEffect `json:"effects"`
}
type InvestmentService struct {
	ServiceUsingDB
	locks        [64]sync.Mutex
	dcaSyncLocks [64]sync.Mutex
}

var Investments = &InvestmentService{ServiceUsingDB: ServiceUsingDB{container: datastore.Container}}

func investmentID() string {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b[:])
}
func investmentError(message string) error { return errs.NewNormalError(21, 3, 400, message) }
func (s *InvestmentService) lock(uid int64) func() {
	m := &s.locks[uint64(uid)%64]
	m.Lock()
	return m.Unlock
}

var investmentPresets = []models.InvestmentInstrument{
	{Id: "crypto:bitcoin", Type: "CRYPTO", Symbol: "BTC", Name: "比特币", Precision: 18},
	{Id: "crypto:ethereum", Type: "CRYPTO", Symbol: "ETH", Name: "以太坊", Precision: 18},
	{Id: "crypto:solana", Type: "CRYPTO", Symbol: "SOL", Name: "Solana", Precision: 18},
	{Id: "crypto:tether", Type: "CRYPTO", Symbol: "USDT", Name: "Tether", Precision: 18},
	{Id: "crypto:usd-coin", Type: "CRYPTO", Symbol: "USDC", Name: "USD Coin", Precision: 18},
}

func (s *InvestmentService) Settings(c core.Context, uid int64) (*models.InvestmentSettings, error) {
	item := &models.InvestmentSettings{Uid: uid, BaseCurrency: "CNY", TimeZone: ""}
	_, err := s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).Get(item)
	return item, err
}
func (s *InvestmentService) SaveSettings(c core.Context, uid int64, item models.InvestmentSettings) (*models.InvestmentSettings, error) {
	if item.BaseCurrency != "CNY" {
		return nil, investmentError("初版以人民币为本位币")
	}
	if _, err := time.LoadLocation(item.TimeZone); err != nil || item.TimeZone == "" {
		return nil, investmentError("请选择有效的会计时区")
	}
	defer s.lock(uid)()
	item.Uid = uid
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		old := new(models.InvestmentSettings)
		has, err := sess.Where("uid=?", uid).Get(old)
		if err != nil {
			return err
		}
		if !has {
			_, err = sess.Insert(&item)
		} else {
			_, err = sess.Where("uid=?", uid).Cols("base_currency", "time_zone").Update(&item)
		}
		return err
	})
	return &item, err
}
func (s *InvestmentService) Instruments(c core.Context, uid int64) ([]models.InvestmentInstrument, error) {
	items := append([]models.InvestmentInstrument{}, investmentPresets...)
	var own []models.InvestmentInstrument
	err := s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).Find(&own)
	for _, item := range own {
		if item.Provider != "" {
			_ = marketquotes.Default.Register(instrumentBinding(item))
		}
	}
	return append(items, own...), err
}

func instrumentBinding(item models.InvestmentInstrument) marketquotes.Binding {
	return marketquotes.Binding{Market: item.Market, Provider: item.Provider, ProviderID: item.ProviderID, Currency: item.Currency}
}

func investmentContext(c core.Context) context.Context {
	if c != nil {
		return c
	}
	return context.Background()
}

func (s *InvestmentService) SearchInstruments(c core.Context, query, market string) ([]marketquotes.Candidate, error) {
	result, err := marketquotes.Default.Search(investmentContext(c), query, market)
	if err != nil {
		return nil, investmentError(err.Error())
	}
	return result, nil
}

func (s *InvestmentService) BindInstrument(c core.Context, uid int64, id string, binding marketquotes.Binding) (*models.InvestmentInstrument, error) {
	item := new(models.InvestmentInstrument)
	sess := s.UserDataDB(uid).NewSession(c)
	defer sess.Close()
	has, err := sess.Where("uid=? AND id=?", uid, id).Get(item)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, investmentError("请选择属于当前账本的自定义资产")
	}
	confirmed, err := marketquotes.Default.Resolve(investmentContext(c), binding)
	if err != nil {
		return nil, investmentError(err.Error())
	}
	if item.Type != confirmed.Type && !(binding.Provider == "tencent" && (item.Type == "FUND" || item.Type == "STOCK") && (confirmed.Type == "FUND" || confirmed.Type == "STOCK")) {
		return nil, investmentError("资产类型与所选行情不一致")
	}
	defer s.lock(uid)()
	item.Market, item.Provider, item.ProviderID, item.Currency = binding.Market, binding.Provider, binding.ProviderID, binding.Currency
	_, err = sess.Where("uid=? AND id=?", uid, id).Cols("market", "provider", "provider_id", "currency").Update(item)
	return item, err
}

func (s *InvestmentService) CreateInstrument(c core.Context, uid int64, item models.InvestmentInstrument) (*models.InvestmentInstrument, error) {
	if item.Type != "CRYPTO" && item.Type != "STOCK" && item.Type != "FUND" && item.Type != "OTHER" {
		return nil, investmentError("资产类型无效")
	}
	item.Name = strings.TrimSpace(item.Name)
	item.Symbol = strings.TrimSpace(item.Symbol)
	if len([]rune(item.Name)) == 0 || len([]rune(item.Name)) > 64 || len(item.Symbol) == 0 || len(item.Symbol) > 24 {
		return nil, investmentError("请填写有效的资产名称与代码")
	}
	if item.Provider != "" || item.ProviderID != "" || item.Market != "" || item.Currency != "" {
		confirmed, err := marketquotes.Default.Resolve(investmentContext(c), instrumentBinding(item))
		if err != nil {
			return nil, investmentError(err.Error())
		}
		if item.Type != confirmed.Type && !(confirmed.Provider == "tencent" && (item.Type == "STOCK" || item.Type == "FUND")) {
			return nil, investmentError("资产类型与所选行情不一致")
		}
	}
	item.Id = "custom:" + investmentID()
	item.Uid = uid
	item.Precision = 18
	_, err := s.UserDataDB(uid).NewSession(c).Insert(&item)
	return &item, err
}
func (s *InvestmentService) Accounts(c core.Context, uid int64) ([]models.PortfolioAccount, error) {
	items := make([]models.PortfolioAccount, 0)
	err := s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).Find(&items)
	return items, err
}
func (s *InvestmentService) CreateAccount(c core.Context, uid int64, item models.PortfolioAccount) (*models.PortfolioAccount, error) {
	// Platform and initial crypto selections are saved through the atomic setup endpoint.
	item.Platform, item.Instruments = "", nil
	item.PaymentInstruments = nil
	if !validatePortfolioCurrency(item.Currency) {
		return nil, investmentError("账户计量币种无效")
	}
	if item.Currency == "" {
		item.Currency = "CNY"
	}
	item.Name = strings.TrimSpace(item.Name)
	if len([]rune(item.Name)) == 0 || len([]rune(item.Name)) > 64 || len(item.Kind) > 32 {
		return nil, investmentError("请填写有效的投资账户名称")
	}
	item.Id = investmentID()
	item.Uid = uid
	if item.Kind == "" {
		item.Kind = "OTHER"
	}
	_, err := s.UserDataDB(uid).NewSession(c).Insert(&item)
	return &item, err
}
func readInvestmentEvents(sess *xorm.Session, uid int64) ([]InvestmentEvent, error) {
	var rows []models.InvestmentEventRecord
	err := sess.Where("uid=?", uid).OrderBy("occurred_at asc,id asc").Find(&rows)
	if err != nil {
		return nil, err
	}
	events := make([]InvestmentEvent, 0, len(rows))
	for _, r := range rows {
		var e InvestmentEvent
		if err = json.Unmarshal([]byte(r.Payload), &e); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}
func (s *InvestmentService) Events(c core.Context, uid int64) ([]InvestmentEvent, error) {
	sess := s.UserDataDB(uid).NewSession(c)
	defer sess.Close()
	return readInvestmentEvents(sess, uid)
}
func replayInvestments(events []InvestmentEvent) (*investments.Result, error) {
	facts := make([]investments.Event, 0, len(events))
	for _, e := range events {
		facts = append(facts, e.Event)
	}
	return investments.Replay(facts)
}
func (s *InvestmentService) validateEvent(sess *xorm.Session, uid int64, e *InvestmentEvent) error {
	if e.Type == investments.Income || e.Type == investments.Expense {
		return s.validateWalletEntry(sess, uid, e)
	}
	if e.Wallet != nil {
		return investmentError("普通投资记录不能携带钱包收支信息")
	}
	if len(e.Note) > 1000 {
		return investmentError("备注过长")
	}
	if e.OccurredAt <= 0 || e.OccurredAt > time.Now().Unix()+60 {
		return investmentError("投资发生时间不能在未来")
	}
	for _, id := range []string{e.AccountID, e.ToAccountID, e.SettlementAccountID} {
		if id == "" {
			continue
		}
		has, err := sess.Where("uid=? AND id=?", uid, id).Exist(&models.PortfolioAccount{})
		if err != nil {
			return err
		}
		if !has {
			return investmentError("投资账户不存在或不属于当前用户")
		}
	}
	for _, id := range []string{e.InstrumentID, e.SettlementInstrumentID} {
		if id == "" {
			continue
		}
		known := false
		for _, preset := range investmentPresets {
			if id == preset.Id {
				known = true
				break
			}
		}
		if !known {
			has, err := sess.Where("uid=? AND id=?", uid, id).Exist(&models.InvestmentInstrument{})
			if err != nil {
				return err
			}
			if !has {
				return investmentError("资产不存在或不属于当前用户")
			}
		}
	}
	if e.Type == "BUY" || e.Type == "SELL" {
		if e.SettlementInstrumentID != "" {
			if e.CashAccountID != "" {
				return investmentError("资产结算不能同时选择资金账户")
			}
		} else {
			id, err := strconv.ParseInt(e.CashAccountID, 10, 64)
			if err != nil || id <= 0 {
				return investmentError("请选择实际结算资金账户")
			}
			acc := new(models.Account)
			has, err := sess.Where("uid=? AND account_id=? AND deleted=? AND system_role=?", uid, id, false, "").Get(acc)
			if err != nil {
				return err
			}
			if !has || acc.Hidden || acc.Type != models.ACCOUNT_TYPE_SINGLE_ACCOUNT {
				return investmentError("结算资金账户不可用")
			}
			if acc.Currency == "CNY" {
				if e.ExchangeRate != "" && e.ExchangeRate != "1" {
					return investmentError("人民币结算汇率必须为 1")
				}
				e.ExchangeRate = "1"
			}
			for _, v := range []string{e.Amount, e.Fee} {
				if v == "" {
					continue
				}
				if err := investments.ValidateDecimal(v); err != nil {
					return investmentError("结算金额必须为普通十进制字符串，最多 18 位小数")
				}
				d, err := decimal.NewFromString(v)
				if err != nil || d.Exponent() < -2 {
					return investmentError("资金账户结算金额最多支持两位小数")
				}
				if d.Abs().GreaterThan(decimal.New(999999999999999, -2)) {
					return investmentError("结算金额超出账户支持范围")
				}
			}
		}
	} else if e.CashAccountID != "" {
		return investmentError("已有持仓或内部转移不会扣除资金账户，请清空结算账户")
	}
	return nil
}

// Mutate atomically validates replay, settles cash, stores facts/audit and invalidates history.
// SQLite's settings-row write is acquired before reading holdings; unique idempotency protects retries.
func (s *InvestmentService) Mutate(c core.Context, uid int64, input InvestmentEvent, key, operation string, preview bool) (*InvestmentPreview, error) {
	return s.mutate(c, uid, input, key, operation, preview, nil)
}

func (s *InvestmentService) mutate(c core.Context, uid int64, input InvestmentEvent, key, operation string, preview bool, dcaDay *models.CryptoDCADay) (*InvestmentPreview, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	defer s.lock(uid)()
	if operation == "create" && input.DCA != nil && dcaDay == nil {
		return nil, investmentError("定投来源信息只能由已启用的计划生成")
	}
	if !preview && operation == "create" && (len(key) < 8 || len(key) > 128) {
		return nil, investmentError("请求缺少有效的 Idempotency-Key")
	}
	raw, _ := json.Marshal(input)
	hash := sha256.Sum256(append([]byte(operation+":"), raw...))
	digest := hex.EncodeToString(hash[:])
	var response *InvestmentPreview
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		settings := &models.InvestmentSettings{}
		has, err := sess.Where("uid=?", uid).Get(settings)
		if err != nil {
			return err
		}
		if !has {
			settings = &models.InvestmentSettings{Uid: uid, BaseCurrency: "CNY", TimeZone: "Asia/Shanghai"}
			if _, err = sess.Insert(settings); err != nil {
				return err
			}
		}
		if _, err = sess.Where("uid=?", uid).Incr("revision", 1).Update(&models.InvestmentSettings{}); err != nil {
			return err
		}
		if !preview && operation == "create" {
			record := new(models.InvestmentIdempotency)
			has, err := sess.Where("uid=? AND request_key=?", uid, key).Get(record)
			if err != nil {
				return err
			}
			if has {
				if record.Digest != digest {
					return ErrInvestmentConflict
				}
				response = new(InvestmentPreview)
				if err := json.Unmarshal([]byte(record.Response), response); err != nil {
					return err
				}
				if response.Event.BookID == "" {
					response.Event.BookID = DefaultBookID(uid)
				}
				return nil
			}
		}
		events, err := readInvestmentEvents(sess, uid)
		if err != nil {
			return err
		}
		if dcaDay != nil {
			if err = validateCryptoDCAExecution(sess, uid, dcaDay, events); err != nil {
				return err
			}
		}
		index := -1
		var old InvestmentEvent
		if operation != "create" {
			for i, e := range events {
				if e.ID == input.ID {
					index = i
					old = e
					break
				}
			}
			if index < 0 {
				return investmentError("投资记录不存在")
			}
			if old.Version != input.Version || old.Voided {
				return ErrInvestmentConflict
			}
		}
		if operation == "void" {
			input = old
			input.Voided = true
		} else {
			if index >= 0 && ((old.Wallet != nil) != (input.Wallet != nil)) {
				return investmentError("不能在钱包收支和投资交易之间转换记录")
			}
			input.Voided = false
			if operation == "revise" {
				// A manual correction is not a new automatic market conversion.
				// The original observation remains in the previous audit revision.
				input.Conversion = nil
				input.DCA = old.DCA
			}
			if input.Conversion != nil && operation == "create" {
				quote := input.Conversion
				now := time.Now().Unix()
				if quote.ObservedAt > now+5 || quote.ExpiresAt < now || quote.ExpiresAt > quote.ObservedAt+120 || quote.ObservedAt < now-120 {
					return investmentError("换算行情已过期，请刷新行情后重新确认")
				}
			}
			if index >= 0 && input.BookID == "" {
				input.BookID = old.BookID
			}
			input.BookID, err = Books.ResolveInSession(sess, uid, input.BookID, index >= 0 && input.BookID == old.BookID)
			if err != nil {
				return err
			}
			if err = s.validateEvent(sess, uid, &input); err != nil {
				return err
			}
		}
		if index < 0 {
			input.ID = fmt.Sprintf("%020d-%s", time.Now().UnixNano(), investmentID())
			input.Version = 1
			events = append(events, input)
		} else {
			input.Version = old.Version + 1
			events[index] = input
		}
		result, err := replayInvestments(events)
		if err != nil {
			return investmentError(err.Error())
		}
		response = &InvestmentPreview{Event: input, Positions: result.Positions, Effects: result.Effects}
		// Preview executes settlement in a rolled-back transaction too, so balance limits are checked.
		if index >= 0 {
			if err = s.reverseSettlement(sess, uid, old.ID); err != nil {
				return err
			}
		}
		if !input.Voided && (input.Type == "BUY" || input.Type == "SELL") && input.SettlementInstrumentID == "" {
			var delta string
			for _, effect := range result.Effects {
				if effect.EventID == input.ID {
					delta = effect.CashDelta
					break
				}
			}
			if err = s.settleCash(c, sess, uid, input, delta, settings.TimeZone); err != nil {
				return err
			}
		}
		if preview {
			if !input.Voided && input.Wallet != nil {
				if err = s.settleWallet(c, sess, uid, input, settings.TimeZone); err != nil {
					return err
				}
			}
			return errInvestmentPreviewRollback
		}
		if !input.Voided && input.Wallet != nil {
			if err = s.settleWallet(c, sess, uid, input, settings.TimeZone); err != nil {
				return err
			}
		}
		payload, _ := json.Marshal(input)
		record := &models.InvestmentEventRecord{Id: input.ID, Uid: uid, OccurredAt: input.OccurredAt, Version: input.Version, Voided: input.Voided, Payload: string(payload)}
		if index < 0 {
			_, err = sess.Insert(record)
		} else {
			var n int64
			n, err = sess.Where("id=? AND uid=? AND version=?", input.ID, uid, old.Version).AllCols().Update(record)
			if err == nil && n != 1 {
				return ErrInvestmentConflict
			}
		}
		if err != nil {
			return err
		}
		if _, err = sess.Insert(&models.InvestmentEventRevision{Id: investmentID(), Uid: uid, EventId: input.ID, Version: input.Version, RecordedAt: time.Now().Unix(), Payload: string(payload)}); err != nil {
			return err
		}
		from := input.OccurredAt
		if index >= 0 && old.OccurredAt < from {
			from = old.OccurredAt
		}
		if err = InvalidateWealthSnapshots(sess, uid, from); err != nil {
			return err
		}
		if dcaDay != nil {
			if err = completeCryptoDCADay(sess, uid, dcaDay, input.ID); err != nil {
				return err
			}
		}
		if operation == "create" {
			serialized, _ := json.Marshal(response)
			_, err = sess.Insert(&models.InvestmentIdempotency{Id: investmentID(), Uid: uid, RequestKey: key, Digest: digest, Response: string(serialized)})
		}
		return err
	})
	if err == errInvestmentPreviewRollback {
		return response, nil
	}
	return response, err
}

var errInvestmentPreviewRollback = fmt.Errorf("investment preview rollback")

func (s *InvestmentService) settleCash(c core.Context, sess *xorm.Session, uid int64, e InvestmentEvent, deltaText, zone string) error {
	delta, err := decimal.NewFromString(deltaText)
	if err != nil {
		return err
	}
	if delta.IsZero() {
		return nil
	}
	id, _ := strconv.ParseInt(e.CashAccountID, 10, 64)
	cash := new(models.Account)
	has, err := sess.Where("uid=? AND account_id=? AND deleted=? AND system_role=?", uid, id, false, "").Get(cash)
	if err != nil {
		return err
	}
	if !has {
		return investmentError("资金账户不存在")
	}
	minor := delta.Shift(2)
	if !minor.Equal(minor.Truncate(0)) || minor.Abs().GreaterThan(decimal.NewFromInt(models.MaximumTransactionAmount)) {
		return investmentError("结算金额超出精度或范围")
	}
	if cash.Category.IsAsset() && decimal.NewFromInt(cash.Balance).Add(minor).IsNegative() {
		return investmentError("资金账户余额不足")
	}
	system := new(models.Account)
	has, err = sess.Where("uid=? AND currency=? AND system_role=? AND deleted=?", uid, cash.Currency, InvestmentSettlementRole, false).Get(system)
	if err != nil {
		return err
	}
	if !has {
		system = &models.Account{AccountId: Accounts.GenerateUuid(uuid.UUID_TYPE_ACCOUNT), Uid: uid, Category: models.ACCOUNT_CATEGORY_VIRTUAL, Type: models.ACCOUNT_TYPE_SINGLE_ACCOUNT, Name: "投资结算（系统）", Currency: cash.Currency, SystemRole: InvestmentSettlementRole, Icon: 1, Color: "387F79", CreatedUnixTime: time.Now().Unix()}
		if system.AccountId <= 0 {
			return errs.ErrSystemIsBusy
		}
		if _, err = sess.Insert(system); err != nil {
			return err
		}
	}
	from, to := cash.AccountId, system.AccountId
	if delta.IsPositive() {
		from, to = to, from
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return err
	}
	_, offset := time.Unix(e.OccurredAt, 0).In(loc).Zone()
	tx := &models.Transaction{Uid: uid, BookId: e.BookID, Type: models.TRANSACTION_DB_TYPE_TRANSFER_OUT, AccountId: from, RelatedAccountId: to, Amount: minor.Abs().IntPart(), RelatedAccountAmount: minor.Abs().IntPart(), TransactionTime: utils.GetMinTransactionTimeFromUnixTime(e.OccurredAt), TimezoneUtcOffset: int16(offset / 60), Comment: "投资" + map[string]string{"BUY": "买入", "SELL": "卖出"}[e.Type], InvestmentEventId: e.ID}
	if err = Transactions.createTransactionInSession(c, sess, tx, nil, nil); err != nil {
		return err
	}
	_, err = sess.Insert(&models.InvestmentTransactionLink{Uid: uid, EventId: e.ID, TransactionId: tx.TransactionId}, &models.InvestmentTransactionLink{Uid: uid, EventId: e.ID, TransactionId: tx.RelatedId})
	return err
}
func (s *InvestmentService) reverseSettlement(sess *xorm.Session, uid int64, eventID string) error {
	var txs []models.Transaction
	if err := sess.Where("uid=? AND investment_event_id=? AND deleted=?", uid, eventID, false).Find(&txs); err != nil {
		return err
	}
	for _, tx := range txs {
		delta := tx.Amount
		if tx.Type == models.TRANSACTION_DB_TYPE_TRANSFER_IN || tx.Type == models.TRANSACTION_DB_TYPE_INCOME {
			delta = -delta
		}
		if _, err := sess.Where("uid=? AND account_id=?", uid, tx.AccountId).Incr("balance", delta).Update(&models.Account{}); err != nil {
			return err
		}
	}
	_, err := sess.Where("uid=? AND investment_event_id=? AND deleted=?", uid, eventID, false).Cols("deleted", "deleted_unix_time").Update(&models.Transaction{Deleted: true, DeletedUnixTime: time.Now().Unix()})
	return err
}

// Account deletion retains audit revisions while voiding all related facts in
// one transaction, after the caller validates the remaining investment replay.
func (s *InvestmentService) voidAccountInvestments(sess *xorm.Session, uid int64, events []InvestmentEvent, ids map[string]bool) error {
	for _, event := range events {
		if !ids[event.ID] {
			continue
		}
		oldVersion := event.Version
		event.Version++
		payload, err := json.Marshal(event)
		if err != nil {
			return err
		}
		n, err := sess.Where("uid=? AND id=? AND version=?", uid, event.ID, oldVersion).Cols("version", "voided", "payload").Update(&models.InvestmentEventRecord{Version: event.Version, Voided: true, Payload: string(payload)})
		if err != nil {
			return err
		}
		if n != 1 {
			return ErrInvestmentConflict
		}
		if _, err := sess.Insert(&models.InvestmentEventRevision{Id: investmentID(), Uid: uid, EventId: event.ID, Version: event.Version, RecordedAt: time.Now().Unix(), Payload: string(payload)}); err != nil {
			return err
		}
	}
	return nil
}
func InvalidateWealthSnapshots(sess *xorm.Session, uid, from int64) error {
	_, err := sess.Where("uid=? AND recorded_at>=?", uid, from).Cols("invalidated").Update(&models.WealthSnapshot{Invalidated: true})
	return err
}

// Guard all legacy mutation paths, including batch APIs and data clearing.
func guardInvestmentTransactions(sess *xorm.Session, uid int64, ids []int64, allowTransferFees ...bool) error {
	if len(allowTransferFees) == 0 || !allowTransferFees[0] {
		if err := guardTransferFeeMutation(sess, uid, ids); err != nil {
			return err
		}
	}
	query := sess.Where("uid=? AND investment_event_id<>? AND deleted=?", uid, "", false)
	if len(ids) > 0 {
		query = query.In("transaction_id", ids)
	}
	has, err := query.Exist(&models.Transaction{})
	if err != nil {
		return err
	}
	if has {
		return ErrInvestmentLinked
	}
	return nil
}

func guardSystemAccounts(sess *xorm.Session, uid int64, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	has, err := sess.Where("uid=? AND system_role<>?", uid, "").In("account_id", ids).Exist(&models.Account{})
	if err != nil {
		return err
	}
	if has {
		return ErrInvestmentLinked
	}
	return nil
}
