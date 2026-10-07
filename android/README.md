# CYLedger Android 独立运行版

APK 内包含 Vue / Framework7 界面、ARM64 Go / SQLite 共享库和 Java / JNI 外壳。正式包为 `com.cyledger.android`，只访问手机回环服务 `127.0.0.1:18761`，不需要电脑常驻。界面操作见 [使用入口](../README.CYLEDGER.md)，本次 APK 校验值与实测范围见 [验收与交接](../docs/CYLEDGER_ACCEPTANCE.md)。

## 使用与数据边界

打开 **CYLedger** 直接进入个人账本。首次使用自动建立简体中文/人民币档案；升级沿用已有唯一档案及 UID，保留账本。多个、停用或删除档案会阻止自动进入，避免误选。注册接口关闭；会话由 Go 经 JNI 仅交给本应用 WebView，没有公开自动登录 HTTP 接口。

数据库为应用私有目录 `files/ledger/data/cyledger.db`，附件为 `files/ledger/storage`。离线记账直接写入手机数据库。电脑与手机账本独立，没有自动同步。“我的 → 数据备份”可创建、导出、删除和恢复完整手机备份；CSV/Excel 不能替代完整备份。

正式版关闭 WebView 调试和系统云备份，提供指纹/面容/系统锁屏凭据及本机手势锁。返回手势、系统栏/键盘、文件选择、CSV/Excel/图片 ZIP 和消费分析文本保存由原生外壳接入。导出时在系统文件选择器指定保存位置；导出文件不能替代完整账本备份。深色、浅色及跟随系统主题均可用。

资产提醒需 Android 通知授权。系统可能延后通知；不保证应用关闭期间持续结算，重新打开时补齐到期费用、定存收益、可取得数据的货币基金收益及基金定投。货币基金收益等待日期在前台每分钟重试，补记成功后刷新账单和理财页面；基金定投也在前台按分钟分批检查，缺净值或余额不足保持待确认；它不向交易平台实际下单或扣款。公开行情、汇率等功能仍需互联网。

“资产 → 加密交易所 → 每日定投”按公开历史参考价记录已设置的每日计划，不向交易所下单。关闭后重新打开会补记可取得行情的日期；缺数据待处理，余额不足直接暂停，手动恢复后不补买暂停期间。未来三天稳定币不足时在应用内弹窗，不要求额外通知权限。计划、执行记录及投资事实随 SQLite 完整备份保存。

## 手机完整备份与隐私

在“我的 → 数据备份”创建备份；可开启每天首次打开时自动保存一份，默认关闭。备份保存在应用私有的 `files/backups`，通过系统文件选择器导出到用户选定位置。备份包含 SQLite 全库、配置、附件、主题/卡片/搜索等显示偏好，附逐文件 SHA-256；不包含登录令牌、手机锁屏/手势凭据以及每日提醒、自动备份等原生开关。

备份先阻止请求与定时任务修改数据，再用 SQLite `VACUUM INTO` 获取一致快照。恢复先校验格式、路径、大小、摘要、SQLite 完整性及唯一有效本机档案，用户确认后自动备份当前账本；独立维护进程关闭主进程后替换目录并重启。恢复前目录和备份保留供回退，不静默删除。手机格式为 `cyledger-mobile-backup-v1`，单包最多 2 GiB / 50,000 文件，与电脑停服备份格式不同。ZIP 未加密，应保管好导出的副本。

应用锁在启动或进入后台超过 30 秒后启用；手势连续五次失败冷却一分钟，可使用系统凭据解锁。指纹/面容和凭据依赖手机系统已配置的锁屏。系统任务列表的内容隐藏可单独开启；每日提醒需通知权限，系统节电可能推迟提醒。系统快捷开关可直接打开手动记账。

微信/支付宝截图识别使用随 APK 内置的 Tesseract 中英文模型，在设备上运行。导入原图删除默认关闭，开启后仍由 Android 系统确认；不支持删除的文件来源提示手动处理。账单图片批量 ZIP 导出每批最多 18 MB，完整备份导出走独立文件流，不受此限制。

恢复失败时先保留原文件和应用数据：格式/校验失败不会替换账本；目录替换中断会在启动时依据恢复日志回退。不要卸载来排错。成功恢复后核对金额、附件、愿望、导入批次、理财展示规则、定投计划/确认状态及界面偏好，并确认是否需要重新设置原生提醒或自动备份。`files/restore-previous-*` 为恢复前保留目录，未核对恢复结果时不要删除。

## 构建

本机工具位置：

| 工具 | 位置/要求 |
|---|---|
| Android SDK | `C:\Users\45057\AppData\Local\Android\Sdk`，API 36、Build Tools 35.0.0 |
| NDK | `D:\tools\cyledger\android-sdk\ndk\28.2.13676358` |
| Go | `D:\tools\cyledger\go\bin\go.exe`，或 PATH 中的 Go |
| Java / Node / Python | PATH 中需要 JDK 的 `javac`/`keytool`、npm 和 Python |

在项目根执行：

```powershell
python .\scripts\build_android.py
```

脚本构建前端、Go 共享库、JNI、Java、D8 和签名 APK，检查 APK/ELF 的 16 KB 对齐。不需要另装 Android Studio、Gradle 或模拟器。脚本面向 Windows；SDK 可由 `ANDROID_HOME` 或 `--sdk` 指定，NDK 用 `--ndk` 指定。

产物为 `dist/android/CYLedger-Android-arm64.apk`。当前仅构建 ARM64，声明最低 Android 8.0，实际验收仅覆盖 Android 16 真机。前端和后端源码均未变化且缓存仍存在时，可使用 `--skip-frontend --skip-backend`；清理构建缓存后应执行完整构建。

APK 只打包网页资源、原生库、默认配置和许可证，排除运行账本、个人配置、签名材料及 `dist/android` 中的安装包。

## 覆盖升级

```powershell
adb devices -l
adb -s <手机序列号> install --no-incremental -r .\dist\android\CYLedger-Android-arm64.apk
adb -s <手机序列号> shell am start -n com.cyledger.android/com.cyledger.android.MainActivity
```

系统出现 USB 安装提示时需在手机上允许。**必须保留同一签名并覆盖安装，不能先卸载正式版或清除其数据。**

私密签名密钥与密码在 `.runtime/android-signing/`，须长期保留并另行妥善备份，不能作为构建临时文件清理或提交仓库。丢失或更换密钥将无法直接覆盖升级已安装应用。

## 隔离验收

```powershell
python .\scripts\build_android.py --qa
adb -s <手机序列号> install --no-incremental -r .\dist\android\CYLedger-Android-arm64-qa.apk
```

QA 包 `com.cyledger.android.qa` 使用独立私有目录与端口 `18762`，允许 WebView 调试，只能放虚构数据。需要电脑调用测试 API 时使用 `adb forward tcp:18862 tcp:18762`；结束后执行 `adb forward --remove tcp:18862`。不得把测试脚本指向正式端口。

用户要求清理后可卸载 QA、删除测试 APK 和过程文件，保留正式包与签名材料。每次交付先完成相称的账务检查、QA 真机验收，再同签名覆盖正式版；正式账本仅做必要的只读核对。
