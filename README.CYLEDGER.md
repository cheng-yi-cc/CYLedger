# CYLedger 初版

简体中文、人民币本位的个人记账与投资资产管理工具。日常收入、支出、转账、分类、模板和 CSV 导入沿用 ezBookkeeping；投资账户、股票/基金手动估值及加密资产行情在同一 Vue / Go 应用中运行。

公开仓库：[cheng-yi-cc/CYLedger](https://github.com/cheng-yi-cc/CYLedger)。CYLedger 新增与修改部分使用 [Apache License 2.0](LICENSE)。上游固定为 `mayswind/ezbookkeeping` 的 `v2.0.0`，提交 `b2a3f4ede42c8a7aa3bab1e6dbbc7b8e6196ef35`，其 MIT 许可证完整保存在 [licenses/ezbookkeeping-MIT-LICENSE](licenses/ezbookkeeping-MIT-LICENSE)，模块路径保持不变。归属见 [NOTICE](NOTICE)；decimal 依赖的许可位于 `licenses/shopspring-decimal-LICENSE`。

## 本机预览

本次本机交付已经运行在 **http://localhost:8080/**，可直接打开。若 8080 已运行本应用，不要重复启动；服务停止后，在项目根目录运行：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\Start-CYLedger.ps1
```

桌面默认进入账单页；手机入口为 **http://localhost:8080/mobile**，底部固定为“首页、日历、资产、统计、我的”五页，保留浅色、深色及跟随系统主题。本次本机实例的用户名为 `cyledger`，密码只保存在私密文件 [.runtime/LOCAL_LOGIN.txt](.runtime/LOCAL_LOGIN.txt)。这是本次交付创建的个人账号，不是通用安装的默认账号；其他机器首次安装请按 [运维说明](docs/CYLEDGER_OPERATIONS.md#创建自己的登录账户) 创建自己的账户。本机实例没有验收用账户余额、持仓或流水，详细验证结果见 [验收报告](docs/CYLEDGER_ACCEPTANCE.md)。

空账本可以按这个顺序开始使用：

1. 在“账户”页建立银行卡、现金等日常账户，填写实际余额。如果计划导入历史流水，应填写对应的历史起始余额；只从今天开始记账才填写当前余额。
2. 配置收入、支出分类，然后开始日常记账；常用记录可保存为模板。
3. 手机进入“资产 → 投资理财 → 账户与资产”（桌面直接进入“资产 → 账户与资产”），建立投资账户；再选“记录投资 → 期初持仓”，录入已有资产数量，成本知道就填，不知道可以留空。

不要把已包含历史流水的当前余额再与整段历史导入叠加，也不要将同一笔已有持仓同时录为期初和新买入。

从源码重新构建前，先停止现有服务，再运行。前台服务可按 `Ctrl+C`；本次后台预览请按[运维说明中的进程校验步骤](docs/CYLEDGER_OPERATIONS.md#停止本次后台预览)停止，避免陈旧 PID 指向其他程序。

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\Start-CYLedger.ps1 -Build -DirectNpm
```

本机 Go / GCC 位于 `D:\tools\cyledger\go\bin` 和 `D:\tools\cyledger\mingw64\bin`；启动脚本会自动发现。`-DirectNpm` 仅为本次命令使用独立 npm 配置，绕过本机失效的全局代理，不修改全局设置。原有 Node.js 可继续使用。

开发前端时先启动后端，再运行 `npm run serve`（默认端口 8081）。可通过环境变量 `CYLEDGER_DEV_BACKEND` 指向其他后端，例如 `http://127.0.0.1:18080/`。

## 已实现的流程

- 手机首页显示月度收支与每日账单；日历可切换月份、查看指定日期并带入日期记账；资产按资金、信贷、债务应收及投资分组；统计支持日、月、年和自定义区间、收支图、已有资产快照及分类对比；“我的”集中提供资料、模板、周期记账、分类标签、图片、数据管理与设置入口。
- 本次手机调整只整合现有功能，没有新增预算、报销、愿望清单、自动/语音记账。设置同步只同步偏好设置；CSV 映射导入继续在电脑版完成，完整备份继续使用运维脚本。
- 日常账单、账户、收支统计、分类模板、编辑查询、通用 CSV 映射预览与导入；迁移前明确选择历史流水或当前余额方式。
- 投资账户与资产定义；预置 BTC、ETH、SOL、USDT、USDC，私人资产支持 CRYPTO、STOCK、FUND、OTHER。
- 期初持仓、买入、卖出、账户间转移；支持法币和另一投资资产结算，支持结算币手续费及转移同币网络费。
- 每账户每资产的移动加权成本；未知成本保留未知；18 位十进制输入、36 位除法；完整卖出清空成本尾差。
- 预览后确认入账、持仓事件修订/撤销与全历史重算；幂等请求、版本冲突、并发超卖保护。
- 与原有资金转账同事务提交；系统结算账户有独立角色，不计入资产、不出现在普通账户选择器，关联流水不能从旧接口单独改动。
- 服务端共享 Coinbase WebSocket 与心跳，REST 降级、重连与乱序保护；人民币汇率有独立日期与来源；可选 CoinGecko 15 分钟批量备用。
- 同一估值服务计算日常现金、投资、负债、缺价及过期状态；账户/资产两种持仓视角；分布图和使用后积累的历史记录。
- 交易修订后用历史快照保存的报价/汇率重新计算当时持仓和现金；历史没有价格的部分保持不可计算。
- 投资流水（含费用及转入账户）与当前持仓 CSV；全部 SQLite 表、附件、配置及校验清单的完整备份/空实例恢复。
- 手机与电脑页面、PWA 应用外壳缓存、未同步的本地草稿、退出时清理私人草稿。

## 行情与使用范围

本机实际验证 BTC、ETH、SOL、USDT 可收到 Coinbase 自动报价。USDC 没有经验证的对应 Coinbase 美元交易对：未配置 CoinGecko 时会显示无法自动估值，可录入手动价格。所有新建私人资产默认为手动估值，不能仅凭相同缩写自动匹配。

CoinGecko Demo 密钥仅放服务端环境变量 `CYLEDGER_COINGECKO_DEMO_API_KEY`；未配置不调用该备用源。股票/基金初版支持份额、成本和手动价格，尚未接入自动报价。

这是自托管账本，不包含交易所下单、转币、钱包扫描、杠杆/DeFi、税务计算、多人协作或多设备离线合并，也没有实现端到端加密。不会要求交易所密钥、私钥或助记词。行情变动只改变估值。

历史快照在查看资产页时最多每 5 分钟保存一次，后台每 15 分钟采样已配置用户；不是逐笔历史行情库。接口展示最近 2000 个快照，数据库和完整备份保留所有记录。初始化以前没有历史，不补造旧曲线。

## 部署与维护

仓库默认分支为 `main`。原有上游多平台发布与翻译徽章工作流仅在上游仓库执行；本仓库不自动向上游或其镜像仓库发布。可按下列方式自行构建和部署。

```sh
docker compose up -d --build
```

Compose 仅启动一个应用，将持久化命名卷挂载到 `/app/runtime`；自动生成并保留签名密钥，默认关闭公开注册。首次账户、配置、日志、健康检查、备份、恢复、升级步骤详见 [运维说明](docs/CYLEDGER_OPERATIONS.md)。Docker 配置检查与容器实际运行的验证状态分别见 [验收报告](docs/CYLEDGER_ACCEPTANCE.md)。

## 验证命令

```sh
go test ./pkg/investments ./pkg/marketquotes ./pkg/services ./pkg/datastore ./pkg/models ./pkg/converters/custom
npx vue-tsc --noEmit
npx vitest run src/lib/__tests__/ledger_display.test.ts
npm run build
python -m unittest discover -s scripts -p test_backup.py -v
docker compose config --quiet
```

真实行情联网测试需显式设置 `CYLEDGER_LIVE_MARKET_TEST=1`，再执行 `go test ./pkg/marketquotes -run TestLivePublicProviders -count=1 -v`。普通测试不依赖公网。

原始需求保存在 [开发任务参考](docs/CYLEDGER_BRIEF.md)，不应将其写成已实现/已验证清单；实际结果以代码和验收报告为准。
