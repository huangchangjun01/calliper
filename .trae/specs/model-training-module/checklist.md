# Checklist — 模型训练模块

## ML 服务能力
- [x] 训练保存权重前生成带版本号的历史快照文件
- [x] rollback(period, version) 恢复到指定版本并热重载、更新 versions.json
- [x] GET /api/v1/models/schedule 返回 5 个定时任务（盘前8:30/收盘15:30/评估16:00/轻量17:00/全量周六02:00）
- [x] POST /api/v1/models/{period}/rollback 检测非法周期与不存在版本

## 后端接口
- [x] GET /api/v1/admin/models 返回三个模型真实数据（版本/准确率/训练时间/状态），ML 不可达时降级提示
- [x] GET /api/v1/admin/training/history 分页返回训练记录（倒序）
- [x] POST /api/v1/admin/training/run 支持单周期与 all 全量，完成后写历史记录
- [x] GET /api/v1/admin/training/schedule 代理 ML 调度清单
- [x] POST /api/v1/admin/training/rollback 代理回滚并返回新版本
- [x] model_training_logs 表由 AutoMigrate 自动创建

## 前端训练中心
- [x] 管理后台展示调度任务列表（含下次执行时间）
- [x] 模型表格展示真实版本/准确率/训练时间/健康状态
- [x] 训练历史表格可查看（周期/版本/准确率/触发方式/时间/状态）
- [x] "全量训练"按钮可一键触发并刷新状态与历史
- [x] 每模型"回滚"操作可从历史版本选择并回滚
- [x] 不再调用废弃路由 POST /admin/models/{id}/train

## 验收
- [x] 后端 go build 通过；ML 服务启动无错误
- [x] 三端联调：模型真实数据、调度清单、手动训练→历史记录、版本回滚 全部可用
- [x] 验证后关闭额外启动的服务，交由用户自行验收