# CYLedger

简体中文、人民币本位的个人记账与资产管理工具。基于 ezBookkeeping v2.0.0，保留 Vue、Go 和 SQLite，在日常收支之外提供多账本、预算与统计、投资持仓、基金确认与定投、加密每日定投、报销、债务、信用卡、定存与货币基金收益。

支持电脑网页和独立 Android 应用。Android 在手机内运行 Go / SQLite，断网也能记账；手机与电脑使用各自的账本，尚无自动同步。

## 开始使用

手机安装包：`dist/android/CYLedger-Android-arm64.apk`。**首页**提供账单搜索、手动记账和数据卡片；账户与持仓在 **资产**，预算和总结在 **统计**，愿望、导入导出、完整备份和个性设置在 **我的**。已有正式版须使用同签名覆盖升级，不能卸载或清除数据。构建和安装见 [Android 说明](android/README.md)。

Windows 在项目目录执行：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\Start-CYLedger.ps1 -Build -DirectNpm
```

打开 [电脑界面](http://localhost:8080/) 或 [手机界面预览](http://localhost:8080/mobile)。已构建后省略 `-Build -DirectNpm`。脚本在前台运行，按 `Ctrl+C` 停止。首次安装需按 [运维说明](docs/CYLEDGER_OPERATIONS.md#创建自己的登录账户) 创建账户；升级已有账本先停服并备份。

## 文档

| 文档 | 内容 |
|---|---|
| [使用与开发入口](README.CYLEDGER.md) | 记账、资产操作、本机工具与验证命令 |
| [产品范围](docs/CYLEDGER_BRIEF.md) | 产品目标、账务口径与未实现范围 |
| [架构](docs/CYLEDGER_ARCHITECTURE.md) | 数据模型、原子提交、估值、路由与 Android 边界 |
| [接口指南](docs/CYLEDGER_API.md) | API 路径、鉴权、输入、幂等和错误处理 |
| [运维](docs/CYLEDGER_OPERATIONS.md) | 环境变量、部署、备份、恢复和故障排查 |
| [验收与交接](docs/CYLEDGER_ACCEPTANCE.md) | 已验证范围、正式 APK 校验值及限制 |

公开行情依赖网络及供应商；缺失成本、价格或历史汇率保留未知。应用不连接银行或交易所账户，不执行扣款、下单或转币。

## 许可与归属

CYLedger 新增与修改部分采用 [Apache License 2.0](LICENSE)。上游 [ezBookkeeping](https://github.com/mayswind/ezbookkeeping) 固定为 v2.0.0，基线提交 `b2a3f4ede42c8a7aa3bab1e6dbbc7b8e6196ef35`；上游代码保留其 [MIT 许可](licenses/ezbookkeeping-MIT-LICENSE)。其他依赖及图标声明见 [NOTICE](NOTICE)、[第三方清单](third-party-dependencies.json) 和 `licenses/`。Go 模块路径保持上游名称。
