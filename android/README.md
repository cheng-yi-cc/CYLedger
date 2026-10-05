# CYLedger Android 独立运行版

APK 内包含 Vue / Framework7 界面、ARM64 Go / SQLite 共享库和 Java / JNI 外壳。正式包为 `com.cyledger.android`，只访问手机回环服务 `127.0.0.1:18761`，不需要电脑常驻。界面操作见 [使用入口](../README.CYLEDGER.md)，本次 APK 校验值与实测范围见 [验收与交接](../docs/CYLEDGER_ACCEPTANCE.md)。

## 使用与数据边界

打开 **CYLedger** 直接进入个人账本。首次使用自动建立简体中文/人民币档案；升级沿用已有唯一档案及 UID，保留账本。多个、停用或删除档案会阻止自动进入，避免误选。注册接口关闭；会话由 Go 经 JNI 仅交给本应用 WebView，没有公开自动登录 HTTP 接口。

数据库为应用私有目录 `files/ledger/data/cyledger.db`，附件为 `files/ledger/storage`。离线记账直接写入手机数据库。电脑与手机账本独立，尚无自动同步或完整手机备份入口；CSV 不能替代数据库、附件和配置的完整备份。

正式版关闭 WebView 调试和系统云备份，使用手机系统访问保护。返回手势、系统栏/键盘、文件选择与 CSV 保存由原生外壳接入。深色、浅色及跟随系统主题均可用。

资产提醒需 Android 通知授权。系统可能延后通知；不保证应用关闭期间持续结算，重新打开时补齐到期费用、定存收益和可取得数据的基金收益。公开行情、汇率等功能仍需互联网。

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
