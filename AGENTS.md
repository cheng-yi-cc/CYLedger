# 项目约定

## 技术与边界

- 这是 ezBookkeeping v2.0.0 的二次开发，保留 Go 模块路径、Vue、Vuetify、Framework7、Pinia 和 SQLite；不要另建无关前端或重写日常账务。
- 用户界面与新增项目文档默认简体中文，本位币固定为人民币；会计时区取用户设置。
- 投资事实在 `pkg/services/investments.go` 原子提交，成本回放在 `pkg/investments`，公开行情在 `pkg/marketquotes`，统一估值在 `pkg/services/wealth.go`。
- 新接口所有数量、金额、价格和汇率均为十进制字符串。输入先限制格式和长度，再交给十进制库；禁止浮点参与成本运算。
- 未知成本、缺失报价或历史汇率保持未知，不能当成零。行情不能生成收入或修改持仓数量。
- 系统结算账户必须在旧接口、批量操作、账户合并、普通列表和资产汇总中受保护。
- 不记录密钥或完整财务数据，不把 `.runtime`、`runtime`、备份和测试令牌提交到仓库。

## 本地运行与验证

- 本地启动：`powershell -ExecutionPolicy Bypass -File .\scripts\Start-CYLedger.ps1`；打开 `http://localhost:8080/`。
- 重建加 `-Build -DirectNpm`。本机工具路径和首次创建账户步骤见 `README.CYLEDGER.md`。
- 账务改动运行相关 Go 核心与 SQLite 集成测试；界面改动做类型检查及适当浏览器验收；备份改动运行 `scripts/test_backup.py`。
- 不为可逆、影响小、只是复述实现的改动添加测试。必要检查通过后不无故扩大测试范围。
- 修改历史事实必须使快照失效，并只用快照保存的历史价格重建。金额、数量、成本和关联在备份恢复后必须一致。
- 默认在原文件直接修改。用户明确结束任务或要求清理时，才清理过程文件与预览；不能删除正式账本。
- 每次交付项目修改都告诉用户本地预览方法；最终报告区分已实现、已验证、未实现和依赖外部配置的部分。

## 上游归属

`LICENSE` 为 Apache 2.0，适用于 CYLedger 新增与修改部分。必须保留 `NOTICE`、`licenses/ezbookkeeping-MIT-LICENSE` 中的上游版权和 MIT 许可，以及其他第三方声明。新增依赖须更新归属；不要直接复制强互惠许可项目的实现后改标 Apache 2.0。
