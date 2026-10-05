# 公共参考行情缓存

`Default.Start(ctx)` 使用服务端生命周期启动一次；所有用户和浏览器共享连接与缓存。`Quotes()`、`Get(id)`、`FX()` 只读内存，不会触发外部请求。外部请求只包含公开资产标识、搜索词和必要的供应商鉴权，不接收或发送持仓数量、成本、账户名称、用户 ID。

```go
marketquotes.Default.Register(savedBinding) // 自定义行情须先恢复已保存的公开绑定
marketquotes.Default.Restore(savedQuotes, savedFX) // 可选：从数据库恢复公共缓存
marketquotes.Default.Start(serverContext)
quotes := marketquotes.Default.Quotes()
rates := marketquotes.Default.FX()
```

持久化与用户手动估值由投资服务负责。本包不保存私人资产。恢复时保留原时间；恢复的数据先显示 `stale`，直到供应商刷新。服务层将共享的公开身份映射回当前用户的私人资产 ID，不在用户接口中枚举其他人的绑定。人民币手动价格优先于自动行情；删除手动价格后才恢复自动估值。

## 选择与身份

新资产可以按名称或代码搜索，用户明确选择市场、代码、来源和计价币种后绑定。沪深、北京、香港和美国证券分别使用供应商返回的身份；场外基金使用六位基金代码；加密资产使用 CoinGecko ID 或经过核验的 Coinbase 美元现货产品 ID。相同简称不会自动合并。绑定前复查身份与实际报价；失败时保留已有资产及手动价格。既有手动资产也可以补绑。

香港市场的港元、人民币、美元柜台按供应商实际币种分别保存，不能仅因为市场是香港就按港元换算。所有证券报价均核对币种；当前中国内地自动绑定限定人民币品种，不把美元/港元 B 股作为人民币证券接受。

中国市场包含供应商可检索并核验的股票及场内 ETF/LOF；场外基金单独检索已公布单位净值。货币基金返回的“每万份收益”不是单位净值，禁止用于份额乘价格估值，这类资产仍用手动价格。搜索结果不保证覆盖所有上市产品、基金或币种，退市、停牌、缺报价及供应商不返回的品种不会用模拟价格补齐。自定义公开绑定的共享缓存上限为 500 个。

## 供应商与刷新

| 来源 | 用途 | 刷新方式 |
|---|---|---|
| Coinbase Advanced Trade | 主流币种美元参考价格 | 共享 `ticker_batch`，有变化时约 5 秒；同时订阅每秒 `heartbeats` |
| Coinbase 公共 REST | 预置币启动/断线备用、自定义加密资产参考价 | 预置币最多每分钟一批；自定义绑定每分钟核验产品并读取最新成交，不接入 WebSocket |
| CoinGecko | 已绑定加密资产，以及可选预置币备用 | 自定义绑定可尝试无密钥公开查询；配置服务端 `CYLEDGER_COINGECKO_DEMO_API_KEY` 后同时启用预置币备用。两者合用一个至少 15 分钟的后台批次，失败与限流也计入间隔 |
| 腾讯财经 | 中国、香港和美国证券的公开参考报价 | 已绑定标的按分钟批量查询，每批最多 50 个，保留各市场的报价时间与币种 |
| 天天基金 | 场外基金已公布单位净值 | 已绑定基金每 30 分钟查询一次；净值日期不代表当前成交价或当前时刻 |
| Frankfurter 的 ECB 数据 | USD/CNY、HKD/CNY 日参考汇率 | 启动获取，成功后 24 小时更新；失败或旧日期响应后 15 分钟重试 |

上述频率为后台刷新规则。显式搜索与首次绑定需要按需查询身份/价格，会产生额外请求；同一搜索词与市场缓存 10 分钟。CoinGecko 无密钥可用性及额度取决于供应商和部署网络，返回限流或不可达时保持旧报价/未知状态。腾讯与基金公开端点也可能调整，页面统一称为公开参考价格或已公布净值，不承诺交易所实时行情。

使用已实测的 Coinbase **Advanced Trade** 公共流 `wss://advanced-trade-ws.coinbase.com`，无需 Coinbase 账户或账户密钥。

产品列表端点为 `https://api.coinbase.com/api/v3/brokerage/market/products`。每次建立映射都检查真实返回的产品 ID、基础资产 ID、计价资产 ID、现货类型和在线状态，不能因为配置了候选产品就声称支持。2026-09-20 本机实测验证 BTC-USD、ETH-USD、SOL-USD、USDT-USD；USDC-USD 未返回，因此后台无 CoinGecko 配置或手动估值时 USDC 可能无法自动估值；主动兑换可触发下面的公开按需查询。稳定币没有固定 1 美元兜底。

汇率端点固定为 `https://api.frankfurter.dev/v2/providers/ecb/rate/usd/cny` 与 `https://api.frankfurter.dev/v2/providers/ecb/rate/hkd/cny`，明确选择 ECB；不将 Frankfurter 默认混合来源标记为 ECB。返回的 `date` 是参考汇率日期，`receivedAt` 是抓取时间。周末沿用上个工作日参考价正常，不冒充实时外汇成交价格。港元资产缺少 HKD/CNY、美元资产缺少 USD/CNY 时，人民币估值保持未知。

## 状态约定

- `live`：已收到真实流报价，且对应已验证产品的心跳仍健康。价格不变时流可以不重发，心跳不会改写 `sourceTime` 或 `receivedAt`。
- `delayed`：REST、CoinGecko、证券公开参考价、基金已公布净值、ECB 日参考值，或短时断线后仍未过期的流报价。
- `stale`：过期或刚从数据库恢复的数据。保留价格与原时间，可用于明确标注的参考估值。
- `unavailable`：从未取得有效报价，价格或汇率为空字符串，不能按零计价。

Coinbase 非流报价超过两分钟标记过期；CoinGecko 超过两个刷新周期标记过期。证券抓取超过 10 分钟、基金抓取超过 48 小时或两者的源时间超过 10 天标记过期；市场休市时保留上次报价及真实日期，重新抓取不能把它标记为实时。ECB 抓取超过 36 小时或源日期超过七天标记过期，允许周末和短期节假日保留正常参考值。`Get` 的布尔值表示存在真实非空报价，因此过期价格仍返回 `true`，调用方须同时处理状态。

涨跌幅是可缺省的十进制字符串。加密资产表示供应商的 24 小时涨跌，证券表示对应交易日涨跌，基金表示净值涨跌。来源没有提供、格式无效或周期不匹配时保持未知，不当成零，也不把不同周期称为同一种个人单日收益。

金额与汇率以十进制字符串输出。无效、非正数、异常指数、未来异常时间、旧序号和倒退的行情时间都不会改写缓存。HTTP 错误保留旧报价及旧时间；不会转成模拟值。连接在心跳读超时后关闭并指数退避重连，普通 ticker 消息不能掩盖心跳失效。

## 验证

```powershell
go test ./pkg/marketquotes -count=1
$env:CYLEDGER_LIVE_MARKET_TEST = '1'
go test ./pkg/marketquotes -run TestLivePublicProviders -count=1 -v
go test ./pkg/marketquotes -run TestLiveReferenceSearchAndQuotes -count=1 -v
```

普通测试全部使用本地假服务，覆盖产品核实、精度、共享请求预算、心跳超时重连、乱序、取消、过期、失败保留和恢复。联网探测需显式开启；已于 2026-09-20 本机执行通过，BTC、ETH、SOL、USDT 取得真实流报价，USD/CNY 取得 2026-09-18 的 ECB 日参考汇率。CoinGecko Demo 没有实际密钥，本机只验证过无密钥公共查询可达；Demo 鉴权和 15 分钟预算由本地假服务验证。

新增场景覆盖公开绑定的市场/币种/代码校验、场外基金与货币基金收益区分、港元缺汇率、人民币手动价优先、用户隔离、报价/汇率乱序和恢复。2026-09-21 本机对 `600519`、`510300`、港元柜台 `00700`、人民币柜台 `80700`、美元基金柜台 `09046`、`AAPL`、场外基金 `000001`、Coinbase `DOGE-USD` 完成搜索与报价核验，并查询 `000009` 确认其返回的是货币基金每万份收益；这代表这些样本当时可用，不代表所有市场标的或未来可用性。

官方依据：[Coinbase 端点与公开权限](https://docs.cdp.coinbase.com/coinbase-app/advanced-trade-apis/websocket/websocket-endpoints)、[Ticker batch](https://docs.cdp.coinbase.com/api-reference/advanced-trade-api/websocket/ticker-batch)、[CoinGecko Demo](https://docs.coingecko.com/demo/reference/simple-price)、[Frankfurter](https://frankfurter.dev/)、[ECB 更新说明](https://www.ecb.europa.eu/stats/policy_and_exchange_rates/euro_reference_exchange_rates/html/index.en.html)。使用 CoinGecko 数据时，产品界面及部署文档应保留 CoinGecko 来源署名，并由部署者核对当时的额度与使用条款。


## 主动兑换的行情刷新

用户在加密账户输入兑换数量时，优先使用可用的近期行情；需要刷新时查询 CoinGecko 公开 simple-price 批次，覆盖预置币种和本次已验证的自定义 CoinGecko 映射。与后台共享请求预算，包含失败每分钟最多尝试一次；自定义 Coinbase 映射另有同等逐标的门限。无须额外密钥，但依赖网络可达及供应商额度，不承诺持续可用。

刷新保留源时间和抓取时间；失败不会更新旧报价日期。兑换只接受正常行情状态、五分钟内的非流价格及有效人民币汇率，并保存 120 秒有效的换算快照。界面允许按实收修正到账数；转出数和参考行情同时留在记录中。手机分流代理需包含 CYLedger 应用。

## 货币基金逐日收益数据

`monetary.go` 提供货币基金名称/代码搜索与按日期读取天天基金公开万份收益，和单位净值估值分离。输入需为有效六位基金代码，响应按日期核对；缺失或未公布日期不补零、不拿相邻日期替代。

本模块只提供公开数据；人民币账户的显式绑定、交易日和 15:00 起息规则、定存本金扣除、D+1 入账及永久防重由 `pkg/monetaryincome` 和 `pkg/services/monetary_income.go` 处理。公开源不保证覆盖全部基金或持续可达。账户和收益关系见 [架构](../../docs/CYLEDGER_ARCHITECTURE.md)，接口见 [API 指南](../../docs/CYLEDGER_API.md)。
