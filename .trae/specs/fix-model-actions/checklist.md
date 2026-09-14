# Checklist — 模型评估/参数/预测功能完善

## ML 参数能力
- [x] get_params(period) 无文件时返回各周期默认参数，往返一致
- [x] set_params(period, params) 校验键名与值域并持久化到 .ml-models/params_{period}.json
- [x] GET/PUT /api/v1/models/{period}/params 接口可用（require_api_key 校验）
- [x] 训练构造模型时应用已保存参数（如 hidden_size=96 生效）

## ML 评估真实化
- [x] POST /api/v1/models/evaluate 使用真实数据回算三周期准确率（有数据时非恒 0）
- [x] 评估结果回写 versions.json 的 accuracy

## 后端代理接口
- [x] GET/PUT /admin/models/:id/params 代理 ML，非法 id 返回 400
- [x] POST /admin/models/:id/evaluate 代理 ML 评估
- [x] POST /admin/models/:id/predict 代理 ML 预测任务（已异步化，立即返回 accepted）
- [x] GET /admin/models 每个模型 params 为真实对象而非 null；ML 不可达时 params 为空对象
- [x] 四条路由在 admin 组注册，与既有 /models、/models/status 不冲突

## 前端交互
- [x] 评估/预测/保存参数按钮点击有明确成功/失败提示
- [x] 参数弹窗展示当前值可编辑；params 为空时显示"暂无可编辑参数"
- [x] 各 mutation loading 独立，一键全量训练时不导致行内按钮全转圈
- [x] tsc --noEmit 通过

## 验收
- [x] 后端 go build、ML 启动无错误
- [x] 三端联调：评估（真实准确率）、参数（查看-修改-保存-训练生效）、预测 均可用
- [x] 验证后关闭额外启动的服务，交由用户自行验收