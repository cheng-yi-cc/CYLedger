# CYLedger 初版运行、备份与恢复

CYLedger 延续 ezBookkeeping 的 Go + Vue 应用，日常记账与新增投资数据使用同一个 SQLite 数据库。`LICENSE` 为 Apache 2.0；`NOTICE` 和 `licenses/ezbookkeeping-MIT-LICENSE` 保留上游归属与 MIT 声明，第三方依赖声明随镜像交付。这里的备份包含全部用户与财务信息，只应由实例管理员操作。

## Windows 本机运行

本次本机交付的服务已在 **http://localhost:8080/** 运行，可直接打开；如果已有本应用占用 8080，不要重复执行启动命令。用户名为 `cyledger`，密码仅位于私密文件 `.runtime/LOCAL_LOGIN.txt`。这不是通用安装默认密码，下文的创建账户步骤用于其他机器或尚未初始化的实例。

从项目根目录打开 PowerShell。需要 Go（项目 `go.mod` 指定版本）、GCC、Node.js、npm、Python 3.11 或更新版本。当前机器的 Go 与 GCC 分别安装在 `D:\tools\cyledger\go\bin` 和 `D:\tools\cyledger\mingw64\bin`，启动脚本会自动加入本进程 PATH，不改系统设置。

首次构建和启动（重建本次实例前应先停止现有服务）：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\Start-CYLedger.ps1 -Build -DirectNpm
```

`-DirectNpm` 使用运行目录里的独立 npm 配置，绕过本机已有但失效的 npm 用户代理；不修改全局 `.npmrc`。有可用企业代理时可省略该参数。构建包含 Go 二进制和完整 Vue 前端，第一次下载依赖需要网络。

浏览器访问 **http://localhost:8080/**。脚本在当前终端运行服务，按 `Ctrl+C` 停止。服务已经停止时，后续启动：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\Start-CYLedger.ps1
```

仅写入配置、不启动服务：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\Start-CYLedger.ps1 -ConfigureOnly
```

默认仅监听本机 `127.0.0.1`。需要同一局域网手机验收时，运行 `-BindAddress 0.0.0.0`，在手机打开电脑的局域网地址及 `8080` 端口；这不是正式公网部署。`-Port 18080` 可指定其他端口。浏览器首次登录后将界面语言设为简体中文，在投资设置中确认本位币 CNY 和自己的会计时区，时区不由服务端 `TZ` 代替。

运行文件：

| 路径 | 用途 |
|---|---|
| `.runtime/cyledger.exe` | 本机服务程序 |
| `.runtime/cyledger.ini` | 私密配置与自动生成的签名密钥 |
| `.runtime/data/cyledger.db` | 完整 SQLite 数据库，包括新增投资表与关联 |
| `.runtime/storage/` | 附件、头像、自定义图标 |
| `.runtime/log/` | 运行日志 |
| `dist/` | 前端构建文件，可重新生成 |

每次配置会更新运行路径、监听地址、端口，并保持公开注册关闭；已生成的签名密钥保留。可选 CoinGecko Demo 行情密钥通过本进程环境变量 `CYLEDGER_COINGECKO_DEMO_API_KEY` 设置，服务重启后生效。不要填入交易所交易密钥、私钥或助记词。

### 停止本次后台预览

本次交付的 8080 预览在后台运行，其 PID 记录在 `.runtime/server.pid`。需要重建或备份时，在项目根目录运行下面的命令；先验证该进程的程序路径确实是当前项目的 `.runtime/cyledger.exe`，再停止。PID 文件缺失、内容无效或路径不符时应先核对实例，不要直接按旧 PID 终止程序。之后用 `Start-CYLedger.ps1` 启动的前台服务按 `Ctrl+C` 停止即可。

```powershell
$ledgerProcessId = 0
$ledgerPidText = (Get-Content -LiteralPath .\.runtime\server.pid -Raw -ErrorAction Stop).Trim()
if (-not [int]::TryParse($ledgerPidText, [ref]$ledgerProcessId) -or $ledgerProcessId -le 0) {
    throw '运行记录中的 PID 无效，请先核对服务进程。'
}
$ledgerExecutable = (Resolve-Path -LiteralPath .\.runtime\cyledger.exe -ErrorAction Stop).Path
$ledgerProcess = Get-CimInstance Win32_Process -Filter "ProcessId = $ledgerProcessId" -ErrorAction Stop
if ($null -eq $ledgerProcess) {
    Write-Host '记录的进程已停止。'
} elseif (-not [string]::Equals($ledgerProcess.ExecutablePath, $ledgerExecutable, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw '记录的 PID 与当前项目程序路径不符，未停止任何进程。'
} else {
    Stop-Process -Id $ledgerProcessId -ErrorAction Stop
    Write-Host 'CYLedger 后台预览已停止。'
}
```

## 创建自己的登录账户

公开注册默认关闭，不提供共享管理员密码。服务首次启动完成数据库结构初始化后，在项目根目录另开终端运行：

```powershell
python .\scripts\create_user.py --binary .\.runtime\cyledger.exe --config .\.runtime\cyledger.ini --username cy --email your-address@example.com --nickname 我的账本
```

按提示输入两次密码。脚本使用现有 CLI 创建用户，将默认记账货币设为人民币；不把密码写进 shell 历史，并对 CLI 失败输出进行密码脱敏。上游 CLI 接口仍需把密码作为短暂进程参数传递，因此应在你控制的本机上执行。随后返回网页登录；公开注册继续关闭。

## Docker Compose

在已能正常运行 Docker 的机器上，项目根目录执行：

```bash
docker compose up -d --build
```

然后访问 **http://localhost:8080/**。默认使用一个应用容器和名为 `cyledger_data` 的持久化卷，卷中包含配置、SQLite、附件和日志。首次启动自动生成签名密钥。更新镜像不会替换数据卷。

需要修改地址、端口或行情密钥时，将 `.env.example` 复制为 `.env` 并填写实际值。示例内没有可用密钥。正式公网环境应由 HTTPS 反向代理接入，`CYLEDGER_ROOT_URL` 填写实际 HTTPS 地址；不要直接把开发 HTTP 端口暴露到公网。

创建账户：

```bash
docker compose exec app python3 /app/scripts/create_user.py --binary /app/cyledger --config /app/runtime/cyledger.ini --username cy --email your-address@example.com --nickname 我的账本
```

状态、日志和健康检查：

```bash
docker compose ps
docker compose logs --tail=100 -f app
curl --fail http://localhost:8080/healthz.json
```

`/healthz.json` 检查 Web 服务能否响应；不意味着外部行情供应商可用。行情是否实时、过期或缺失，应查看资产页的报价状态。容器以 UID/GID 1000 运行，不赋予额外 Linux capabilities。

当前本机 Docker daemon 未能启动，因此本轮应以原生 Windows 运行验收为准。Dockerfile 和 Compose 是部署交付，不能把未实际运行的容器构建宣称为已通过。

## 完整备份

业务 CSV 适合阅读和迁移，完整恢复必须使用这里的备份。备份包包含：SQLite 所有表、账户余额、投资事件、数量与成本、系统结算角色、旧流水关联、持仓快照、用户配置、附件、版本元数据以及逐文件 SHA256。日志和程序二进制不属于业务恢复数据。

**先停止应用再备份。** SQLite backup API 本身能获得一致的数据库快照，但附件与数据库是两个存储层，停服才能保证它们属于同一状态。`--stopped` 表示你已经停止所有应用进程与定时写入任务，工具不会擅自终止程序。

Windows：在运行服务的终端按 `Ctrl+C`；本次后台预览按上面的进程校验步骤停止，然后：

```powershell
New-Item -ItemType Directory -Path .\backups -Force
$backupFile = '.\backups\cyledger-' + (Get-Date -Format 'yyyyMMdd-HHmmss') + '.zip'
python .\scripts\backup.py create --runtime .\.runtime --output $backupFile --stopped
python .\scripts\backup.py verify $backupFile
```

完成后重新运行 `Start-CYLedger.ps1`。输出应为 `status: ok`。工具拒绝覆盖同名备份；失败产生的包不会通过 `verify`，不要用它恢复。

Docker（以下为 Bash 命令，备份路径可替换）：

```bash
mkdir -p backups
docker compose stop app
docker compose run --rm --no-deps --user 0:0 --entrypoint python3 -v "$PWD/backups:/backups" app /app/scripts/backup.py create --runtime /app/runtime --output /backups/cyledger-backup.zip --stopped
docker compose start app
```

备份辅助容器临时使用 root 以读取私密卷和写入宿主机备份目录，常驻应用仍是非 root。Linux 上生成的包权限为仅创建者可读写，管理员可将包所有者移交给自己的账户后离机保存。任何备份失败时应查看错误，确认服务已经重新启动。

备份包含账户信息、财务数据和签名密钥，**ZIP 没有加密**；应放在受保护目录，并保存一份离机副本。SHA256 用于检测损坏和未同步修改，不能证明文件来自可信发布者；仅恢复自己保管的备份。自托管不等于端到端加密。

## 恢复到空白实例

工具只允许恢复到不存在或完全为空的目录，不覆盖当前账本。恢复前会拒绝路径穿越、绝对路径、重复路径、ZIP 符号链接及校验不一致的条目。恢复后进行 SQLite 完整性检查和表记录数比对，并将数据库/附件路径指向新目录，避免恢复实例意外打开原数据库。

Windows 示例：

```powershell
python .\scripts\backup.py restore .\backups\cyledger-backup.zip --target .\.runtime\recovered
Copy-Item -LiteralPath .\.runtime\cyledger.exe -Destination .\.runtime\recovered\cyledger.exe
powershell -ExecutionPolicy Bypass -File .\scripts\Start-CYLedger.ps1 -RuntimeDirectory .\.runtime\recovered -Port 18080
```

在 **http://localhost:18080/** 用原账户登录，比较恢复前后日常账户余额、投资数量、剩余成本、旧流水关联和净资产。净资产应使用同一份保存的价格/汇率对比；真实行情继续更新后市场估值可能自然改变。确认恢复验收完成后再决定切换使用实例，原数据保持不动。

Docker 恢复到另一个新卷：

```bash
docker volume create cyledger_restored
docker run --rm --user 0:0 --entrypoint python3 -v cyledger_restored:/app/runtime -v "$PWD/backups:/backups:ro" cyledger:local /app/scripts/backup.py restore /backups/cyledger-backup.zip --target /app/runtime
docker run --rm --user 0:0 --entrypoint sh -v cyledger_restored:/app/runtime cyledger:local -c 'chown -R 1000:1000 /app/runtime'
```

卷初始化后必须把数据目录所有者交回应用的 UID 1000。在 `.env` 将 `CYLEDGER_VOLUME` 改为 `cyledger_restored`，再执行 `docker compose up -d`。切换前备份当前实例，停用旧容器；旧卷保留用于回退。不要执行 `docker compose down -v`，该参数会删除持久化卷。

## 升级与验证

升级前停服并完成完整备份，记录当前源码提交；再更新本项目版本，使用 `Start-CYLedger.ps1 -Build` 或 `docker compose up -d --build` 构建。数据库迁移在启动时执行。升级失败时使用旧版程序和升级前备份恢复到新的空目录/新卷，不直接用旧程序打开已经迁移的新数据库。

本地可运行的备份恢复检查：

```powershell
python -m unittest discover -s scripts -p test_backup.py -v
```

测试实际创建 SQLite 账本、二进制附件和签名配置，恢复到空目录后核对余额、18 位数量、36 位计算成本文本、净资产快照、关联与附件，并检查篡改、恶意路径和覆盖已有目录的拒绝行为。测试证据保留在 `.runtime/backup-tests-*`。这属于备份工具验收；产品接口、用户隔离和手机页面仍需按开发文档分别验收。

限制：完整备份工具当前仅支持 SQLite + 本地附件存储，单包上限 16 GiB、100,000 个文件；不是异地附件存储或在线多服务一致性备份。导入历史账单时仍需自行选择“历史流水重建余额”或“当前余额起步”，避免将当前余额与已包含的历史流水重复累计。
