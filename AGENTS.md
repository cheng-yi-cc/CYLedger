# 项目约定

## 技术与边界

- 这是 ezBookkeeping v2.0.0 的二次开发，保留 Go 模块路径、Vue、Vuetify、Framework7、Pinia 和 SQLite；不要另建无关前端或重写日常账务。
- 用户界面与新增项目文档默认简体中文，本位币固定为人民币；会计时区取用户设置。
- 投资事实在 `pkg/services/investments.go` 原子提交，成本回放在 `pkg/investments`，公开行情在 `pkg/marketquotes`，统一估值在 `pkg/services/wealth.go`。
- 新接口所有数量、金额、价格和汇率均为十进制字符串。输入先限制格式和长度，再交给十进制库；禁止浮点参与成本运算。
- 未知成本、缺失报价或历史汇率保持未知，不能当成零。普通估值行情不能生成收入或修改持仓数量；用户显式绑定的货币基金自动收益由 `pkg/services/monetary_income.go` 独立逐日结算，万份收益不得当作净值。
- 系统结算账户必须在旧接口、批量操作、账户合并、普通列表和资产汇总中受保护。
- 钱包收支复用投资事件，在同一事务更新实际币种持仓及一笔人民币统计账单；系统账单不重复计入资产，原币展示来自原事件，禁止普通账单接口单独改删。账户单位切换不得重写历史币种，普通资金账户须由 `account_currency.go` 复核可改条件。
- 稳定币每枚约 1 美元只能用于可修改的收付数量预填，不能作为行情或历史汇率；手动价格保留原币，历史快照重建沿用原有价格与汇率。
- 不记录密钥或完整财务数据，不把 `.runtime`、`runtime`、备份和测试令牌提交到仓库。
- 报销到账、债务本金/利息、余额校准、分期费用与定存收益必须在对应服务中原子提交并防重；定存和分期不能再次增加本金，普通行情不能驱动这些入账。
- `Account.Extend.AssetProfile` 只存展示与账户规则，不维护第二份余额；新增表模型为 `Book`、`CalendarEvent`、`AssetPresentation`、`ReimbursementReceipt`、`AssetAdjustment`、`DebtMovement`、`CreditInstallment`、`FixedDeposit`、`MonetaryIncomeBinding`、`MonetaryIncomeDay`、`StatisticsBudget`、`StatisticsNote`、`StatisticsPreference`、`LocalLedgerItem`、`InvestmentHoldingProfile`、`InvestmentPlan`、`InvestmentOrder`，与投资模型统一在 `cmd/database.go` 注册。
- 基金定投/待确认入账须通过 `investment_plans.go` 原子完成指令、投资事件和资金流水；暂停/删除计划或撤销事件不能移除每期防重身份。展示偏差不改真实盈亏，持仓数量/成本校准必须留下 `ADJUST` 事实。
- 基金解绑、重绑、删除收益不得删除逐日防重记录；账户级联删除须核对预览令牌，保护转账对端余额和历史投资依赖。
- 转账手续费与原转账同事务提交、修订、删除；借款约定日期关联日历事项，禁止脱离原账单独立改金额/日期。导入批次须永久幂等，撤回整体事务执行。
- 预算由账务事实重算，不能维护另一份余额或受临时统计筛选影响；优惠不再次改实收付金额，一级含子级标签统计须按账单去重。预算、总结和统计偏好保存必须校验修订号。
- API 在 `cmd/webserver.go` 注册：`/api/v1/books/*`、`calendar/*`、`ledger/*`、`statistics/*`、`assets/*`、`monetary-income/*`、`investments/*`、`wealth/*`；完整方法与字段以接口文档和代码为准。

## 本地运行与验证

- 本地启动：`powershell -ExecutionPolicy Bypass -File .\scripts\Start-CYLedger.ps1`；打开 `http://localhost:8080/`。
- 重建加 `-Build -DirectNpm`。本机工具路径见 `README.CYLEDGER.md`，首次创建账户见 `docs/CYLEDGER_OPERATIONS.md`。
- 账务改动运行相关 Go 核心与 SQLite 集成测试；界面改动做类型检查及适当浏览器验收；电脑备份改动运行 `scripts/test_backup.py`，手机备份改动检查 `pkg/localbackup`、`android/backend` 并用 QA 验证附件和设置恢复。
- 手机已连接电脑时，涉及手机界面或手机账务流程的改动默认一步完成：构建 APK、在已连接真机验收、同签名覆盖升级正式版，并给出手机内的预览路径；不以仅浏览器验证或“尚未打包 APK”结束交付。使用独立测试版及虚构数据验证删除等操作，不能删除、清空或覆盖正式账本；正式版升级使用 `adb install -r`，不得先卸载或清除数据。设备未连接或安装授权受阻时明确报告。
- 不为可逆、影响小、只是复述实现的改动添加测试。必要检查通过后不无故扩大测试范围。
- 加密今日收益必须用会计零点的历史收盘价和已发布历史汇率回放当天交易，不得用 24 小时涨跌或当前汇率替代；同币汇总只认公开唯一身份。
- 修改历史事实必须使快照失效，并只用快照保存的历史价格重建。金额、数量、成本和关联在备份恢复后必须一致。
- 默认在原文件直接修改。用户明确结束任务或要求清理时，才清理过程文件与预览；不能删除正式账本。
- 每次交付项目修改都告诉用户本地预览方法；最终报告区分已实现、已验证、未实现和依赖外部配置的部分。
- Android 构建用 `python scripts/build_android.py`，测试版加 `--qa`；`.runtime/android-signing/` 是覆盖升级所需的持久材料，清理时必须保留。手机与电脑数据库独立。
- 手机备份只保存允许的显示偏好，不保存会话令牌和设备解锁凭据；恢复须先校验、备份当前账本并停止后端，再替换目录。愿望记录不能当作真实资金流水。
- Compose 参数为 `CYLEDGER_BIND`、`CYLEDGER_PORT`、`CYLEDGER_ROOT_URL`、`CYLEDGER_VOLUME`、`TZ`；可选行情密钥为 `CYLEDGER_COINGECKO_DEMO_API_KEY`，Android SDK 用 `ANDROID_HOME` 或 `--sdk`。不得提交真实密钥或修改全局代理来适配一次构建。

## 深入文档

| 文档 | 用途 |
|---|---|
| `README.CYLEDGER.md` | 使用与开发入口 |
| `docs/CYLEDGER_ARCHITECTURE.md`、`docs/CYLEDGER_API.md` | 数据模型、事务、路由和请求契约 |
| `docs/CYLEDGER_OPERATIONS.md`、`android/README.md` | 部署、恢复、签名和真机升级 |
| `docs/CYLEDGER_ACCEPTANCE.md` | 当前交付与已验证范围，替换过期记录而非追加流水账 |

## 上游归属

`LICENSE` 为 Apache 2.0，适用于 CYLedger 新增与修改部分。必须保留 `NOTICE`、`licenses/ezbookkeeping-MIT-LICENSE` 中的上游版权和 MIT 许可，以及其他第三方声明。新增依赖须更新归属；不要直接复制强互惠许可项目的实现后改标 Apache 2.0。
