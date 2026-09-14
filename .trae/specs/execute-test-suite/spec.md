# 全量测试执行 Spec

## Why
已产出覆盖 11 个模块、564 条测试用例文档（`docs/test-cases/`）。现需基于这些用例对系统进行全量执行验证（P0+P1+P2 全优先级 + 前端交互 + WebSocket），逐条记录结果与证据，暴露实现缺陷与文档口径偏差，产出测试执行报告。

## What Changes
- 测试环境准备与核验：后端 8080、ML 8000、Redis 6379、PostgreSQL 5432（含 calliper_trading 与 calliper_tsdb 双库）均已运行；前端 3000 需启动
- 准备测试账号：当前 `admin/admin123` 登录 401（账号不存在），需创建 admin 与 testuser 账号
- 按模块执行 API 测试用例（HTTP 调用），逐条记录 通过/失败/阻塞 + 证据
- 执行 WebSocket 用例（WS-001~020）
- 启动前端（`cd frontend; npm.cmd run dev`，端口 3000）并执行前端交互用例（浏览器自动化）
- 生成测试执行报告 `docs/test-reports/`（各模块报告 + 汇总总览 + 缺陷清单）
- **BREAKING**: 无代码改动；测试会在数据库产生测试数据（账号/订单/自选/预测），保留供用户核验；结束后仅关闭本测试启动的服务（前端），预先已运行的服务（后端/ML/Redis/PG）保持不动

## Impact
- Affected specs: complete-test-case-suite（用例文档为执行基准）
- Affected code: 无业务代码改动；新增 `docs/test-reports/` 报告
- 数据：新增测试账号、订单、预测等测试数据；不破坏现有数据
- 外部依赖：行情/指数类用例依赖 Sina/Yahoo/AKShare 网络可达；预测类依赖真实数据与已训练模型

## 环境现状（2026-09-03 核验）
| 组件 | 端口 | 状态 |
|---|---|---|
| 后端 Gateway | 8080 | 运行中（/health 返回 healthy） |
| ML 服务 | 8000 | 运行中 |
| Redis | 6379 | 运行中 |
| PostgreSQL（主库+TSDB 同实例） | 5432 | 运行中（TSDB_PORT=5432，见 backend/.env） |
| 前端 | 3000 | 未启动（vite server.port=3000，代理 /api 与 /ws 到 8080） |
| Kafka | — | KAFKA_BROKERS 为空，producer no-op |
| 测试账号 admin/admin123 | — | **不存在（登录 401），需创建** |

---

## ADDED Requirements

### Requirement: 测试环境与账号准备
系统 SHALL 在测试前核验环境组件、创建缺失的测试账号，并记录偏差。

#### Scenario: 健康检查
- **WHEN** 调用 `GET /health` 及各服务连通性检查
- **THEN** 记录各组件可用状态；不可用组件影响到的用例标记为「阻塞」

#### Scenario: 测试账号创建
- **WHEN** `admin/admin123` 登录返回 401
- **THEN** 通过 bcrypt 哈希直插 users 表创建 admin（role=admin）与 testuser（role=user），随后登录成功并取得 JWT

#### Scenario: 前端启动
- **WHEN** 执行 `cd frontend; npm.cmd run dev`
- **THEN** 前端在 http://localhost:3000 可访问，登录页可渲染

### Requirement: API 测试用例执行与记录
系统 SHALL 按模块执行 AUTH/STOCK/MKT/PRED/EVAL/TRADE/SIM/ADM/ML/DASH 全部 API 用例（P0+P1+P2），每条记录结果与证据。

#### Scenario: 逐条执行与断言
- **WHEN** 对某用例执行其「测试步骤」中的具体请求
- **THEN** 对照「预期结果」断言状态码与响应字段，记录为 通过/失败/阻塞（阻塞=前置条件无法满足，如外部数据源不可达、模型未训练），并附实际响应/截图/日志证据

#### Scenario: 异常用例可执行性
- **WHEN** 用例需制造异常场景（停库/停ML/停外网）
- **THEN** 优先通过参数注入或构造数据实现；无法安全制造的场景标记「阻塞（需人工环境）」，不强行破坏运行中的服务

### Requirement: WebSocket 用例执行
系统 SHALL 执行 WS-001~020（连接鉴权/订阅/推送/取消/重连/心跳/暂停恢复/服务重启）。

#### Scenario: WS 生命周期
- **WHEN** 使用 WebSocket 客户端携带 token 连接 `/ws` 并订阅 `stock:{symbol}`
- **THEN** 验证连接、订阅、quote 推送、心跳、断线重连、暂停恢复；服务重启场景标记「阻塞（需重启后端，单独安排）」

### Requirement: 前端交互用例执行
系统 SHALL 通过浏览器自动化执行交互用例（登录/注册/跳转/三态/表单校验/表格/筛选/独立 loading 等）。

#### Scenario: 浏览器自动化执行
- **WHEN** 浏览器自动化按用例步骤操作页面
- **THEN** 断言页面可见结果（提示文案/跳转/数据渲染/loading 态），附截图证据

### Requirement: 长耗时 ML 用例执行
系统 SHALL 跳过模型训练类用例（用户要求：训练耗时过长不执行），训练触发类用例（ML 训练端点、admin 训练触发）统一标记「跳过（用户要求，不做训练测试）」，不触发任何真实训练；其余非训练类 ML 用例（预测/特征/状态/健康/参数/回滚/评估读接口）正常执行。

#### Scenario: 训练类用例跳过
- **WHEN** 用例需要触发真实模型训练（`POST /api/v1/models/train/{period}`、`/admin/training/run` 等）
- **THEN** 不执行、标记「跳过」，报告注明原因，避免 CPU 长时间训练阻塞

#### Scenario: 非训练类用例正常执行
- **WHEN** 用例不触发训练（预测查询、模型状态、健康检查、参数读写、特征计算、评估读接口等）
- **THEN** 正常执行并记录 通过/失败/阻塞；依赖已训练模型的用例在模型缺失时按实际响应记录

### Requirement: 测试执行报告
系统 SHALL 生成测试执行报告（`docs/test-reports/`），含各模块逐条结果、汇总统计、缺陷清单。

#### Scenario: 报告产出
- **WHEN** 全部用例执行完毕
- **THEN** 产出：各模块报告（用例编号/结果/证据/备注）+ 汇总总览（通过/失败/阻塞统计、通过率、模块分布）+ 缺陷清单（失败用例、实现口径与文档差异、环境阻塞项）

---

## MODIFIED Requirements
无（本任务为测试执行，不改动既有功能需求）

## REMOVED Requirements
无
