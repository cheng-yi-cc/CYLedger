# OpenBill Android 运维

仅提供 Android 应用。工具和构建步骤见 [Android 说明](../android/README.md)，日常操作见 [使用入口](../README.OPENBILL.md)。

## 安装和本地预览

在项目根执行 `python scripts/build_android.py`，正式产物为 `dist/android/OpenBill-Android-arm64.apk`。使用 `adb install -r dist/android/OpenBill-Android-arm64.apk` 同签名覆盖升级，手机打开 **OpenBill**。测试用 `--qa` 构建独立 **OpenBill 测试**，正式数据与测试数据隔离。

从 CYLedger 升级时，保留包名 `com.cyledger.android`、签名、数据库路径 `files/ledger/data/cyledger.db`、原生桥和备份格式身份。这些兼容标识不展示为应用名称。`.runtime/android-signing/` 中的密钥和密码必须长期保存，不得提交仓库、重新生成或作为临时文件清理。

## 备份和恢复

在“我的 → 数据备份”创建完整备份，使用系统文件选择器导出。新文件名以 `OpenBill-` 开头，旧 `CYLedger-` 备份仍可列出、导出和恢复。格式继续使用 `cyledger-mobile-backup-v1`，包含数据库、附件、配置和允许的显示偏好，不包含会话令牌或设备解锁凭据。CSV/Excel 仅用于账单交换，不能代替完整备份。

恢复会先校验格式、路径、大小、摘要、数据库完整性和本机档案，再自动备份当前账本，通过独立维护进程停止后端并替换目录。失败时保留恢复前数据；完成后核对金额、附件和偏好。未核对前不要删除 `files/restore-previous-*`。ZIP 未加密，须妥善保管。

## 故障排查

| 问题 | 处理 |
|---|---|
| 覆盖升级签名不匹配 | 找回原 `.runtime/android-signing/`，不要卸载或清除数据 |
| 停在开屏或提示启动失败 | 重试或关闭后重新打开，保留应用数据；QA 可查看日志和 WebView |
| 动画关闭 | OpenBill 遵循系统动画开关，静态开屏仍可正常进入 |
| 行情失败 | 核对网络、供应商与可选中转；缺失值保持未知，不影响离线记账 |
| 恢复失败 | 保留原文件及账本，查看校验错误，不能通过卸载排错 |

可选行情中转是公开报价辅助服务，配置见 [行情网络](OPENBILL_MARKET_NETWORK.md)，不承载用户账本。应用不提供电脑界面、电脑备份或 Docker 账本部署。
