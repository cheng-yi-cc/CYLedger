# 公共参考行情缓存

`Default.Start(ctx)` 使用服务端生命周期启动一次；所有用户和浏览器共享连接与缓存。`Quotes()`、`Get(id)`、`FX()` 只读内存，不会触发外部请求。外部请求只包含五个公开资产标识，不接收或发送持仓数量、成本、账户名称、用户 ID。

```go
marketquotes.Default.Restore(savedQuotes, savedFX) // 可选：从数据库恢复公共缓存
marketquotes.Default.Start(serverContext)
quotes := marketquotes.Default.Quotes()
rates := marketquotes.Default.FX()
```

持久化与用户手动估值由投资服务负责。本包不保存私人资产。恢复时保留原时间；恢复的数据先显示 `stale`，直到供应商刷新。

## 供应商与刷新

| 来源 | 用途 | 刷新方式 |
|---|---|---|
| Coinbase Advanced Trade | 主流币种美元参考价格 | 共享 `ticker_batch`，有变化时约 5 秒；同时订阅每秒 `heartbeats` |
| Coinbase 公共 REST | 启动及断线后的延迟参考价 | 最多每分钟一批，仅请求缺少健康流报价的已验证产品 |
| CoinGecko Demo | 补充与低频备用 | 配置服务端 `CYLEDGER_COINGECKO_DEMO_API_KEY` 后启用；批次间隔至少 15 分钟，失败与限流也计入间隔 |
| Frankfurter 的 ECB 数据 | USD/CNY 日参考汇率 | 启动获取，成功后 24 小时更新；失败后 15 分钟重试 |

初版使用已实测的 Coinbase **Advanced Trade** 公共流 `wss://advanced-trade-ws.coinbase.com`；它与开发文档最初举例的 Exchange 公共流是不同的端点与消息格式。无需 Coinbase 账户或账户密钥。

产品列表端点为 `https://api.coinbase.com/api/v3/brokerage/market/products`。每次建立映射都检查真实返回的产品 ID、基础资产 ID、计价资产 ID、现货类型和在线状态，不能因为配置了候选产品就声称支持。2026-09-20 本机实测验证 BTC-USD、ETH-USD、SOL-USD、USDT-USD；USDC-USD 未返回，因此无 CoinGecko 配置或手动估值时 USDC 明确无法自动估值。稳定币没有固定 1 美元兜底。

汇率端点固定为 `https://api.frankfurter.dev/v2/providers/ecb/rate/usd/cny`，明确选择 ECB；不将 Frankfurter 默认混合来源标记为 ECB。返回的 `date` 是参考汇率日期，`receivedAt` 是抓取时间。周末沿用上个工作日参考价正常，不冒充实时外汇成交价格。

## 状态约定

- `live`：已收到真实流报价，且对应已验证产品的心跳仍健康。价格不变时流可以不重发，心跳不会改写 `sourceTime` 或 `receivedAt`。
- `delayed`：REST、CoinGecko、ECB 日参考值，或短时断线后仍未过期的流报价。
- `stale`：过期或刚从数据库恢复的数据。保留价格与原时间，可用于明确标注的参考估值。
- `unavailable`：从未取得有效报价，价格或汇率为空字符串，不能按零计价。

Coinbase 非流报价超过两分钟标记过期；CoinGecko 超过两个刷新周期标记过期。ECB 抓取超过 36 小时或源日期超过七天标记过期，允许周末和短期节假日保留正常参考值。`Get` 的布尔值表示存在真实非空报价，因此过期价格仍返回 `true`，调用方须同时处理状态。

金额与汇率以十进制字符串输出。无效、非正数、异常指数、未来异常时间、旧序号和倒退的行情时间都不会改写缓存。HTTP 错误保留旧报价及旧时间；不会转成模拟值。连接在心跳读超时后关闭并指数退避重连，普通 ticker 消息不能掩盖心跳失效。

## 验证

```powershell
go test ./pkg/marketquotes -count=1
$env:CYLEDGER_LIVE_MARKET_TEST = '1'
go test ./pkg/marketquotes -run TestLivePublicProviders -count=1 -v
```

普通测试全部使用本地假服务，覆盖产品核实、精度、共享请求预算、心跳超时重连、乱序、取消、过期、失败保留和恢复。联网探测需显式开启；已于 2026-09-20 本机执行通过，BTC、ETH、SOL、USDT 取得真实流报价，USD/CNY 取得 2026-09-18 的 ECB 日参考汇率。CoinGecko Demo 没有实际密钥，本机只验证过无密钥公共查询可达；Demo 鉴权和 15 分钟预算由本地假服务验证。

官方依据：[Coinbase 端点与公开权限](https://docs.cdp.coinbase.com/coinbase-app/advanced-trade-apis/websocket/websocket-endpoints)、[Ticker batch](https://docs.cdp.coinbase.com/api-reference/advanced-trade-api/websocket/ticker-batch)、[CoinGecko Demo](https://docs.coingecko.com/demo/reference/simple-price)、[Frankfurter](https://frankfurter.dev/)、[ECB 更新说明](https://www.ecb.europa.eu/stats/policy_and_exchange_rates/euro_reference_exchange_rates/html/index.en.html)。使用 CoinGecko 数据时，产品界面及部署文档应保留 CoinGecko 来源署名，并由部署者核对当时的额度与使用条款。
