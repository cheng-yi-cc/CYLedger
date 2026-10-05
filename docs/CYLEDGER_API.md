# CYLedger 接口指南

接口入口为 `/api/v1`，复用 ezBookkeeping 登录鉴权，客户端使用 `Authorization: Bearer <token>`。用户身份来自鉴权上下文，不能用请求体中的 UID 代替。正式 Android 自动取得本机会话的机制只供内嵌 WebView，不是外部登录 API。

接口注册以 `cmd/webserver.go` 为准。新增路由清单如下，普通账户、流水、分类、模板及导入导出仍沿用上游接口；相关新增字段见 `pkg/models/account_asset_profile.go`、`transaction.go`、`book.go`。

## 路由清单

| 方法 | 路径（省略 `/api/v1`） |
|---|---|
| GET | `/books/list` |
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
- 投资创建/修订/撤销及加密账户创建使用 `Idempotency-Key` 请求头；余额校准、报销到账、债务和分期使用请求体 `requestId`。重试保持同一键及同一内容，改内容须创建新请求。
- 投资修订、分期更新与资产偏好使用返回的版本/修订号。冲突后重新读取与预览，不盲目覆盖。
- `GET /wealth/summary` 可能保存估值快照；`POST */sync` 可能写入到期费用/收益，不能当作无副作用的健康检查。健康检查使用 `/healthz.json`。

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

必须先搜索、核对实际基金并由用户明确绑定。仅人民币可用资金账户允许绑定。`bookId`、`categoryId` 可省略；首次使用有效默认值，后续收益继承前一条记录。暂停/解绑使用 `POST /monetary-income/pause` 的 `accountId`，保留历史流水与逐日防重。同步接收 `accountId` 和可选 `force`，缺数据时等待，不能绕过防重。

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

投资先调用 `/investments/events/preview` 核对持仓/资金效果，再按相同事实提交；预览不落账。修订/撤销使用路径中的事件 ID、原版本和新的幂等键。字段定义为 `services.InvestmentEvent` 内嵌的 `investments.Event`，并带 `cashAccountId`、`bookId` 和可选转换快照。

加密参考兑换 `/investments/conversion` 保存 120 秒有效快照，实际到账可手动修正；过期/不匹配报价不得成为有效历史汇率。缺成本、价格或历史汇率不返回虚构零值。

`books/move` 移动流水归属，最多 1000 笔，转账双边同步且不改余额；投资结算不通过普通批量移动修改。归档账本只供查询，成本回放始终包含全部有效历史。

## 错误处理

| 情况 | 处理 |
|---|---|
| 未登录/会话无效 | 重新通过原有认证流程取得会话，不能改 UID 绕过 |
| `ErrInvestmentLinked`，HTTP 409 | 从投资事件修订/撤销，不能单独更改关联资金流水 |
| `ErrInvestmentConflict`，HTTP 409 | 版本变化或同键不同内容；重新读取并让用户确认 |
| 输入、归属、金额或历史约束，通常 HTTP 400 | 保留表单，显示服务端业务错误，修正后再提交 |
| 网络中断/未知提交结果 | 同一请求键重试并查询结果，不能换键盲目再记一次 |

账务操作可能因过额、超卖、共享额度引用、历史关联或账户状态被拒绝。具体业务错误沿用现有错误信封；不能仅凭中文提示反推稳定错误码。运维配置见 [运维说明](CYLEDGER_OPERATIONS.md)，内部计算边界见 [架构](CYLEDGER_ARCHITECTURE.md)。
