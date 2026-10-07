# CYLedger 验收与交接

当前源码验证：2026-10-07，`fix/market-data-routing` 行情网络重构。当前手机正式版仍是 2026-10-06 的已安装版本；本次没有升级正式 APK、部署中转或接触正式账本。

## 当前交付

行情采用国内/海外独立连接池，可选自建 HTTPS/WSS 公开行情中转。搜索不再持全局锁等待上游，相同查询合并且各调用方独立取消；基金、证券与 Coinbase 注册标的独立、有界并发刷新。Android 增加按公开行情 socket 选择物理网络及同网络 DNS，网络变化时清理闲置连接。

中转只允许固定公开行情目标，校验低权限令牌、限制响应及缓存大小、HTTP 上游并发和 WebSocket 数量，不转发客户端账户凭据。取消等待不提前释放仍在运行的上游请求额度。原有账本、持仓数量、成本回放、货币基金收益计算、基金确认和备份允许列表未重写；来源时间、未知值、旧报价过期与十进制精度规则保留。

配置、运行命令、边界及验收矩阵见 [行情网络说明](CYLEDGER_MARKET_NETWORK.md)。全部修改位于独立分支；未经合并不影响 `main`。

## 已验证

验证记录：[Actions 37571509495](https://github.com/cheng-yi-cc/CYLedger/actions/runs/37571509495)，源码提交 `790c5b64edd3bfdba3143852aeb431a1a7e6860b`。其后的收尾变更只涉及文档和回归工作流，不改变运行代码。

| 检查 | 结果与范围 |
|---|---|
| Go 并发回归 | `go test -race -count=1 -timeout=5m ./pkg/marketquotes ./pkg/marketrelay ./android/backend` 通过；38 个顶层测试通过，含子用例共 57 项通过；2 个需显式开启的真实数据源联网测试跳过 |
| 行情回归 | 慢加密搜索不阻塞基金、独立取消/查询合并、快速返回部分结果但不缓存为完整目录、基金阻塞时证券仍刷新、并发上限及既有净值/收益/报价/汇率规则 |
| 中转回归 | 白名单、鉴权、缓存及凭据隔离、上游 429 不缓存、调用方取消仍占用上游额度、模拟上游的完整 WebSocket 往返 |
| 配置与 Java | Python 配置校验 3 项通过；Android Java 源码编译通过 |
| 原生构建 | 中转服务构建通过；Android ARM64 Go 共享库和 JNI 共享库编译通过 |
| 静态检查 | `gofmt`、Python 脚本语法及 `git diff --check` 通过 |

这些结果证明代码构建及受控回归，不代表中国大陆网络实测。没有打开 `CYLEDGER_LIVE_MARKET_TEST`；模拟中转往返不等于真实 Coinbase 服务已连通。

## 未完成与外部依赖

尚未部署中转、配置生产域名/HTTPS/令牌、测量冷/热查询耗时，或在真机验证 VPN 开关、禁止旁路、Wi-Fi/蜂窝切换。远程电脑连接离线，无法构建同签名 APK 并覆盖安装；本次 Android 验证止于 Java/原生编译，不是完整 APK 或正式升级。

未部署可从使用地直达的中转时，关闭 VPN 后海外源仍可能不可达。Windows 的直连连接池不能绕过系统 TUN；Android 也遵守 VPN 禁止旁路及系统锁定规则，不能保证任意 VPN 下都直连成功。

前端仍按现有约 10–15 秒轮询汇总，没有新增逐笔 SSE/WebSocket 推送。Coinbase 实时流沿用原有已验证订阅范围；其他绑定标的及 CoinGecko 保留各自刷新预算，不宣称所有币种逐秒更新。基金净值、万份收益和参考汇率只显示来源已公布的数据。

## 既有正式版与数据保护基线

上次手机正式版验收为 2026-10-06，设备 `23113RKC6C` / Android 16 / ARM64。原验收记录包含理财页面、虚构 QA 账本买卖/确认/定投/编辑/撤销，以及正式版同签名保留数据升级。本次不重复声明这些场景已在新网络实现上完成真机回归。

- 正式 APK 原路径：`D:\My Project\CY Finance Manager\dist\android\CYLedger-Android-arm64.apk`；版本 `0.2.0` / `versionCode=3`，67,602,171 字节。
- 原产物 SHA-256：`cf27bd1bf447ca6e7bf2e17f12559d9e28618f22e7b04d5e88b75ccd18b3fc17`。这不是本次网络版本的 APK 摘要。
- `.runtime/android-signing/`、账本、附件、配置、会话资料及备份必须保留。正式升级只用原签名 `adb install -r`，不得卸载、清数据或把 QA 账本替换到正式应用。
- 原有日常账务、多账本、债务/报销、统计预算、离线导入、钱包收支及完整备份功能保留。完整备份/恢复和账务全量 SQLite 集成测试未在本次网络改动中全部重跑。

## 本地预览

在拉取该分支的独立源码目录运行，勿覆盖正在使用的正式目录：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\Start-CYLedger.ps1 -Build -DirectNpm
```

打开 `http://localhost:8080/mobile`，进入“资产 → 理财 → 新增基金/加密资产”或人民币资金账户的“收益同步”。手机预览必须使用独立 QA 版；配置中转后按 [行情网络说明](CYLEDGER_MARKET_NETWORK.md) 构建，先完成网络矩阵，再考虑同签名正式升级。
