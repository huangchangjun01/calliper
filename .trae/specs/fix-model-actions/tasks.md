# Tasks — 模型评估/参数/预测功能完善

> 依据《模型评估/参数/预测功能完善 Spec》（.trae/specs/fix-model-actions/spec.md）编制。

## Task 1: ML 参数读写与持久化（ml-service）
- [x] `model_manager.py` 新增 `_period_default_params(period)` 返回各周期默认参数（short: hidden_size=128/num_layers=2/dropout=0.3；medium: xgb_max_depth=6/xgb_learning_rate=0.05/lgb_num_leaves=31；long: d_model=256/nhead=8/num_layers=4），对齐模型类构造默认值
- [x] 新增 `get_params(period)`：读取 `.ml-models/params_{period}.json`，无文件则返回默认值；校验 period 合法
- [x] 新增 `set_params(period, params)`：校验键名与值域（参考前端 Short/Medium/LongModelParams 字段），覆写 `params_{period}.json`
- [x] Validation：python 导入并调用 get/set_params 往返一致；非法键值被拒绝

## Task 2: ML 参数接口与评估真实化（ml-service）
- [x] `app/api/models.py` 新增 `GET /api/v1/models/{period}/params`（返回该周期当前参数对象）
- [x] 新增 `PUT /api/v1/models/{period}/params`（body 为参数对象，require_api_key，401 校验同现有接口风格）
- [x] `POST /api/v1/models/evaluate` 改为调用 `request.app.state.prediction_task.run_model_evaluation()`，返回 `[{period, accuracy, evaluated_at, ...}]`（沿用 EvaluationResult 结构），并将各周期 accuracy 回写 versions.json；run 失败返回 500 错误详情
- [x] 确认 `POST /api/v1/predictions/run` 已可触发全量预测（存在则保留）
- [x] Validation：三个接口在 ML 服务启动后 curl 验证；evaluate 返回真实 accuracy（有数据时非 0）

## Task 3: 训练应用持久化参数（ml-service）
- [x] `train_models.train_all_models(periods)`：构造各模型前调用 model_manager 的 get_params 读取（或直接读 params_{period}.json），传入模型构造器（short: hidden_size/num_layers/dropout；medium: xgb/ lgb 参数；long: d_model/nhead/num_layers）
- [x] ModelManager.train_single 训练时同样读取已保存参数（保持与 train_all_models 一致）
- [x] Validation：设 set_params 改 hidden_size=96 后训练，模型构造使用 96；versions/params 文件正常落盘

## Task 4: 后端代理接口（backend）
- [x] `admin_handler.go` 新增 `GetModelParams`（GET /admin/models/:id/params，代理 ML GET {period}/params，period 由 id 映射：short_term/medium_term/long_term 直接透传，否则 400）
- [x] 新增 `UpdateModelParams`（PUT，body 参数对象，代理 ML PUT）
- [x] 新增 `EvaluateModel`（POST，代理 ML POST /models/evaluate，返回 results 数组）
- [x] 新增 `PredictModel`（POST，代理 ML POST /predictions/run）
- [x] `GetModels` 聚合：对每个模型调用 ML params 接口填充 ModelInfo.Params；ML 不可达时 params 置空对象 `{}` 而非报错（整体仍返回 degraded 标记）
- [x] `main.go` 在 admin 组注册 4 条路由：GET/PUT `/models/:id/params`、POST `/models/:id/evaluate`、POST `/models/:id/predict`（注意与既有 GET `/models`、`/models/status` 不冲突）
- [x] Validation：`go build ./...` 通过；curl 验证四接口代理正确（含 400 非法 id）

## Task 5: 前端交互完善（frontend）
- [x] `ModelManagement/index.tsx`：评估/预测按钮成功后 `message.success`（当前可能无反馈）；保存参数成功提示保留
- [x] 参数弹窗：依赖 ModelInfo.params（Task 4 已填充）；仅当 params 非空时正常渲染字段，为空显示提示"暂无可编辑参数"
- [x] 训练/评估/预测等 mutation 的 loading 分离，避免一键全量训练时行内按钮全部转圈
- [x] Validation：`npx.cmd tsc --noEmit` 通过；管理后台评估/参数/预测点击均有反馈

## Task 6: 全链路验证
- [x] 后端 go build、ML 启动无错误
- [x] 启动三端：管理后台依次点 评估/参数（查看-修改-保存）/预测，均可用且数据正确（评估准确率真实、参数持久化后训练生效）
- [x] 验证后关闭额外启动的服务，交由用户自行验收

# Task Dependencies
- Task 2 依赖 Task 1（接口需底层 params 能力）
- Task 3 依赖 Task 1（训练读取参数）
- Task 4 依赖 Task 2（代理 ML 新接口）
- Task 5 依赖 Task 4（前端消费后端 params）
- Task 6 依赖全部

# 可并行执行的任务组
- 组 1：Task 1（ML params 底层）与 Task 4 中 GetModels 部分（后端聚合，不依赖 ML 新接口的 GET 亦可先实现；建议组 1 完成后并行 Task 2、Task 3）
- 组 2：Task 2、Task 3（接口层与训练应用，依赖 Task 1，可并行）
- 组 3：Task 5（前端，依赖 Task 2/4）