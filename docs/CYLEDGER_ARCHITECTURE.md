# CYLedger 架构

同一 Go 模块提供原有记账、新增投资与资产服务，Vue 手机侧使用 Framework7，桌面侧使用 Vuetify，状态使用 Pinia。SQLite 保存原始账务、关联及可重建快照。未拆分微服务或另建前端。

## 数据流与边界

```text
Vue 网页 / Android WebView
  → 原有鉴权和 cmd/webserver.go
  → pkg/api：有界输入、当前用户
  → pkg/services：归属验证、用户锁、同一数据库事务
  → 原始账户/流水 + 投资事件 + 业务关联
  → pkg/investments 成本回放 / wealth.go 统一估值
```

`pkg/marketquotes` 只处理公开资产身份、行情和汇率，不接收持仓或用户财务数据。`pkg/monetaryincome` 计算起息本金；`pkg/services/monetary_income.go` 在显式绑定后独立生成每日收益。

## 数据模型

以下为注册于 `cmd/database.go` 的 Go 模型名。物理表名由 XORM 与数据库配置生成，不应自行假定复数表名。

| 模型/字段 | 职责 |
|---|---|
| `Book` | 默认、归档、排序及显示偏好；流水/模板/投资事件携带账本归属，账户仍共用 |
| `CalendarEvent` | 手动或借款账单关联的到期事项，完成状态不产生现金流水 |
| `LocalLedgerItem` | 愿望、关键词规则、永久幂等的导入批次；不是第二份账户余额 |
| `StatisticsBudget` / `StatisticsNote` / `StatisticsPreference` | 预算规则、账本与周期总结、各周期模块顺序及预算口径；修订号避免并发覆盖 |
| `Transaction.DiscountAmount` / `TransactionTag.ParentTagId` | 原币优惠金额、两级标签关系；优惠不改实收付，两级关系同用户且无循环 |
| `Account.Extend.AssetProfile` | 类型、分组、简称、卡号、夜间图标、资产统计选择、信用规则与借款日期；没有第二份余额 |
| `AssetPresentation` | 隐藏、生效账本和提醒偏好，带修订号 |
| `ReimbursementReceipt` | 原支出与到账流水关联；待报额由有效事实推导 |
| `AssetAdjustment` | 余额校准请求摘要和差额流水，用于幂等 |
| `DebtMovement` | 借还款本金与利息流水关联、请求摘要 |
| `CreditInstallment` | 来源支出/账单、版本、各期本金/服务费与已入账关联 |
| `FixedDeposit` | 原账户内的定存本金、期限、利率、到期/关闭及收益流水 |
| `MonetaryIncomeBinding` / `MonetaryIncomeDay` | 基金绑定与逐日本金、万份收益、金额、流水和永久防重身份 |
| `InvestmentSettings` / `PortfolioAccount` / `InvestmentInstrument` | 本位币/时区、投资账户及资产身份/公开行情绑定 |
| `PortfolioAccount.Currency/PaymentInstruments` | 钱包/交易所显示与新增收支默认单位、至多两项默认收付币种；不保存第二份现金余额 |
| `InvestmentHoldingProfile` | 单项理财名称、分组、备注、盈亏偏差、生效账本和资产计入规则；不保存余额 |
| `InvestmentPlan` / `InvestmentOrder` | 定投周期与游标、每期永久身份、基金确认状态、净值来源和已入账事件关联 |
| `InvestmentEventRecord` / `InvestmentEventRevision` | 当前投资事件与历史修订，保留版本/作废状态 |
| `CryptoDCAPlan` / `CryptoDCADay` | 显式每日定投规则、逐日固定条款与永久防重身份；不维护另一份持仓 |
| `InvestmentTransactionLink` / `InvestmentIdempotency` | 事件与普通资金流水关系、重复请求结果 |
| `InvestmentQuote` / `WealthSnapshot` | 公共行情持久缓存、私人手动价和历史估值快照 |

普通账户金额沿用整数分；新增 API 的金额、数量、价格、汇率使用十进制字符串，禁止浮点进入成本计算。投资数量/价格最多 18 位小数，成本除法 36 位；输出展示舍入不改变保存事实。

## 手机工作区与完整备份

`LocalLedgerItem` 保存 `wish`、`keyword` 和内部 `import` 批次。愿望/规则请求校验用户归属、引用和修订号；金额采用十进制字符串，愿望进度是计划或从事实推导的显示值，不生成账户余额。显示偏好位于按本机档案隔离的 `cy_ledger_experience_*`；备份白名单只允许此项和 `ebk_app_settings`。

`Transaction.TransferFeeAmount/TransferFeeCategoryId/TransferFeeParentId` 关联转账与手续费收支。创建、修订、删除在原事务中更新账户；关联手续费禁止被普通批量入口单独改动。移动账本和账户级联删除同步处理关联。`DebtDueDate` 关联 `CalendarEvent.TransactionId`，账单修改/删除/移动同步更新提醒；关联事项不能独立修改日期金额或删除，完成标记不修改账务。`LocationName` 保存用户输入的地点。

导入沿用已有解析器和批量入账服务；用户/批次 ID 的永久摘要支持重启后幂等，内容改变或已撤回的批次 ID 拒绝重用。批次撤回在一个事务中删除尚存在的原始账单，恢复余额并沿用投资、报销及关联保护，失败整体回滚。

Android `LocalBridge` 只允许本机页面且解锁后发起操作。备份同时获得请求、定时任务写门禁，用 SQLite 一致快照打包附件和白名单设置；恢复在私有维护进程中停止后端后替换目录，保留恢复前目录及日志回退。外部网页不在此 WebView 执行。

## 核心流程

**投资**：事件、资金账户与系统结算对手的双边流水、关联和幂等记录在同一事务提交。持仓按用户/账户/资产重放全部有效历史；筛选账本只筛选事件显示，不截断成本历史。普通接口不能修改投资结算，系统账户从正常选择、资产统计和删除中排除。历史修订使相关快照失效，只用已保存历史价格重建。

**手机理财**：`investment_holdings.go` 将新增账户、资产、展示规则与期初事实一次提交；编辑份额/成本使用 `ADJUST` 事件记录目标持仓，校验原数量/成本及规则版本。只改展示信息保留回放所得的完整成本精度，不生成校准事实。`wealth.go` 的 `totalValue` 应用“不计入总资产/盈亏不计入”规则，原市值、成本和盈亏仍保留；历史快照保存当时规则。货币基金从原资金账户显示，累计收益仅统计逐日关联且仍有效的实际收入。

**基金定投和确认**：`investment_plans.go` 在用户锁与事务中生成 `plan:<计划ID>:<日期>` 永久指令，事务外读取 `FundHistory` 已公布单位净值，再在一个事务调用投资入账并完成指令。数量、金额、费率、成本运算都使用十进制；货币基金万份收益不得参与份额确认。缺净值/现金不足保留待确认；暂停阻止入账，删除取消待确认，撤销已入账事件不删除每期标记。账户删除预览令牌包含计划和待确认状态，清理普通账务暂停定投。前台每分钟触发，后端按上次尝试限流并限制单批补齐数量及网络查询；不提供应用退出后的系统唤醒。

**理财统计**：`investment_report.go` 以有效事件回放计算交易效果，资产结算收益按筛选资产归属，组合币种不能准确拆分时保留未知。历史曲线仅从有效快照取值；年化使用资金占用日的简单估算，不伪装成复合收益率。

**钱包收支**：沿用投资事件的 `INCOME/EXPENSE` 和 `wallet` 元数据，保存原币金额、确认的人民币历史汇率、分类和一至两种实际币种数量。`wallet_entries.go` 验证归属与分类，`pkg/investments/wallet.go` 回放数量和成本，`InvestmentService.Mutate` 在同一事务写入事件、修订、关联及一笔人民币收支账单；预览执行后整体回滚。收入取得成本为原金额乘历史汇率；混合支出的处置价值按实际数量分配，释放移动加权成本，未知成本继续未知。人民币统计账单记在受保护的系统账户，不再次形成资产或现金扣款；原币展示从事件读取，修订/撤销同步反转账单、重放持仓并使快照失效。

**货币单位与估值**：钱包可切换人民币/美元展示和新增收支默认值，历史事件币种不随之改变。普通资金账户没有独立的历史币种字段，只有零余额、无全部历史（包括已删除）、无计价规则或共享额度依赖时允许修改币种，由 `account_currency.go` 在保存事务中复核。私人手动价格保存原币，当前人民币估值使用缓存汇率；历史快照重建只用快照原有的价格与汇率。稳定币每枚约 1 美元仅用于可修改的收付数量预填，不进入行情缓存。

**加密每日定投**：用户在交易所账户显式启用规则后，`crypto_dca.go` 按保存时区生成逐日安排；客户端前台及重新打开时触发同步，不依赖手机永久后台。历史行情在数据库事务外获取，两个币种均使用指定分钟公开美元开盘价，人民币成本使用此前已公布的 ECB 汇率。随后在 `investments.go` 原有事务内复核归属、计划修订和待处理状态，回放扣款时及后续稳定币余额，原子保存买入、修订、幂等、快照失效和逐日完成标记。缺数据保留待处理，余额不足直接暂停并跳过尚未买入的安排；恢复只从未来时间继续。人工撤销不删除日期身份；账户删除的预览令牌覆盖计划，删除时同步停止计划。累计投入来自有效投资事实，持仓成本和收益复用原回放与统一估值。

**货币基金收益**：用户通过代码/名称搜索并明确开始日期；按会计时区、交易日和 15:00 截止回算 D 日本金，扣除有效定存，按 `本金 × 万份收益 ÷ 10000` 四舍五入到分，在 D+1 日入账。缺日停止等待；沿用上一笔收益的账本、分类、标签和收支统计属性。重绑、解绑和删除收益均保留防重身份，不复活已处理日期。

基金公开接口的域名解析由 `pkg/marketquotes/fund_dns.go` 限定处理，优先 HTTPS 解析、失败回退系统解析，仍服从手机 VPN 路由；不拆分应用或修改 VPN 设置。加密行情使用自己的公开源与共享刷新限额。连接预算、缓存期限与缺价策略见 [行情模块](../pkg/marketquotes/README.md)。

**报销**：普通支出携带报销对象，实际到账关联原支出并减少应收额；余额/资产快照根据有效支出、到账和结束状态计算。报销支出及到账从生活收支排除。已到账金额和历史修改相互约束，不能超额报销或留下无来源关联。

**债务**：有资金账户时本金为同币转账；利息/优惠使用独立收支，二者原子提交。无资金账户时只记外部本金，排除生活统计且不能同时记利息。还款/收回受剩余本金约束，请求重试不会重复写入。

**信用卡与定存**：信用账期从实际流水计算；分期只分配已存在本金及费用，不再次扣本金，尾差精确到分，费用在约定日防重入账。定存占用原余额，不创建第二份资产，到期仅产生收益。还款提醒与计划不执行实际银行还款。

**余额与删除**：校准以 `expectedBalance` 检查并发并新增差额流水。账户删除先预览关联范围，再确认；事务内撤销转账双边、作废投资、处理报销/债务/计划及模板、重放持仓并使快照失效。引用变化或无法成立的后续持仓导致回滚。

**统计与预算**：`statistics_workspace.go` 持久保存预算规则、总结和模块偏好，使用用户归属验证及修订号比较更新。Vue 的 `statistics-report.ts` 使用精度 40 的 Decimal 重放支出、退款与月度结转；坐标绘制才转为数值。总预算和分类预算各自统计，不相加。临时账户/标签筛选不影响预算事实。下钻链接携带原账本与筛选条件；日期桶和日均使用用户会计时区。普通外币流水没有确认的历史汇率时保持未知，资产走势只展示已保存且有效的历史快照。

两级标签在普通标签表扩展，不复用标签分组。父与子必须同一用户，不能自引用、循环或超过两级；存在子标签时不能删除父标签或将其降为二级。一级含子级统计按账单去重。优惠金额与普通收支在同一事务保存，为原币十进制字符串；退款、转账、投资结算和报销到账不能携带非零优惠。旧客户端省略修改字段时保留原优惠和标签父级。

手机账户选择与资产页共用 `src/lib/asset-tools.ts` 的分组排序规则，再平铺可选账户；保留隐藏、账本和交易类型约束。普通新增收入/支出选钱包时进入 `WalletEntryPage.vue`，币种不同时不把已有现金金额直接当成钱包金额。银行卡自动标志由 `src/lib/banks.ts` 解析名称，自上传图标优先。

## 路由

路由统一挂在 `/api/v1` 并使用原有鉴权。新增路由如下；字段、请求例子及错误处理见 [接口指南](CYLEDGER_API.md)。普通账户/交易接口继续复用上游路由。

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
| GET | `/investments/dca` |
| POST | `/investments/dca/save`、`/investments/dca/enabled`、`/investments/dca/sync` |
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

## Android 与部署

Android 通过 JNI 在应用进程内运行相同后端，正式/QA 分别使用回环端口 18761/18762 和各自私有目录。个人会话仅由原生外壳交给本应用 WebView；正式包禁用调试和注册。`ReminderBridge`/`ReminderReceiver` 处理资产提醒，`DailyReminderReceiver` 处理每日记账提醒，`LedgerTileService` 打开手动记账；这些入口不能代替后台常驻服务。

Windows 和 Compose 的配置、签名密钥、数据库及附件放在持久运行目录，不能提交仓库。启动时 XORM 同步模型，随后 `Books.MigrateAll` 为历史事实补默认账本。迁移前备份、恢复校验与回退见 [运维说明](CYLEDGER_OPERATIONS.md)。没有独立下游项目；Android 是同仓库内的消费者。
