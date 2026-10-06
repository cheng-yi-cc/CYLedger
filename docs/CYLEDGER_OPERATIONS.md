# CYLedger 运维说明

本页处理电脑/服务器实例。Android 的本机运行、签名和升级见 [Android 说明](../android/README.md)，手机使用“我的 → 数据备份”，与本页电脑停服备份的格式和操作不同。

## Windows 启动

需要 Go/CGO、GCC、Node.js/npm 和 Python。本机工具路径见 [开发入口](../README.CYLEDGER.md#本机开发)。在项目根执行：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\Start-CYLedger.ps1 -Build -DirectNpm
```

打开 [http://localhost:8080/](http://localhost:8080/)；手机布局为 [http://localhost:8080/mobile](http://localhost:8080/mobile)。已构建时省略 `-Build -DirectNpm`。脚本在前台运行，按 `Ctrl+C` 停止；不要重复启动已占用端口的实例。

`-DirectNpm` 仅使用运行目录内的独立 npm 配置绕过失效代理，不改变全局设置。脚本支持 `-Port 18080`、`-BindAddress 0.0.0.0`、`-RuntimeDirectory <独立目录>`、`-ConfigureOnly`。默认只监听 `127.0.0.1`，局域网监听不等于公网部署。

| 持久路径 | 用途 |
|---|---|
| `.runtime/cyledger.exe` | 电脑服务程序 |
| `.runtime/cyledger.ini` | 私密配置与自动生成的签名密钥 |
| `.runtime/data/cyledger.db` | SQLite 全部业务表 |
| `.runtime/storage/` | 附件、头像和自定义图标 |
| `.runtime/log/` | 服务日志 |
| `.runtime/android-signing/` | Android 持久签名材料，禁止作为临时文件清理 |
| `backups/` | 建议存放完整备份，受 Git 忽略 |
| `dist/` | 网页构建及 Android 最终 APK |

若另行安排后台运行，停止前必须核对进程的绝对程序路径、配置和监听端口。旧 PID 文件不证明进程仍属于本应用；不能只按陈旧 PID 结束进程。

## 创建自己的登录账户

公开注册默认关闭。服务首次启动完成数据库初始化后另开终端：

```powershell
python .\scripts\create_user.py --binary .\.runtime\cyledger.exe --config .\.runtime\cyledger.ini --username cy --email your-address@example.com --nickname 我的账本
```

按提示输入密码，默认记账货币为人民币。密码不写入 shell 历史；上游 CLI 仍以短暂进程参数接收密码，应在可信本机执行。已有本机个人账户信息保存在私密 `.runtime/LOCAL_LOGIN.txt`，不是新安装的默认凭据。

## 配置与环境变量

| 变量 | 生效位置与含义 |
|---|---|
| `CYLEDGER_BIND` | Compose 宿主机监听地址，默认 `127.0.0.1` |
| `CYLEDGER_PORT` | Compose 宿主机端口，默认 `8080`；Windows 脚本用 `-Port` |
| `CYLEDGER_ROOT_URL` | Compose 对外完整地址，默认 `http://localhost:8080/` |
| `CYLEDGER_VOLUME` | Compose 持久卷名，默认 `cyledger_data`；仅在有意切换实例时修改 |
| `TZ` | 容器系统时区，默认 `Asia/Shanghai`；会计时区仍取用户设置 |
| `CYLEDGER_COINGECKO_DEMO_API_KEY` | 可选服务端行情密钥，重启后生效；不是交易账户密钥 |
| `ANDROID_HOME` | Android 构建使用的 SDK 根目录，可用 `--sdk` 覆盖 |

`.env.example` 是 Compose 配置模板，实际 `.env` 不提交。证券公开参考价、场外净值、货币基金万份收益和 ECB 汇率无需新增密钥；网络、供应商接口及额度仍是外部条件。报价规则与显式联网检查见 [行情模块](../pkg/marketquotes/README.md)。

## Docker Compose

在 Docker 可正常运行的机器上执行：

```bash
docker compose up -d --build
docker compose ps
docker compose logs --tail=100 -f app
curl --fail http://localhost:8080/healthz.json
```

容器以 UID/GID 1000 运行，持久卷包含配置、SQLite、附件和日志；更新镜像不替换该卷。首次创建账户：

```bash
docker compose exec app python3 /app/scripts/create_user.py --binary /app/cyledger --config /app/runtime/cyledger.ini --username cy --email your-address@example.com --nickname 我的账本
```

公网部署需 HTTPS 反向代理及正确的 `CYLEDGER_ROOT_URL`。当前只完成 Compose 配置检查，未完成当前版本镜像构建、容器运行和公网验收。健康检查只证明 Web 服务响应，不代表外部行情可用。

## 完整备份

**先停止应用及所有写入任务。** SQLite backup API 可保持数据库一致，但附件与数据库需要停服才能对应。`--stopped` 是操作者的确认，脚本不会自动停止服务。

```powershell
New-Item -ItemType Directory -Path .\backups -Force | Out-Null
$ledgerBackup = '.\backups\cyledger-' + (Get-Date -Format 'yyyyMMdd-HHmmss') + '.zip'
python .\scripts\backup.py create --runtime .\.runtime --output $ledgerBackup --stopped
if ($LASTEXITCODE -ne 0) { throw '备份失败。' }
python .\scripts\backup.py verify $ledgerBackup
if ($LASTEXITCODE -ne 0) { throw '备份校验失败。' }
```

备份包含 SQLite 所有表、账本/账户/流水/模板、投资事件及关联、报销/债务/分期/定存、自动收益防重记录、预算规则/总结/统计偏好、愿望/关键词/导入批次、费用/到期关联、优惠和标签父级字段、配置和附件，附版本信息与逐文件 SHA-256；不包含日志和程序。完成后重新启动服务。

电脑备份不会读取浏览器 localStorage，因此首页卡片、搜索历史和本机主题等浏览器偏好不在该 ZIP 中。手机原生备份会额外保存允许的显示偏好，两种格式不能混用；不能用 CSV/Excel 代替完整备份。

Docker 停服备份（Bash）：

```bash
mkdir -p backups
docker compose stop app
docker compose run --rm --no-deps --user 0:0 --entrypoint python3 -v "$PWD/backups:/backups" app /app/scripts/backup.py create --runtime /app/runtime --output /backups/cyledger-backup.zip --stopped
docker compose start app
```

辅助容器临时使用 root 读取卷，常驻服务仍使用 UID 1000。核对命令结果并确保服务恢复运行。备份包拒绝同名覆盖；ZIP 未加密且含密钥及财务数据，应保护访问并另存离机副本。SHA-256 检测损坏，不能证明发布者身份。

## 恢复到空白实例

只能恢复到不存在或完全为空的目录，不覆盖现有账本。恢复拒绝路径穿越、重复/绝对路径、符号链接和校验不一致，执行 SQLite 完整性及表记录数检查，并重写数据库/附件路径以隔离原实例。

```powershell
python .\scripts\backup.py restore .\backups\cyledger-backup.zip --target .\.runtime\recovered
Copy-Item -LiteralPath .\.runtime\cyledger.exe -Destination .\.runtime\recovered\cyledger.exe
powershell -ExecutionPolicy Bypass -File .\scripts\Start-CYLedger.ps1 -RuntimeDirectory .\.runtime\recovered -Port 18080
```

打开 [恢复实例](http://localhost:18080/)，用原账户核对余额、数量、成本、账本归属和各类关联。估值对比使用同一份保存的价格/汇率；实时行情刷新造成的估值变化不等于恢复错误。确认后才决定切换，保留原实例。

Docker 使用新卷恢复（Bash）：

```bash
docker volume create cyledger_restored
docker run --rm --user 0:0 --entrypoint python3 -v cyledger_restored:/app/runtime -v "$PWD/backups:/backups:ro" cyledger:local /app/scripts/backup.py restore /backups/cyledger-backup.zip --target /app/runtime
docker run --rm --user 0:0 --entrypoint sh -v cyledger_restored:/app/runtime cyledger:local -c 'chown -R 1000:1000 /app/runtime'
```

在 `.env` 将 `CYLEDGER_VOLUME` 改为新卷，再启动并核对。保留旧卷回退，禁止 `docker compose down -v` 删除数据卷。工具仅支持 SQLite + 本地附件，单包上限 16 GiB / 100,000 文件，不提供远程附件的一致性备份。

## 升级与验证

1. 确认配置、程序和数据库路径；停服，备份并校验，记录旧源码提交。
2. 先将备份恢复到独立目录和端口，用新程序验证。多账本迁移会建立默认“日常账本”并补齐历史归属，重复执行不重复迁移。模型同步补充统计三表、`LocalLedgerItem`、优惠/费用/借款日期/地点和标签父级字段，以及日历的交易关联；旧流水不自动生成费用、愿望或到期提醒，原有标签保持一级。钱包货币单位新增字段默认人民币，收付偏好不改旧交易；钱包收支仍保存在投资事件及其关联账单中，没有独立的美元现金账本。
3. 验证余额、成本、持仓及关联后，重建正式程序并启动；检查 `/healthz.json`、登录、账本筛选、资产详情和统计。恢复验收还需核对预算规则及结转口径、总结、模块顺序、优惠、标签层级、愿望和导入批次；有钱包收支时核对原币金额、历史汇率、实际币种数量与人民币关联账单，手动价格须保留原币。不要以重新计算出的行情估值代替事实核对。
4. 失败时用旧版本和升级前备份恢复到另一空目录；不要用旧程序直接打开已迁移的新库，不覆盖当前库。

Android 使用独立签名升级流程，详见 [Android](../android/README.md)。备份工具验证命令为 `python -m unittest discover -s scripts -p test_backup.py -v`；运行会生成隔离测试目录，用户要求清理时可删除。

## 常见问题

| 现象 | 核查 |
|---|---|
| 端口占用或界面仍旧 | 核对服务路径/版本和 `dist`，停止旧实例后重建；PWA 更新下载后刷新，不清空账本 |
| 基金收益等待 | 核对六位代码、开始日期、时区、公开万份收益及网络；缺日不跳过，不用净值替代万份收益 |
| 开启 VPN 后基金查询慢或失败 | 两个基金接口直接优先使用应用内 HTTPS 解析，并在有效期内复用解析和基金目录结果；只有此路径失败才回退系统解析。失败时可点“重新查询”。保留 CYLedger 的 VPN 分应用选择供加密行情使用，无须把整个应用改为直连 |
| 新增账户后反复保存失败 | 若提示账户资料已保存、后续设置未完成，继续在原账户重试即可；移动端允许资料不变时继续保存余额或基金绑定，不重复创建账户。错误提示保留接口返回的具体原因 |
| 分期/定存没有到期记录 | 核对日期/时区、账户与分类、是否结束；重新进入应用触发同步，不能把提醒当作真实还款 |
| 资产不完整 | 查看缺失价格、历史成本或汇率及其日期，不手工填零消除未知 |
| BTC 等持仓数量正确但人民币估值为“—” | 在持仓详情查看美元报价与折算汇率；预置币主源缺失/过期时会自动使用 CoinGecko 公开备用报价，无须 Demo 密钥。账户与持仓详情每 15 秒读取本地估值缓存；供应商都不可达时仍保持未知 |
| 钱包收支没有默认币种或数量不符 | 在账户编辑页设置“货币单位 / 收支默认币种”；每笔保存前核对实际数量。每枚约 1 美元仅是预填，历史日期或缺少参考汇率时须手动填写；修改已有收支从钱包账单进入，不单独改系统统计账单 |
| 普通账户无法修改货币单位 | 有余额、历史记录（含已删除）、模板、收益/定存绑定或共享额度等计价依赖时必须保留原币种；不能通过清空账本绕过保护 |
| 统计金额为“—”或占比为空 | 检查当前账本/日期/筛选及外币历史汇率；未知金额或净退款不绘制占比，资产走势没有有效快照时留空 |
| 预算与收支金额不同 | 预算按自身账本、日期和分类计算，不受临时账户/标签筛选影响；核对退款、排除统计的流水、沿用规则和结转开关 |
| 预算/总结/模块设置保存冲突 | HTTP 409 表示修订号已过期；保留未保存内容，重新读取后再编辑，不能用旧修订号强行覆盖 |
| Android 没有提醒 | 查看资产提醒开关和系统通知授权；应用停止、节电限制或系统调度可能影响送达 |
| 签名不兼容 | 核对原 `.runtime/android-signing`；不能用卸载正式版解决升级签名问题 |

日志不应记录完整账本、密钥或测试令牌。排错和破坏性验证使用独立库；正式数据与签名材料不能作为过程文件删除。

收尾清理可删除测试脚本、截图、独立验收数据、QA 安装包及 `.runtime/android/` 构建缓存；保留正式 `dist/`、`.runtime/cyledger.exe`、配置、登录资料、数据、附件、历史备份及 `.runtime/android-signing/`。缓存清理后下次 Android 构建不能使用 `--skip-backend`，应执行完整构建。
