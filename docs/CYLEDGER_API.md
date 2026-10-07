# CYLedger 接口指南

接口入口为 `/api/v1`，复用 ezBookkeeping 登录鉴权，客户端使用 `Authorization: Bearer <token>`。用户身份来自鉴权上下文，不能用请求体中的 UID 代替。正式 Android 自动取得本机会话的机制只供内嵌 WebView，不是外部登录 API。

接口注册以 `cmd/webserver.go` 为准。新增路由清单如下，普通账户、流水、分类、模板及导入导出仍沿用上游接口；相关新增字段见 `pkg/models/account_asset_profile.go`、`transaction.go`、`book.go`。


## 手机工作区

- `GET /api/v1/ledger/items?kind=wish|keyword`：当前用户的愿望/关键词。
- `POST /api/v1/ledger/items/save`：`{id, kind, revision, data}`；新增 `id=""`、`revision="0"`，编辑必须携带已读修订号。
- `POST /api/v1/ledger/items/delete`：`{id, kind, revision}`；只删除当前用户和匹配修订的项目。
- `GET /api/v1/ledger/imports`：导入批次 `{id,name,count,createdAt,deleted}`，`count` 为十进制整数字符串。
- `POST /api/v1/ledger/imports/undo`：`{id}`，整批撤回可重复请求。

原有 `POST /transactions/import.json` 请求可附 `batchId`（最多 64 字符）及 `sourceName`（最多 128 字符）；重试保持同一 ID 和内容。批次入账、指纹和交易 ID 在同一事务保存。撤回后保留防重身份，该 ID 不得再次导入。

普通账单请求/响应增加 `transferFeeAmount`（最多两位小数的带符号字符串）、`transferFeeCategoryId`、`debtDueDate`（YYYY-MM-DD 或空串）、`locationName`（最多 200 字）。`transferFeeParentId` 仅由响应返回关联的原转账 ID。手续费仅适用于转出账单，正数选支出分类、负数选收入分类；关联费用不能单独编辑。约定日期只支持借入/借出，响应的日历事项含 `transactionId`；提醒变更请修改原账单。修改请求省略扩展字段则保留旧值，清空日期须显式传空串。模型定义仍是完整字段与约束的依据。

愿望保存示例（`POST /ledger/items/save`，不产生资金流水）：

```json
{"id":"","kind":"wish","revision":"0","data":{"name":"旅行","icon":"gift","target":"1000.00","initial":"50.00","startDate":"2026-10-05","endDate":"","mode":"manual","cycle":"month","amount":"0.00","ratio":"100.00","bookIds":[],"accountIds":[],"incomeCategoryIds":[],"expenseCategoryIds":[],"note":"","archived":false,"logs":[]}}
```

`mode` 可选 `manual/schedule/income/balance/asset`，`cycle` 为 `day/week/month/year`；`target` 大于零，`ratio` 在 0–100 之间。`logs` 最多 300 条，金额允许带负号表示取出，手动模式累计不能为负。关键词 `data` 为 `{keywords,categoryId,accountId,tagIds,enabled}`：至少一个关键词、有效分类，账户可为空字符串，标签最多 10 个。引用必须属于当前用户。每种公开类型最多 500 项，`data` 上限 60 KiB；完整 JSON 上限 64 KiB。保存返回分配的 ID 和递增修订号，后续保存/删除必须使用最新修订号。


## 手机理财、基金确认与定投

| 路由 | 契约 |
|---|---|
| `GET /investments/holdings` | 当前用户单项展示规则 |
| `POST /investments/holdings` | 保存 `accountId/instrumentId/name/group/note/profitOffset/hidden/excludeFromTotal/excludeProfit/bookIds/version`；版本须匹配 |
| `POST /investments/holdings/setup` | `profile/instrument/quantity/cost/price/bookId/occurredAt`；携带 `Idempotency-Key`，原子建立理财及已有持仓 |
| `POST /investments/holdings/update` | 同上，附 `expectedQuantity/expectedCost`；原子保存规则与必要的 `ADJUST` 校准，保留未改成本的全部回放精度 |
| `GET /investments/plans`、`POST /investments/plans` | 定投读取/保存，字段为模型 `InvestmentPlan`；`cycle=daily/weekly/biweekly/monthly`，金额含费，`feePercent` 为0–100，须传 `version` |
| `GET /investments/orders`、`POST /investments/orders` | 确认指令读取/创建，模型 `InvestmentOrder`；创建携带 `Idempotency-Key`，买入二选一 `amount/quantity`，卖出填 `quantity` |
| `POST /investments/orders/confirm` | `{id,version,price,date}`；手动确认净值并原子入账 |
| `POST /investments/orders/cancel` | `{id,version}`；永久取消本期，不删除防重身份 |
| `POST /investments/plans/sync` | `{force}`；生成到期指令并按历史净值确认，返回 `{created,pending}`；`created` 为本次成功入账数，`pending` 为仍待确认数，大批积压分次处理 |
| `GET /investments/report?accountId=&instrumentId=` | 回放交易效果、真实快照历史和简单年化估算；未知字段为 `null` |

日期为 `YYYY-MM-DD`、时间 `HH:mm`、会计时区为有效 IANA 名称。指令为基金 `BUY/SELL`，须有人民币资金账户；`amount/fee` 最多两位小数，`quantity/price/feePercent` 使用受限十进制字符串。待确认不影响资金和持仓；15:00起顺延至下一公开净值日期。入账事件附 `fund:{tradeDate,confirmDate,price,priceDate,source,orderId}`，后续可沿用事件修订/撤销接口。已处理或取消的指令不能再次入账。

`GET /monetary-income/list` 附只读 `totalIncome/lastPerTenThousand`，前者由仍有效的关联收入累加，后者为最后一个已结算日的万份收益；两者都不维护第二份本金。

新增定投示例（`POST /investments/plans`，替换为当前用户的有效账户、基金和账本 ID）：

```json
{"id":"","version":0,"accountId":"portfolio-id","instrumentId":"fund-id","cashAccountId":"cash-id","bookId":"book-id","amount":"100.00","feePercent":"0.15","cycle":"monthly","startDate":"2026-10-31","endDate":"","time":"10:00","timeZone":"Asia/Shanghai","note":"每月定投","paused":false,"deleted":false}
```

更新时传回完整计划及最新 `version`；`paused=true` 暂停，`deleted=true` 删除并取消待确认期次，均保留历史防重身份。`nextDate` 由服务维护，不由客户端推进；按月计划锚定开始日，短月份取月末。读取 `/investments/orders` 后，`status=pending/completed/cancelled` 分别表示待确认、已入账、已取消；`error` 给出等待原因。`POST /investments/plans/sync` 的 `force=true` 只跳过重试间隔，不跳过到期、归属、余额或防重校验，也不会向银行/基金平台实际扣款。

## 加密账户与钱包品牌

`POST /investments/accounts/crypto` 接收 `{name, kind, platform, bookId, holdings:[{instrumentId, quantity}]}`，须携带 `Idempotency-Key`；账户与已有持仓在同一事务保存，重试不能重复增加数量。`kind="WALLET"` 时 `platform` 可省略或传空字符串；选 Bitget Wallet 时为 `bitget-wallet`。`kind="EXCHANGE"` 仍须提供匹配的交易所，Bitget 交易所为 `bitget`。非空的未知品牌或类型不匹配仍被拒绝。

`POST /investments/accounts/update` 提交 `{id,name,kind,platform,instruments}` 及可选的货币/收付设置；钱包品牌采用相同规则，传 `platform=""` 可取消品牌选择。编辑品牌不改变账户类型、已有数量与交易流水；持有中的币种不能移除。

加密账户创建及修改支持 `currency: "CNY" | "USD"` 和 `paymentInstruments: string[]`。旧账户默认人民币；创建时省略币种按人民币，修改时省略则保留原值。付款币种至多两项，顺序为优先/备用，须在账户选择的币种中且不得重复；第一项也是默认收款币种。修改时省略 `paymentInstruments` 保留原设置，传 `[]` 清空默认选择。切换账户货币单位只改变市值显示和新增收支默认值，历史金额及汇率保留。

钱包收支复用 `/investments/events`、`/preview`、`/{id}/revise`、`/{id}/void`，创建须带 `Idempotency-Key`，修订和撤销须带最新 `version`。事件 `type` 为 `INCOME` 或 `EXPENSE`，`amount` 为原币金额（正数、最多两位小数），`exchangeRate` 为原币到人民币的历史汇率；人民币为 `1`。`instrumentId/quantity` 表示第一种实际收付币种，可附 `additionalMovements:[{instrumentId,quantity}]` 表示第二种支出币种，收入只允许一种。须提供 `wallet:{currency,categoryId,fxDate,fxSource}`，日期为 `YYYY-MM-DD`，分类须为对应收支的有效子分类。数量/汇率为受限十进制字符串；`fee="0"`、`cost=null`，无现金账户或兑换结算字段，费用计入实际金额及数量。

收支、币种成本回放、一笔人民币统计账单和快照失效原子完成；修订/撤销同步重建持仓和关联账单。系统统计账户不计入资产余额，普通账单接口不能单独改删这笔关联记录。账单响应附 `wallet:{accountId,accountName,currency,amount,exchangeRate,fxDate}`，从原投资事实解析；人民币统计金额按分四舍五入，前端显示原币金额。`GET /wealth/summary` 的 `fxRates` 提供估值所用汇率，不能据当前行情改写历史收支。

钱包支出示例：将账户、账本、分类和币种 ID 换为当前用户的有效值，先提交 `/investments/events/preview`，确认后用相同内容提交 `/investments/events`，后者携带 `Idempotency-Key`。此例扣除 10 USD24 和 5 USDC，以实际确认的 7.2 汇率生成一笔 108 元支出。

```json
{
  "type":"EXPENSE", "accountId":"wallet-id", "bookId":"book-id",
  "instrumentId":"private-usd24-id", "quantity":"10",
  "additionalMovements":[{"instrumentId":"crypto:usd-coin","quantity":"5"}],
  "amount":"15.00", "exchangeRate":"7.2", "fee":"0", "cost":null,
  "occurredAt":1791162000, "note":"美元消费",
  "wallet":{"currency":"USD","categoryId":"category-id","fxDate":"2026-10-05","fxSource":"用户确认"}
}
```

普通单币账户的 `GET /accounts/get.json` 返回 `currencyEditable`。只有零余额、无任何账单历史（含已删除）、无相关模板/收益绑定/定存或计价金额规则的账户可改币种；共享信用额度的关联账户也锁定。`POST /accounts/modify.json` 在事务内重新校验条件，有历史的账户继续使用原币种。

## 路由清单

| 方法 | 路径（省略 `/api/v1`） |
|---|---|
| GET | `/books/list` |
| GET | `/ledger/items`、`/ledger/imports` |
| POST | `/ledger/items/save`、`/ledger/items/delete`、`/ledger/imports/undo` |
| GET / POST | `/statistics/preferences` |
| GET | `/statistics/budgets`、`/statistics/notes`、`/statistics/auxiliary` |
| POST | `/statistics/budgets/save`、`/statistics/budgets/delete`、`/statistics/notes/save` |
| GET | `/monetary-income/search` |
| GET | `/assets/reimbursements` |
| GET | `/assets/debts` |
| POST | `/assets/debts/record` |
| GET | `/assets/credit-reports` |
| GET | `/assets/installments` |
| POST | `/assets/installments/preview` |
| POST | `/assets/installments/save` |
| POST | `/assets/installments/close` |
| POST | `/assets/installments/sync` |
| POST | `/assets/adjust-balance` |
| POST | `/assets/reimbursements/receive` |
| POST | `/assets/reimbursements/state` |
| GET | `/monetary-income/list` |
| POST | `/monetary-income/save` |
| POST | `/monetary-income/pause` |
| POST | `/monetary-income/sync` |
| GET | `/calendar/list` |
| POST | `/calendar/save` |
| POST | `/calendar/complete` |
| POST | `/calendar/delete` |
| GET | `/calendar/pending` |
| GET | `/assets/preferences` |
| POST | `/assets/preferences` |
| GET | `/assets/deposits` |
| POST | `/assets/deposits/save` |
| POST | `/assets/deposits/close` |
| POST | `/assets/deposits/sync` |
| POST | `/books/create` |
| POST | `/books/modify` |
| POST | `/books/move` |
| GET | `/investments/settings` |
| POST | `/investments/settings` |
| GET | `/investments/accounts` |
| POST | `/investments/accounts` |
| POST | `/investments/accounts/update` |
| POST | `/investments/accounts/crypto` |
| POST | `/wealth/accounts/delete/preview` |
| POST | `/wealth/accounts/delete` |
| POST | `/investments/conversion` |
| GET | `/investments/instruments/search` |
| POST | `/investments/instruments/bind` |
| GET | `/investments/instruments` |
| POST | `/investments/instruments` |
| GET | `/investments/events` |
| POST | `/investments/events` |
| POST | `/investments/events/preview` |
| POST | `/investments/events/:id/revise` |
| POST | `/investments/events/:id/void` |
| GET / POST | `/investments/holdings`、`/investments/plans`、`/investments/orders` |
| POST | `/investments/holdings/setup`、`/investments/holdings/update` |
| POST | `/investments/orders/confirm`、`/investments/orders/cancel`、`/investments/plans/sync` |
| GET | `/investments/report` |
| GET | `/investments/positions` |
| GET | `/investments/quotes` |
| POST | `/investments/quotes/manual` |
| GET | `/investments/export` |
| GET | `/wealth/summary` |
| GET | `/wealth/history` |


## 通用约定

- 新增账务 API 中金额、数量、价格、汇率和大整数 ID 以字符串传递，例如 `"100.25"`；时间戳为 Unix 秒，业务日期为 `YYYY-MM-DD`，时区为 IANA 名称。旧流水接口仍使用其整数分格式，不能混用。
- 投资类 JSON 绑定限制请求体 64 KiB；格式/长度验证先于十进制解析。普通资金支持两位小数，投资数量/价格上限为 18 位；不接收指数表示或浮点替代。
- 返回使用现有成功/错误信封。先处理 HTTP 状态及业务错误，再读取结果；失败不能当作空列表或零余额。
- 投资事件创建及加密账户创建使用 `Idempotency-Key` 请求头；余额校准、报销到账、债务和分期使用请求体 `requestId`。创建重试保持同一键及同一内容，改内容须创建新请求；事件修订/撤销使用版本比较，不缓存幂等键结果。
- 投资修订、分期更新、资产偏好及预算/总结/统计偏好使用返回的版本/修订号。冲突后重新读取与预览，不盲目覆盖。
- `GET /wealth/summary` 可能保存估值快照；`POST */sync` 可能写入到期费用/收益，不能当作无副作用的健康检查。健康检查使用 `/healthz.json`。

`GET /wealth/summary` 的持仓附带 `dailyPnl`（人民币十进制字符串或 `null`）、`dayStart`（会计时区当日零点的 Unix 秒）及 `dailyReason`（未知原因）。取得公开历史基准时附 `dailyReference:{source,price,currency,at,sourceTime,period,fxRate,fxDate}`，其中周期以秒计，价格和汇率为十进制字符串。该值以零点边界前的公开收盘价和历史汇率比较，并通过同一成本引擎回放当天交易；跨账户转移不制造收益，费用计入当日。历史基准查询异步完成，后续读取取得结果；不得将 `null` 当零，也不能用供应商的 24 小时涨跌代替。页面读取保持每 5 秒一次，前台活跃时共享 CoinGecko 请求预算为每分钟最多一批。

单笔 `transactions/get.json` 返回实际 `createdAt`（Unix 秒）、`scheduledCreated`，已关联货币基金日结记录时附 `monetaryIncome`（收益日期、基金代码、万份收益、计息本金等原始记录）。查询按当前用户和账单 ID 校验归属；详情的账单日期和实际记录时间分别展示，不以零点账单日期冒充真实到账时间。


## 查询与最小示例

在已有合法会话中查询。以下环境变量仅是调用示例，不是应用新增配置；不要把令牌写入文档或版本库。

```powershell
$ledgerBase = 'http://localhost:8080/api/v1'
$ledgerHeaders = @{ Authorization = "Bearer $env:CYLEDGER_TEST_TOKEN" }
Invoke-RestMethod "$ledgerBase/assets/debts" -Headers $ledgerHeaders
Invoke-RestMethod "$ledgerBase/monetary-income/search?q=000198" -Headers $ledgerHeaders
```

债务 `POST /assets/debts/record` 示例（账户/账本 ID、时间须替换为当前用户的有效值，金额是示例）：

```json
{
  "debtAccountId": "123", "cashAccountId": "456", "action": "repay",
  "principal": "100.00", "interest": "5.00", "time": 1791162000,
  "timeZone": "Asia/Shanghai", "bookId": "book-id",
  "note": "分次还款", "requestId": "debt-example-0001"
}
```

`action` 为 `borrow`、`lend`、`repay`、`collect`；借入/借出时利息为 `"0"`。还款/收回不能超过剩余本金。负利息表示优惠且不能超过本金；利息非零必须选择实际资金账户。

基金 `POST /monetary-income/save` 最小示例：

```json
{
  "accountId": "456", "code": "000198", "startDate": "2026-09-30",
  "timeZone": "Asia/Shanghai", "enabled": true
}
```

必须先搜索、核对实际基金并由用户明确绑定。仅人民币可用资金账户允许绑定。`bookId`、`categoryId` 可省略；首次使用有效默认值，后续收益继承前一条记录。暂停/解绑使用 `POST /monetary-income/pause` 的 `accountId`，保留历史流水与逐日防重。同步接收 `accountId` 和可选 `force`，缺数据时等待；未强制同步的同一账户尝试间隔为 60 秒，已结算到昨日的账户跳过。成功返回最新收益合计，客户端据此刷新相关页面；强制同步不能绕过防重。

`GET /monetary-income/search` 无法访问公开数据源时返回 HTTP 502、`errorCode=224004` 和中文重试提示。限定基金域名优先使用应用内 HTTPS 解析；经数据源确认的货币基金身份缓存 10 分钟，可用于紧随其后的绑定核验，过期后必须重新查询。具体边界见[行情模块](../pkg/marketquotes/README.md)。


## 资产写入字段

| 操作 | 关键输入与约束 |
|---|---|
| 余额校准 | `accountId`、`balance`、`expectedBalance`、`bookId`、`countInStatistics`、`timeZone`、`requestId`；余额已变化则拒绝 |
| 报销到账 | `expenseId`、到账 `accountId`、`amount`、`time`、`timeZone`、`comment`、`requestId`；同币种且不超过待报额 |
| 报销状态 | `expenseId`、`action`：`end` / `reopen` / `unassign`；已有到账时取消受关联约束 |
| 分期 | `id`/`version`（更新）、`accountId`、`expenseId` 或 `statementMonth`、`principal`、`totalFee`、`periods`、`firstDate`、`method`、`remainder`、`bookId`、`timeZone`、`requestId`；先调 `preview` 再 `save` |
| 结束分期 | `id`、`version`；保留已经发生的费用，不把本金重新入账 |
| 定存 | `FixedDeposit` JSON：账户/账本/分类、币种、本金、年利率、期限/单位、开始日期和时区；到期/预计收益由服务校验，`close` 接收 `id` |
| 资产偏好 | `revision`、`rules`、`reminders`；规则仅影响隐藏/生效账本，不能覆盖余额 |
| 账户删除 | `id`、`kind`（`cash`/`portfolio`）；先调用 `/wealth/accounts/delete/preview`，确认影响范围后带返回的 `token` 和 `deleteRelated` 提交 `/delete`；令牌过期须重做预览 |

精确类型定义位于 `pkg/services/asset_adjustment.go`、`reimbursements.go`、`debt_movements.go`、`credit_installments.go`、`account_deletion.go` 和 `pkg/models/asset_tools.go`。日期、ID 和金额均由服务复核，不信任前端预览结果。


## 投资、账本与历史

投资先调用 `/investments/events/preview` 核对持仓/资金效果，再按相同事实提交；预览不落账。修订/撤销使用路径中的事件 ID 和已读 `version`；重复提交旧版本返回冲突，遇到未知提交结果应先读取当前事件及修订号。字段定义为 `services.InvestmentEvent` 内嵌的 `investments.Event`，并带 `cashAccountId`、`bookId` 和可选转换快照。

手动加密资产通过 `POST /investments/instruments` 提交 `{name,symbol,type:"CRYPTO"}`，省略公开行情绑定字段。名称最多 64 字、显示代码最多 24 个 UTF-8 字节；返回当前用户独立的资产 ID，不按同名代码自动合并或获取行情，再以此 ID 录入持有数量和手动价格。

加密参考兑换 `/investments/conversion` 保存 120 秒有效快照，实际到账可手动修正；过期/不匹配报价不得成为有效历史汇率。缺成本、价格或历史汇率不返回虚构零值。

手动报价 `POST /investments/quotes/manual` 接收 `{instrumentId, price, currency, asOf}`。`price` 为大于零的十进制字符串，`asOf` 为源报价时间；`currency` 为 `CNY` 或 `USD`，省略/空串按 `CNY` 兼容旧客户端，`USD` 仅用于加密资产。私人报价保存原币价格，当前估值读取最新缓存的美元/人民币汇率，保留实际日期、来源和过期状态；缺汇率时仍可保存美元报价，但人民币金额未知。历史快照继续保存当时价格和汇率，历史事实修订后的重建不使用当前汇率。`{instrumentId, automatic:true}` 删除手动覆盖，恢复已有自动来源；没有自动来源时保持未知。

`books/move` 移动流水归属，最多 1000 笔，转账双边同步且不改余额；投资结算不通过普通批量移动修改。归档账本只供查询，成本回放始终包含全部有效历史。


## 错误处理

| 情况 | 处理 |
|---|---|
| 未登录/会话无效 | 重新通过原有认证流程取得会话，不能改 UID 绕过 |
| `ErrInvestmentLinked`，HTTP 409 | 从投资事件修订/撤销，不能单独更改关联资金流水 |
| `ErrInvestmentConflict`，HTTP 409 | 版本变化或同键不同内容；重新读取并让用户确认 |
| `ErrStatisticsConflict`，HTTP 409 | 预算、总结或统计偏好的修订号已过期；保留编辑内容，读取新版本后再提交 |
| `ErrLedgerItemConflict`，HTTP 409 | 愿望/关键词修订冲突、记录不存在，或导入批次同 ID 不同内容/已经撤回；先查询当前记录，不能盲目换键重试 |
| 输入、归属、金额或历史约束，通常 HTTP 400 | 保留表单，显示服务端业务错误，修正后再提交 |
| 网络中断/未知提交结果 | 同一请求键重试并查询结果，不能换键盲目再记一次 |

账务操作可能因过额、超卖、共享额度引用、历史关联或账户状态被拒绝。具体业务错误沿用现有错误信封；不能仅凭中文提示反推稳定错误码。运维配置见 [运维说明](CYLEDGER_OPERATIONS.md)，内部计算边界见 [架构](CYLEDGER_ARCHITECTURE.md)。


## 统计、预算、总结与两级标签

`GET /statistics/preferences` 返回 `revision`（字符串）、`modules`（daily/month/year/custom 四个数组，元素 `{id,visible}`）、`carrySurplus`、`carryDeficit`、`dailyBudgetMode`（remaining/fixed）和 `budgetProgress`（remaining/spent）。POST 提交完整对象及读取时的修订号，每种周期的模块必须完整且不重复。

预算保存示例：

```json
{"id":"","bookId":"账本ID","categoryId":"0","name":"月预算","amount":"1500.00","startDate":"2026-10-01","endDate":"2026-10-31","kind":"monthly","repeat":true,"revision":"0"}
```

`POST /statistics/budgets/save`：`categoryId="0"` 为总预算，否则为本人支出分类；`amount` 大于零，最多 13 位整数和 2 位小数。月规则日期必须为同一月首尾；`kind="custom"` 使用任意有效起止日期且 `repeat=false`。身份由用户、账本、类型、日期与分类确定，修改时不可改变这些键；要调整周期应新增规则。名称最多 64 个字符。返回完整规则和递增 `revision`。删除提交 `{id,revision}` 到 `/statistics/budgets/delete`，仅删除规则，不改账单。

`GET /statistics/notes` 返回当前用户的总结；`POST /statistics/notes/save` 提交 `{id,bookId,period,content,revision}`，`period` 为 YYYY 或 YYYY-MM，`bookId=""` 表示全部账本，正文最多 10000 个字符。预算、总结、偏好修订冲突返回 HTTP 409，客户端应重新读取后处理，不静默覆盖。

`GET /statistics/auxiliary` 返回 `debtActions`（本金流水 ID 到借还动作的映射）与 `feeIds`（债务利息、分期服务费流水 ID），供客户端按当前日期和筛选范围计算辅助汇总。此接口不创建账务。

普通 `transactions/add.json`、`transactions/modify.json` 支持 `discountAmount`：最多 13 位整数和 2 位小数的非负原币字符串，默认零；修改时省略则保留原值。实收付金额沿用原接口的整数分。优惠仅允许正数普通收入/支出，不能附在转账、退款、投资结算或报销到账上；它不会再次修改余额。

普通标签创建/修改接口支持字符串 `parentId`；`"0"` 为一级标签。父级必须是本人的有效一级标签，存在子标签的标签不能降级；父标签不能在仍有子标签时删除。修改省略 `parentId` 保留原层级。分组与父级关系互相独立。
