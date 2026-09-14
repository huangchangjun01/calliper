# 模型训练模块 Spec

## Why
管理后台的"模型管理"目前是空壳（`GET /admin/models` 返回 `not_implemented` 与空数组），用户看不到三个模型（short/medium/long）的真实状态；训练历史、定时调度、版本演进均无可视化，无法支撑 ML 训练流程的管理与可信度建设。

## What Changes
- **后端接通 ML 真实数据**：`GET /api/v1/admin/models` 改为代理 ML 服务 `GET /api/v1/models/status`，返回三个模型的真实 版本/准确率/训练时间/健康状态/参数
- **训练历史记录**：新增 `model_training_logs` 表（GORM AutoMigrate）与 `GET /api/v1/admin/training/history` 分页接口，记录每次训练的 周期/版本/准确率/样本量/触发方式/起止时间/状态
- **训练任务控制**：新增 `POST /api/v1/admin/training/run`（单周期或全量，代理 ML `/models/train/{period}`，同步等待并写历史日志）；管理后台"训练"按钮不再调未实现的 `POST /admin/models/{id}/train`
- **训练调度可视化**：ML 服务新增 `GET /api/v1/models/schedule` 返回 APScheduler 任务列表（id/名称/下次执行时间/触发规则）；后端 `GET /api/v1/admin/training/schedule` 代理之，前端展示 8:30 盘前预测 / 15:30 收盘预测 / 16:00 评估 / 17:00 轻量训练 / 周六 02:00 全量训练
- **版本对比/回滚**：ML `model_manager._save_model` 保存新权重前将当前文件备份为带版本号快照（如 `short_term_model_v2.1.0.pt`）；后端 `POST /api/v1/admin/training/rollback`（代理 ML 新增的 `POST /api/v1/models/rollback`）支持回滚到指定历史版本并热重载
- **前端**：管理后台新增"训练中心"区块（原模型管理扩展），包含 调度任务列表、训练历史表格（含触发方式/版本/准确率/状态）、"一键全量训练"入口、每模型"回滚"操作
- **BREAKING**: `GET /admin/models` 返回结构不变（仍为 `{models:[...]}`），字段从虚拟占位变为真实数据；`POST /admin/models/{id}/train` 路由废弃，由 `POST /admin/training/run` 取代

## Impact
- Affected specs: quant-trading-system（模型管理）、manage-prediction-history（预测管理与模型联动）
- Affected code:
  - backend: `internal/models/models.go`（+ModelTrainingLog）、`internal/handlers/admin_handler.go`（GetModels 真实化 + 训练中心接口）、`internal/services/model_service.go`（新增，代理 ML 服务）、`cmd/gateway/main.go`（路由）
  - ml-service: `app/models/model_manager.py`（版本快照 + rollback）、`app/api/models.py`（+GET /schedule、+POST /{period}/rollback）
  - frontend: `src/services/admin.ts`（hooks）、`src/components/ModelManagement/index.tsx`（训练中心）、`AdminPanel.tsx`（挂载）

---

## ADDED Requirements

### Requirement: 模型真实状态接入
系统 SHALL 让我在管理后台看到三个模型的真实状态（版本、准确率、最后训练时间、是否健康、参数），数据来自 ML 服务的真实模型注册表，禁止返回虚构占位数据。

#### Scenario: 查看模型状态
- **WHEN** 管理员进入管理后台 → 模型管理
- **THEN** 展示 short/medium/long 三个模型的真实 version/accuracy/last_train_time/status 与可编辑参数

### Requirement: 训练历史记录
系统 SHALL 持久化每次训练记录（周期、版本、准确率、样本量、触发方式 manual/daily/weekly、开始/完成时间、状态），支持分页查询。

#### Scenario: 记录手动训练
- **WHEN** 管理员触发一次训练且训练完成
- **THEN** 新增一条历史记录，含版本号、准确率与状态 success

#### Scenario: 记录定时训练
- **WHEN** 每日 17:00 轻量训练或周六 02:00 全量训练执行
- **THEN** 对应历史记录写入（触发方式为 daily/weekly）

### Requirement: 训练任务控制
系统 SHALL 支持在管理后台手动触发单周期或全量训练，训练完成后自动刷新模型状态与历史记录。

#### Scenario: 一键全量训练
- **WHEN** 管理员点击"全量训练"
- **THEN** 依次训练 short/medium/long 三个模型，完成后历史记录与模型状态同步刷新

### Requirement: 训练调度可视化
系统 SHALL 在管理后台展示 ML 定时任务清单（盘前预测 8:30、收盘预测 15:30、评估 16:00、轻量训练 17:00、全量训练 周六 02:00）及各自下次执行时间。

#### Scenario: 查看调度
- **WHEN** 管理员查看训练中心
- **THEN** 展示任务名称、触发规则与下次执行时间（基于 ML 调度器实时状态）

### Requirement: 模型版本对比与回滚
系统 SHALL 保留历史版本快照并在管理后台展示版本演进；支持将指定周期模型回滚到任一历史版本，回滚后模型热重载并更新 versions.json。

#### Scenario: 回滚模型
- **WHEN** 管理员选择某周期的一个历史版本点击"回滚"
- **THEN** 该周期模型恢复为该版本权重，versions.json 更新，预测立即使用回滚后的模型

---

## MODIFIED Requirements

### Requirement: 模型管理接口接入真实数据
原有 `GET /admin/models` 返回 `not_implemented` 与空数组。本版本 SHALL 代理 ML 服务模型状态，返回真实数据；原有 `POST /admin/models/{id}/train` 废弃，改为 `POST /admin/training/run`。

#### Scenario: 兼容返回结构
- **WHEN** 前端请求 `GET /admin/models`
- **THEN** 仍返回 `{models: [...]}`，但每项为真实模型数据（id 映射为 short_term/medium_term/long_term）

## REMOVED Requirements
无