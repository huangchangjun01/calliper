# 模型评估/参数/预测功能完善 Spec

## Why
管理后台"模型管理"中 训练/调度/回滚 已可用，但每模型的【评估】【参数】【预测】三个按钮点击后全部失败：后端从未注册对应路由（前端调用 `POST /admin/models/{id}/evaluate`、`PUT /admin/models/{id}/params`、`POST /admin/models/{id}/predict`，全部 404）；ML 服务的评估接口为空实现（传空数据返回准确率恒 0）；模型参数无读写与持久化能力（模型表格 params 恒为 null，参数弹窗无法编辑保存）；预测无模型级触发入口。

## What Changes
- **后端补路由（评估/参数/预测）**：在 admin 分组注册三个代理接口，复用训练中心已有的 ML HTTP 客户端与 `mlSend` 辅助：
  - `POST /admin/models/:id/evaluate` → 代理 ML `POST /api/v1/models/evaluate`
  - `GET /admin/models/:id/params` → 代理 ML `GET /api/v1/models/{period}/params`
  - `PUT /admin/models/:id/params` → 代理 ML `PUT /api/v1/models/{period}/params`（body 为参数对象）
  - `POST /admin/models/:id/predict` → 代理 ML `POST /api/v1/predictions/run`（触发全量预测任务）
- **ML 评估真实化**：`POST /api/v1/models/evaluate` 改为调用 `prediction_task.run_model_evaluation()`（已存在，用真实数据回算三周期准确率），返回 `[{period, accuracy, ...}]` 并回写 versions.json 的 accuracy
- **ML 参数读写与持久化**：新增 `GET/PUT /api/v1/models/{period}/params`，参数持久化到 `.ml-models/params_{period}.json`；各周期默认值 = 模型类构造默认值（short: hidden_size/num_layers/dropout；medium: xgb_max_depth/xgb_learning_rate/lgb_num_leaves；long: d_model/nhead/num_layers）；PUT 校验键名与值域后覆写文件
- **训练应用参数**：`train_models.train_all_models`（及 ML `/train/{period}`）构造模型前读取 `params_{period}.json` 并应用（无文件则用默认值）
- **后端 `GetModels` 填充 params**：代理 ML params 聚合到每个 ModelInfo 的 `params` 字段，使前端参数弹窗可直接编辑；ML 不可达时 params 保持空对象而非报错
- **前端**：三个按钮的调用路径不变（admin.ts 已指向上述路径），组件内部补充 评估/预测 成功后提示与刷新；参数弹窗逻辑不变（数据来自 GetModels 的 params）；校验 missing:评估/预测的 loading 状态修复（共用 mutation loading）
- **BREAKING**: 无（补全既有占位功能，不改变既有可用 API）

## Impact
- Affected specs: model-training-module（训练中心的评估/参数/预测补全）
- Affected code:
  - backend: `internal/handlers/admin_handler.go`（+4 代理接口、GetModels 参数聚合）、`cmd/gateway/main.go`（+4 路由）
  - ml-service: `app/models/model_manager.py`（params 读写/默认值）、`app/api/models.py`（+params 接口、evaluate 真实化）、`app/api/predictions.py`（确认 /run 已可用）、`train_models.py`（训练应用参数）
  - frontend: `src/components/ModelManagement/index.tsx`（评估/预测交互反馈）、`src/services/admin.ts`（必要时类型补全）

---

## ADDED Requirements

### Requirement: 评估可用且结果真实
系统 SHALL 提供 `POST /admin/models/:id/evaluate`，代理 ML 使用真实行情数据评估三个模型，返回各周期准确率并回写版本元数据，评估结果不再恒为 0。

#### Scenario: 点击评估
- **WHEN** 管理员在模型管理中点击某模型"评估"
- **THEN** 后端代理 ML 评估，模型表格的准确率列更新为真实回算值（有真实数据时），并给出成功提示

#### Scenario: 无评估数据
- **WHEN** ML 无法加载真实数据做评估
- **THEN** 请求返回明确错误信息，前端提示"评估失败"，不产生虚假准确率

### Requirement: 参数查看与保存
系统 SHALL 提供 `GET/PUT /admin/models/:id/params`，将模型超参持久化到 `.ml-models/params_{period}.json`，参数弹窗可读取当前值并保存修改，训练时使用保存的参数。

#### Scenario: 查看参数
- **WHEN** 管理员打开某模型"参数"弹窗
- **THEN** 展示该周期默认/已保存的参数字段与当前值（如 hidden_size=128、dropout=0.3）

#### Scenario: 修改参数
- **WHEN** 管理员修改参数并保存
- **THEN** 参数持久化成功提示，后续训练据此参数构造并训练模型

### Requirement: 模型级预测触发
系统 SHALL 提供 `POST /admin/models/:id/predict`，代理 ML 预测任务触发接口，使管理员能从模型管理页触发预测。

#### Scenario: 点击预测
- **WHEN** 管理员点击某模型"预测"
- **THEN** 触发 ML 全量预测任务，成功后提示"预测任务已触发"并刷新相关数据

---

## MODIFIED Requirements

### Requirement: 模型表格 params 真实化
原 `GET /admin/models` 的每个 ModelInfo.params 恒为 null（参数弹窗无法编辑）。本版本 SHALL 让后端聚合 ML 参数接口，params 包含该周期全部可编辑超参的当前值；ML 不可达时 params 为空对象、前端显示"暂不可获取参数"。

#### Scenario: 打开参数弹窗
- **WHEN** 模型表格已加载且 params 非空
- **THEN** 参数弹窗展示各字段当前值，保存后生效

### Requirement: ML 评估接入真实数据
原 `POST /api/v1/models/evaluate` 调用 `evaluate_all({})` 传空 truth 数据，三个周期准确率恒 0。本版本 SHALL 改用 `prediction_task.run_model_evaluation()`（真实数据回算）并回写 versions.json。

#### Scenario: 评估返回真实准确率
- **WHEN** 有足够真实历史数据
- **THEN** 各周期返回基于真实数据的验证准确率

## REMOVED Requirements
无