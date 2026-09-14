# Checklist

## 环境与账号
- [x] Task 1 完成：各服务健康状态已核验并记录；testuser/testadmin 账号已创建且登录成功（admin/admin123 不存在，改用 testadmin/testuser 并记录）；前端 3000 已启动可访问；stocks 202 条与预测数据前置已就绪

## 模块执行与报告
- [x] AUTH-001~044 已逐条执行，结果记录至 `docs/test-reports/01-认证与账户管理.md`
- [x] STOCK-001~056 已逐条执行，结果记录至 `docs/test-reports/02-股票检索与自选股.md`
- [x] MKT-001~038 已逐条执行，结果记录至 `docs/test-reports/03-实时行情.md`
- [x] PRED-001~084 已逐条执行，结果记录至 `docs/test-reports/04-预测模块.md`
- [x] EVAL-001~044 已逐条执行，结果记录至 `docs/test-reports/05-评估模块.md`
- [x] TRADE-001~040、SIM-001~040 已逐条执行，结果记录至 `docs/test-reports/06-交易与模拟交易.md`
- [x] ADM-001~084 已逐条执行（训练触发类用例标记「跳过（用户要求，不做训练测试）」，只读用例正常执行），结果记录至 `docs/test-reports/07-管理后台与模型管理.md`
- [x] ML-001~060 已逐条执行（训练触发类用例标记「跳过（用户要求，不做训练测试）」，非训练类用例正常执行），结果记录至 `docs/test-reports/08-ML服务.md`
- [x] DASH-001~023 已逐条执行，结果记录至 `docs/test-reports/09-决策支持仪表盘.md`
- [x] WS-001~020 已逐条执行，结果记录至 `docs/test-reports/10-WebSocket.md`
- [x] 前端交互用例已通过浏览器自动化执行（28 项，报告含文字证据），结果记录至 `docs/test-reports/11-前端交互.md`
- [x] E2E-001~011 已逐条执行，结果记录至 `docs/test-reports/12-全流程贯通.md`
- [x] LINK-001~020 已逐条执行，结果记录至 `docs/test-reports/13-跨模块数据衔接.md`

## 结果质量
- [x] 每条用例均有结果（通过/失败/阻塞）与证据（实际响应/截图/日志），无遗漏
- [x] 阻塞用例均注明原因（外部依赖/需人工环境/训练类跳过/服务重启等）
- [x] 汇总总览 `docs/test-reports/00-测试执行总览.md` 已产出：通过/失败/阻塞统计、通过率、模块分布
- [x] 缺陷清单已产出：失败用例、实现口径与文档差异、环境阻塞项（P0×6、P1×9、P2×6、环境阻塞类）
- [x] 测试收尾：本测试启动的前端服务已关闭；预置服务未受影响