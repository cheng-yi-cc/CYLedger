from pathlib import Path
import subprocess

MAIN = 'c59580cce0a58b54623b85e43924db953c2fbedd'
def source(ref, path):
    return subprocess.check_output(['git', 'show', f'{ref}:{path}'], text=True)
def replace(text, old, new):
    assert text.count(old) == 1, old
    return text.replace(old, new)
def write(path, text):
    Path(path).write_text(text)

# Resolve only reviewed overlapping files; every other upstream change is kept.
conflicts = set(subprocess.check_output(['git', 'diff', '--name-only', '--diff-filter=U'], text=True).splitlines())
reviewed = {'pkg/marketquotes/service.go', 'pkg/marketquotes/references.go',
            'pkg/marketquotes/README.md', 'docs/CYLEDGER_ACCEPTANCE.md'}
assert conflicts <= reviewed, conflicts
s = source('HEAD', 'pkg/marketquotes/service.go')
s = replace(s, 'CoinbaseRESTURL    string', 'CoinbaseRESTURL    string\n\tCoinbaseExchangeURL string')
s = replace(s, 'lastGeckoAttempt     time.Time', 'lastGeckoAttempt     time.Time\n\tactiveUntil time.Time')
s = replace(s, 'lastReferenceAttempt map[string]time.Time', 'lastReferenceAttempt map[string]time.Time\n\treferenceFailed map[string]bool')
s = replace(s, 'lastReferenceAttempt: make(map[string]time.Time),', 'lastReferenceAttempt: make(map[string]time.Time), referenceFailed: make(map[string]bool),')
s = replace(s, 'hkdFXRestored        bool', 'hkdFXRestored        bool\n\tdayMu sync.Mutex\n\tdays map[string]*dayEntry')
s = replace(s, '\tif config.CoinbaseWSURL == "" {', '\tif config.CoinbaseExchangeURL == "" {\n\t\tconfig.CoinbaseExchangeURL = "https://api.exchange.coinbase.com"\n\t}\n\tif config.CoinbaseWSURL == "" {')
s = replace(s, '\tconfigureReferenceSources(&config)', '\tconfig.CoinbaseExchangeURL = strings.TrimRight(config.CoinbaseExchangeURL, "/")\n\tconfigureReferenceSources(&config)')
s = replace(s, 'referenceWake: make(map[string]chan struct{})}', 'referenceWake: make(map[string]chan struct{}), days: make(map[string]*dayEntry)}')
s = replace(s, '// Start starts background refreshes once', '''// MarkActive preserves the foreground lease used by income and valuation reads.
func (s *Service) MarkActive() {
    s.mu.Lock()
    s.activeUntil = s.config.Now().Add(90 * time.Second)
    s.mu.Unlock()
}

// Start starts background refreshes once''')
write('pkg/marketquotes/service.go', s)
write('pkg/marketquotes/references.go', source('HEAD', 'pkg/marketquotes/references.go'))
# The old DNS race is superseded by the independent route transport, not kept dead.
for path in ('pkg/marketquotes/fund_transport.go', 'pkg/marketquotes/fund_transport_test.go'):
    Path(path).unlink()
p = 'pkg/marketquotes/reference_scheduler.go'
s = Path(p).read_text()
s = replace(s, '\tinterval := time.Minute\n', '\ts.mu.Lock()\n\tnow := s.config.Now()\n\tinterval := time.Minute\n')
s = replace(s, '\t\tinterval = 30 * time.Minute\n', '\t\tinterval = 30 * time.Minute\n\t\tif now.Before(s.activeUntil) {\n\t\t\tinterval = 5 * time.Minute\n\t\t}\n')
s = replace(s, '\t}\n\ts.mu.Lock()\n\tnow := s.config.Now()\n\tbindings', '\t}\n\tbindings')
s = replace(s, '\t\tif previous := s.lastReferenceAttempt[key]; !previous.IsZero() && now.Sub(previous) < interval {', '\t\tbudget := interval\n\t\tif s.referenceFailed[key] { budget = time.Minute }\n\t\tif previous := s.lastReferenceAttempt[key]; !previous.IsZero() && now.Sub(previous) < budget {')
s = replace(s, '\t\t\tif ok && s.putReferenceQuote(q) {\n\t\t\t\tcontinue', '\t\t\tif ok && s.putReferenceQuote(q) {\n\t\t\t\ts.mu.Lock()\n\t\t\t\tif s.lastReferenceAttempt[b.Key()].Equal(now) { delete(s.referenceFailed, b.Key()) }\n\t\t\t\ts.mu.Unlock()\n\t\t\t\tcontinue')
s = replace(s, 's.lastReferenceAttempt[b.Key()] = s.config.Now().Add(-interval + time.Minute)', 's.lastReferenceAttempt[b.Key()] = s.config.Now()\n\t\t\t\ts.referenceFailed[b.Key()] = true')
s = replace(s, 'delete(s.lastReferenceAttempt, b.Key())', 'delete(s.lastReferenceAttempt, b.Key())\n\t\t\t\tdelete(s.referenceFailed, b.Key())')
write(p, s)
p = 'pkg/marketquotes/network.go'
s = Path(p).read_text()
s = replace(s, '{s.config.CoinbaseRESTURL, "/v1/coinbase"},', '{s.config.CoinbaseRESTURL, "/v1/coinbase"},\n\t\t\t{s.config.CoinbaseExchangeURL, "/v1/coinbase-exchange"},')
write(p, s)
p = 'pkg/marketrelay/relay.go'
s = Path(p).read_text()
s = replace(s, 'var coinPath =', 'var candlePath = regexp.MustCompile(`^/products/[A-Z0-9][A-Z0-9._-]{0,63}/candles$`)\nvar ohlcPath = regexp.MustCompile(`^/coins/[a-z0-9][a-z0-9._-]{0,63}/ohlc$`)\nvar coinPath =')
s = replace(s, '\tcase path == "/v1/coingecko/simple/price":', '''    case strings.HasPrefix(path, "/v1/coinbase/") && candlePath.MatchString(strings.TrimPrefix(path, "/v1/coinbase")):
        base = "https://api.coinbase.com/api/v3/brokerage/market" + strings.TrimPrefix(path, "/v1/coinbase")
        allowed = "start end granularity limit"
    case strings.HasPrefix(path, "/v1/coinbase-exchange/") && candlePath.MatchString(strings.TrimPrefix(path, "/v1/coinbase-exchange")):
        base = "https://api.exchange.coinbase.com" + strings.TrimPrefix(path, "/v1/coinbase-exchange")
        allowed = "start end granularity"
    case strings.HasPrefix(path, "/v1/coingecko/") && ohlcPath.MatchString(strings.TrimPrefix(path, "/v1/coingecko")):
        base = "https://api.coingecko.com/api/v3" + strings.TrimPrefix(path, "/v1/coingecko")
        allowed = "vs_currency days precision"
    case path == "/v1/coingecko/simple/price":''')
s = replace(s, '\t\tbase = "https://api.frankfurter.dev/v2/providers/ecb/rate/" + strings.TrimPrefix(path, "/v1/fx/")', '\t\tbase = "https://api.frankfurter.dev/v2/providers/ecb/rate/" + strings.TrimPrefix(path, "/v1/fx/")\n\t\tallowed = "date"')
write(p, s)

# A full mock relay round trip ensures new income lookups do not bypass it.
write('pkg/marketquotes/history_relay_test.go', r'''package marketquotes

import (
    "context"
    "errors"
    "net/http"
    "net/http/httptest"
    "strings"
    "sync/atomic"
    "testing"
    "time"

    "github.com/mayswind/ezbookkeeping/pkg/marketrelay"
)

func TestHistoryLookupsUseRelay(t *testing.T) {
    var direct, upstream atomic.Int32
    relay, err := marketrelay.New(marketrelay.Config{Token: testRelayToken, Client: &http.Client{Transport: routeTransport(func(r *http.Request) (*http.Response, error) {
        upstream.Add(1)
        if r.Header.Get(marketrelay.TokenHeader) != "" { t.Error("relay credential leaked") }
        switch r.URL.Host {
        case "api.coinbase.com", "api.exchange.coinbase.com", "api.coingecko.com", "api.frankfurter.dev":
        default: t.Error("unexpected upstream host")
        }
        return routeResponse(), nil
    })}})
    if err != nil { t.Fatal(err) }
    server := httptest.NewServer(relay)
    defer server.Close()
    s := New(Config{Network: NetworkConfig{RelayURL: server.URL, RelayToken: testRelayToken}, DomesticHTTPClient: server.Client(), HTTPClient: &http.Client{Transport: routeTransport(func(*http.Request) (*http.Response, error) {
        direct.Add(1)
        return nil, errors.New("overseas source is unavailable without relay")
    })}})
    for _, address := range []string{
        s.config.CoinbaseRESTURL + "/products/BTC-USD/candles?start=1&end=61&granularity=ONE_MINUTE&limit=2",
        s.config.CoinbaseExchangeURL + "/products/BTC-USD/candles?start=2026-10-06T00:00:00Z&end=2026-10-06T00:01:00Z&granularity=60",
        s.config.CoinGeckoCoinURL + "/bitcoin/ohlc?vs_currency=usd&days=1&precision=full",
        s.config.FXURL + "?date=2026-10-05",
    } {
        var value map[string]interface{}
        if err := s.getJSON(context.Background(), address, nil, &value); err != nil { t.Fatalf("history route: %s: %v", address, err) }
    }
    if direct.Load() != 0 || upstream.Load() != 4 { t.Fatalf("direct=%d relay=%d", direct.Load(), upstream.Load()) }
    for _, path := range []string{"/v1/coinbase-exchange/accounts", "/v1/coinbase-exchange/products/BTC-USD/candles?url=https://evil.example", "/v1/coingecko/coins/bitcoin/market_chart", "/v1/fx/usd/cny?base=USD"} {
        req := httptest.NewRequest(http.MethodGet, path, nil)
        req.Header.Set(marketrelay.TokenHeader, testRelayToken)
        recorder := httptest.NewRecorder()
        relay.ServeHTTP(recorder, req)
        if recorder.Code != 400 || strings.Contains(recorder.Body.String(), testRelayToken) { t.Errorf("invalid history request: %s", path) }
    }
    if upstream.Load() != 4 { t.Error("unsupported request reached upstream") }
}
''')
p = 'pkg/marketquotes/history_relay_test.go'
s = Path(p).read_text() + r'''
func TestFailedFundRetriesAfterForegroundLeaseExpires(t *testing.T) {
    var now, calls atomic.Int64
    now.Store(1791324000)
    s := New(Config{Now: func() time.Time { return time.Unix(now.Load(), 0) }, HTTPClient: &http.Client{Transport: routeTransport(func(*http.Request) (*http.Response, error) {
        calls.Add(1)
        return nil, errors.New("temporary fund outage")
    })}})
    s.Register(Binding{Market: "CN_FUND", Provider: "eastmoney", ProviderID: "000001", Currency: "CNY"})
    s.MarkActive()
    s.refreshReferenceProvider(context.Background(), "eastmoney")
    now.Add(120) // The 90-second foreground lease has expired.
    s.refreshReferenceProvider(context.Background(), "eastmoney")
    if calls.Load() != 2 { t.Fatalf("fund recovery suppressed by idle budget: %d requests", calls.Load()) }
}
'''
write(p, s)
# Preserve the latest upstream budget documentation and history accounting rules.
s = source(MAIN, 'pkg/marketquotes/README.md')
start = s.index('基金目录与收益接口仅对')
end = s.index('经公开源确认的货币基金目录结果', start)
ours = source('HEAD', 'pkg/marketquotes/README.md')
start_ours = ours.index('基金、收益和腾讯行情使用独立国内连接池')
end_ours = ours.index('经公开源确认的货币基金目录结果', start_ours)
s = s[:start] + ours[start_ours:end_ours] + s[end:]
s = s.replace('手机分流代理需包含 CYLedger 应用。', '配置可达的自建行情中转后，海外现价及今日收益所需历史行情均经中转；未配置时仍依赖系统海外路由。')
write('pkg/marketquotes/README.md', s)
s = source(MAIN, 'docs/CYLEDGER_ACCEPTANCE.md')
s = replace(s, '# CYLedger 验收与交接\n', '''# CYLedger 验收与交接

## 行情网络重构（独立分支，尚未部署）

`fix/market-data-routing` / PR #1 在保留以下主分支账务与页面功能的基础上，增加国内/海外隔离路由、有界并发搜索与刷新、Android 物理网络与同网 DNS、自建 HTTPS/WSS 白名单行情中转。今日收益使用的 Coinbase / Exchange 分钟线、CoinGecko OHLC 和历史 ECB 汇率也接入中转，不修改收益算法。

本次尚未部署中转、配置生产域名/令牌、构建完整 APK 或在真机安装测速。远程电脑离线；后端源码与 Android Java/ARM64/JNI 的 CI 编译不等于真机验收。VPN 禁止旁路和系统 TUN 仍遵守系统限制。38 个 Go 顶层测试及 Java/原生编译已在此前源码提交 `790c5b64` 的 Actions `37571509495` 通过；同步主分支后的结果须查看本 PR 最新 CI，不沿用旧测试计数。

前台仍沿用主分支每 5 秒读取本地估值缓存，Coinbase 支持范围内使用原实时流；CoinGecko 前台每分钟最多一批、空闲至少 15 分钟，基金净值前台 5 分钟、空闲 30 分钟。网络重构没有新增前端逐笔推送，也不把基金已公布净值伪装成实时数据。货币基金待公布收益的按分钟重试及永久防重不变。

在拉取此分支的独立工作目录预览：
```powershell
powershell -ExecutionPolicy Bypass -File .\\scripts\\Start-CYLedger.ps1 -Port 18083 -Build -DirectNpm
```
打开 `http://localhost:18083/mobile` → 资产 → 理财 / 收益同步。配置与网络验收见 [行情网络说明](CYLEDGER_MARKET_NETWORK.md)。同签名正式升级前须完成独立 QA 验证；不得卸载或清空正式账本。

## 已安装主分支的验收基线

以下为主分支 `c59580cc` 已记录的正式版本与真机结果，不代表本次网络重构版已安装或复测。
''')
write('docs/CYLEDGER_ACCEPTANCE.md', s)
p = 'docs/CYLEDGER_MARKET_NETWORK.md'
s = Path(p).read_text()
s = s.replace('基金成功后仍按 30 分钟预算刷新', '基金成功后前台按 5 分钟、空闲按 30 分钟预算刷新')
s = s.replace('CoinGecko 定期批量价格刷新仍至少间隔 15 分钟，不通过页面重刷消耗预算。', '保留主分支 CoinGecko 前台每分钟最多一批、空闲至少 15 分钟的预算，失败和限流也计入。')
s = s.replace('移动投资列表通常每 15 秒、投资工作区每 10 秒刷新', '前台估值页面通常每 5 秒读取本地共享缓存')
s = s.replace('本次保留原有前端汇总轮询', '本次保留已同步主分支的前端汇总轮询')
s = s.replace('powershell -ExecutionPolicy Bypass -File .\\scripts\\Start-CYLedger.ps1 -Build -DirectNpm', 'powershell -ExecutionPolicy Bypass -File .\\scripts\\Start-CYLedger.ps1 -Port 18083 -Build -DirectNpm')
s = s.replace('http://localhost:8080/', 'http://localhost:18083/mobile')
s += '\n今日收益所需的 Coinbase Advanced / Exchange 分钟线、CoinGecko OHLC 及带日期的 ECB 历史汇率也通过白名单中转；保留零点精确收盘与历史日期校验，不以当前价或当前汇率代替历史基准。\n'
write(p, s)
subprocess.run(['gofmt', '-w', 'pkg/marketquotes/service.go', 'pkg/marketquotes/reference_scheduler.go', 'pkg/marketquotes/network.go', 'pkg/marketquotes/history_relay_test.go', 'pkg/marketrelay/relay.go'], check=True)
subprocess.run(['git', 'add', '-A', 'pkg', 'docs'], check=True)
assert not subprocess.check_output(['git', 'diff', '--name-only', '--diff-filter=U']).strip()
subprocess.run(['git', 'diff', '--cached', '--check'], check=True)
