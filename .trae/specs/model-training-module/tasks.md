# Tasks — 模型训练模块

> 依据《模型训练模块 Spec》（.trae/specs/model-training-module/spec.md）编制。

## Task 1: ML 服务版本快照与回滚（ml-service）
- [x] `model_manager._save_model`：写入新权重前，将当前已有文件备份为带版本号快照（如 `{period}_model_v{ver}.{ext}`），保留全部历史版本
- [x] 新增 `model_manager.rollback(period, version)`：按版本号从快照恢复权重并覆盖当前文件、更新 versions.json、热重载模型
- [x] Validation：手动训练后出现版本快照文件；rollback 后模型预测结果对应回滚版本

## Task 2: ML 服务新增接口（ml-service）
- [x] `GET /api/v1/models/schedule`：返回 APScheduler 任务列表（id/name/next_run/trigger）
- [x] `POST /api/v1/models/{period}/rollback`：按 Task 1 的 rollback 能力暴露接口（校验 period 合法、版本存在）
- [x] Validation：schedule 返回 5 个定时任务；rollback 接口响应成功且 versions.json 更新

## Task 3: 后端模型数据真实化（backend）
- [x] 新增 `ModelTrainingLog` 模型（period/version/accuracy/sample_count/trigger_type/status/started_at/finished_at/error_message），AutoMigrate 建表
- [x] `GetModels` 改为代理 ML 服务 `GET /models/status`，映射为现有 `ModelInfo` 字段（id=short_term/medium_term/long_term），ML 不可达时返回降级提示而非空壳
- [x] Validation：`GET /api/v1/admin/models` 返回三个真实模型状态；ML 停止时响应含降级标记

## Task 4: 后端训练中心接口（backend）
- [x] `GET /api/v1/admin/training/history`：分页查询 ModelTrainingLog，按 started_at 倒序
- [x] `POST /api/v1/admin/training/run`：body={period|"all"}，代理 ML `POST /models/train/{period}`（all 则依次三个），同步等待完成后写 History（成功含版本/准确率，失败含 error_message）
- [x] `GET /api/v1/admin/training/schedule`：代理 ML `GET /models/schedule`
- [x] `POST /api/v1/admin/training/rollback`：body={period, version}，代理 ML rollback 接口
- [x] main.go 注册上述路由（admin 分组，admin 鉴权）
- [x] Validation：四个接口 curl 验证返回结构正确；训练一次后 History 出现记录

## Task 5: 前端训练中心（frontend）
- [x] `admin.ts` 新增 hooks：useTrainingHistory / useRunTraining / useTrainingSchedule / useRollbackModel；`useModels` 改接真实字段（period 映射）
- [x] `ModelManagement` 扩展为"训练中心"：调度任务列表（名称/时间/下次执行，来自 schedule）+ 模型表格（真实版本/准确率/训练时间/健康状态）
- [x] 新增"训练历史"表格（周期/版本/准确率/触发方式/时间/状态）+ 训练历史弹窗或内嵌
- [x] 新增"全量训练"按钮（run all）与每模型"回滚"操作（确认弹窗选版本）
- [x] 废弃对 `POST /admin/models/{id}/train` 的调用，改走 training/run
- [x] Validation：`tsc --noEmit` 通过；管理后台模型管理展示真实数据，训练/历史/回滚交互可用

## Task 6: 全链路验证
- [x] 后端 `go build ./...` 通过；ML 服务启动无语法/导入错误
- [x] 启动三端，管理后台验证：模型真实状态、调度清单、手动训练→历史记录、版本回滚
- [x] 验证完成后关闭新增测试服务，交由用户自行启动验收

# Task Dependencies
- Task 2 依赖 Task 1（接口需底层能力）
- Task 4 依赖 Task 2、Task 3（训练历史表 + ML 接口）
- Task 5 依赖 Task 3、Task 4（前端消费后端接口）
- Task 6 依赖全部

# 可并行执行的任务组
- 组 1：Task 1、Task 3（ML 快照 与 后端历史表，互不依赖，可并行）
- 组 2：Task 2、Task 4（接口层，分别依赖组 1 产物）
- 组 3：Task 5（前端，依赖组 2）