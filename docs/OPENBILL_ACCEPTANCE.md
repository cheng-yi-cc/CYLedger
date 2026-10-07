# OpenBill 验收与交接

状态日期：2026-10-07。本次交付为 OpenBill 改名、品牌图标、Android 原生开屏与仅保留 Android 的范围调整。账务算法和正式账本不在本次修改范围内。

## 已实现

- 应用显示名、导出文件名、通知与解锁提示、关于页仓库链接和文档改为 OpenBill。GitHub 仓库已改名为 `cheng-yi-cc/OpenBill`，本地 origin 已同步。
- 深墨绿、暖白与浅绿的“打开的账本 + O”图标，提供 Android 自适应图标、主题图标和 SVG。原生开屏展开账页并画出 O，最长 850 毫秒；动画与后端启动并行，页面就绪后淡出，没有强制停留计时。系统关闭动画时使用静态图形；页面失败可重试。
- 删除独立电脑界面、桌面路由/布局/样式、电脑启动/备份工具、独立桌面后端入口、Docker 账本部署、桌面发布流程及依赖独立服务器的上游 API 工具；移除未使用的桌面代码编辑器、路由与 PWA 构建依赖。保留 Android 所需的 Go / Vue 共用实现，以及可选的公开行情中转服务。
- 日历时钟在应用恢复前台和同步时重新取得系统时区；显式设置的会计时区始终优先。修复时钟测试缺少设置模块隔离的问题。
- 包名 `com.cyledger.android`、签名、数据库路径、原生桥与 `cyledger-mobile-backup-v1` 格式身份保持兼容；新备份文件以 OpenBill 命名，旧 CYLedger 文件名仍被接受。

## 已验证

| 项目 | 结果 |
|---|---|
| 前端 | Vue 类型检查、生产构建通过；手机首页布局既有 4 项测试、日历跨月/系统时区变化/显式时区优先 3 项测试通过；构建中没有 desktop.html 或 desktop 资源 |
| Go / SQLite | `go test ./cmd ./android/backend ./pkg/localbackup` 通过；cmd 为编译检查 |
| 远端前端检查 | 提交 `5a8e5a45` 的 Vue 类型检查、全量 Vitest、生产构建及仅 Android 资源检查通过；[CI 记录](https://github.com/cheng-yi-cc/OpenBill/actions/runs/37639788746) |
| 远端行情检查 | GitHub Actions 的行情竞态回归、Java 编译及 Android ARM64 / JNI 编译通过 |
| Android 构建 | ARM64 共享库、JNI、Java、资源、D8、APK v2/v3 签名及 16 KiB 对齐检查通过 |
| 真机启动 | 型号 23113RKC6C、Android 16；测试版冷启动动画进入首页，关闭系统动画后仍能进入，原系统设置已还原 |
| 本机路由 | 前台 QA 后端 `/healthz.json` 返回 200，已移除的 `/desktop` 返回 404；检查后移除临时端口转发 |
| 手机备份 | 独立空白 QA 账本创建 OpenBill 完整备份，将该测试副本改为旧 CYLedger 文件名后由系统文件选择器恢复，产生恢复前目录并重新进入首页；未使用正式备份 |
| 正式升级 | 已使用 `adb install --no-incremental -r` 覆盖升级并启动；版本 0.3.0 / versionCode 4，首次安装时间仍为 2026-10-01 11:59:53；没有卸载或清除正式数据 |

## 交付与预览

正式包：`dist/android/OpenBill-Android-arm64.apk`，63854887 字节，对应品牌改版提交 `3d161479`。日历时区补充修复已提交源码，按用户确认的并行任务安排，随收益修复任务统一构建并覆盖升级；此处旧包校验值不代表后续统一包。

SHA-256：`5d5565cedaa8da0f5cea65b1c08ed86f7382f5371dbc00aa5f238c27c934afb5`。

签名证书 SHA-256：`19d9a950a1d3608d8098f6ea8ebe47b464126d537946470eb7a369007ad1add3`。

手机打开 **OpenBill** 即可使用；关闭后重新打开查看开屏。开发预览使用 `python scripts/build_android.py --qa`，安装 `dist/android/OpenBill-Android-arm64-qa.apk` 后打开 **OpenBill 测试**。正式和 QA 私有账本隔离；签名材料必须长期保留。

源码已合并至 `main` 并同步到 `cheng-yi-cc/OpenBill`；开发分支已删除，独立工作树的 Git 登记和全部内容已清理；原工作树只剩被本次聊天进程占用的空目录，退出占用后可删除。正式 APK 保存在主目录 `D:/My Project/CY Finance Manager/dist/android/OpenBill-Android-arm64.apk`。本次构建缓存、截图、视频、测试备份副本及 QA 应用已清理，正式账本、原有备份和主目录 `.runtime/android-signing/` 签名材料保留。

## 未验证与外部依赖

最低声明 Android 8.0，本次只做 Android 16 真机验收；其他系统版本和厂商启动窗口尚未实测。未重新执行全部历史账务验收，也未验证真实网络切换矩阵。公开行情仍依赖网络与供应商，可选中转仍需自行配置；本次品牌和平台调整无需新增外部服务。
