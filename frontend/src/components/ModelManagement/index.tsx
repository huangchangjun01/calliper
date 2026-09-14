import { useEffect, useMemo, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { ApiError } from '@/services/api';
import {
  Table,
  Button,
  Tag,
  Modal,
  Alert,
  List,
  Empty,
  InputNumber,
  Space,
  Tooltip,
  message,
} from 'antd';
import {
  PlayCircleOutlined,
  ExperimentOutlined,
  SettingOutlined,
  ThunderboltOutlined,
  ReloadOutlined,
  UndoOutlined,
  ThunderboltFilled,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import {
  useModels,
  useEvaluateModel,
  useUpdateModelParams,
  useTriggerPrediction,
  useTrainingHistory,
  useTrainingSchedule,
  useRunTraining,
  useRollbackModel,
  type ModelInfo,
  type TrainingLog,
  type TrainingJob,
  type ShortModelParams,
  type MediumModelParams,
  type LongModelParams,
  type TrainingPeriod,
} from '@/services/admin';
import dayjs from 'dayjs';

const PERIOD_LABELS: Record<string, string> = {
  short: '短期',
  medium: '中短期',
  long: '长期',
};

const PERIOD_LABELS_API: Record<string, string> = {
  short_term: '短期',
  medium_term: '中短期',
  long_term: '长期',
};

// 前端周期 → 训练接口周期
const PERIOD_TO_API: Record<string, TrainingPeriod> = {
  short: 'short_term',
  medium: 'medium_term',
  long: 'long_term',
};

const TRIGGER_LABELS: Record<string, string> = {
  manual: '手动',
  daily: '每日',
  weekly: '每周',
};

const LOG_STATUS: Record<string, { text: string; color: string }> = {
  running: { text: '运行中', color: 'blue' },
  success: { text: '成功', color: 'green' },
  failed: { text: '失败', color: 'red' },
};

// 训练耗时展示：不足 1 分钟显示秒，超过则显示 Xm Ys
function formatDuration(sec?: number | null): string {
  if (sec == null || sec < 0) return '-';
  if (sec < 60) return `${sec}s`;
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  return s > 0 ? `${m}m ${s}s` : `${m}m`;
}

// 训练历史/日志时间格式化：空/无效/Go 零时间（年份<2000）显示 '-'
function formatTimeLike(value?: string | null): string {
  if (!value) return '-';
  const d = dayjs(value);
  if (!d.isValid() || d.year() < 2000) return '-';
  return d.format('YYYY-MM-DD HH:mm');
}

// 参数名称 + hover 提示（悬停展示该参数的含义与调整影响，无问号图标/help 光标）
function ParamLabel({ label, tip }: { label: string; tip: string }) {
  return (
    <Tooltip title={tip} placement="topLeft" overlayStyle={{ maxWidth: 320 }}>
      <label
        style={{
          borderBottom: '1px dashed rgba(0, 0, 0, 0.4)',
          cursor: 'default',
          lineHeight: 1.6,
        }}
      >
        {label}
      </label>
    </Tooltip>
  );
}

// 调度任务名称去前缀展示
const JOB_LABEL_RULES: Array<[RegExp, string]> = [
  [/盘前|pre.?market|premarket/i, '盘前预测'],
  [/轻量|light|lightweight/i, '每日轻量重训练'],
  [/每周|weekly/i, '每周全量重训练'],
  [/评估|eval/i, '每日模型评估'],
  [/预测|predict|forecast/i, '每日预测'],
  [/每日|daily/i, '每日预测'],
];

function displayJobName(job: TrainingJob): string {
  const name = job.name || job.id || '';
  for (const [re, label] of JOB_LABEL_RULES) {
    if (re.test(name)) return label;
  }
  // 兜底：去掉形如 "xxx:" / "xxx：" 的前缀
  return name.replace(/^[^:：-]*[:：-]\s*/, '').trim() || name;
}

export default function ModelManagement() {
  const { data: modelsData, isLoading: modelsLoading } = useModels();
  const models = modelsData?.models ?? [];
  const degraded = modelsData?.degraded ?? false;

  const queryClient = useQueryClient();
  const evaluateMutation = useEvaluateModel();
  const paramsMutation = useUpdateModelParams();
  const predictMutation = useTriggerPrediction();
  // 全量训练与行内训练各自独立实例，避免 loading 互相阻塞
  const runTrainingAll = useRunTraining();
  const runTrainingOne = useRunTraining();
  const rollbackMutation = useRollbackModel();

  const {
    data: history = [],
    isLoading: historyLoading,
    refetch: refetchHistory,
  } = useTrainingHistory(50, 0);
  const {
    data: jobs = [],
    isLoading: jobsLoading,
    refetch: refetchJobs,
  } = useTrainingSchedule();

  const [paramsVisible, setParamsVisible] = useState(false);
  const [editingModel, setEditingModel] = useState<ModelInfo | null>(null);
  const [paramsForm, setParamsForm] = useState<Record<string, number>>({});

  const [rollbackVisible, setRollbackVisible] = useState(false);
  const [rollbackModel, setRollbackModel] = useState<ModelInfo | null>(null);

  const rollingLogs = useMemo(() => {
    if (!rollbackModel) return [];
    const targetPeriod = PERIOD_TO_API[rollbackModel.period];
    return history.filter((log) => log.period === targetPeriod);
  }, [rollbackModel, history]);

  // 正在训练（前台有 running 记录）的周期集合，用于驱动对应模型行内「训练」按钮状态，
  // 使每个按钮只反映当前模型的训练进度，互不影响。
  const runningPeriods = useMemo(() => {
    const set = new Set<string>();
    for (const log of history) {
      if (log.status === 'running') set.add(log.period);
    }
    return set;
  }, [history]);

  // 正在评估的模型周期 -> 触发评估前的准确率。
  // 评估为后台异步执行，界面以「准确率是否发生变化」判断评估完成并复位按钮，
  // 使每个「评估」按钮只反馈当前模型自己的评估状态，互不影响。
  const [evaluatingIds, setEvaluatingIds] = useState<Record<string, number>>({});

  useEffect(() => {
    if (Object.keys(evaluatingIds).length === 0) return;
    setEvaluatingIds((prev) => {
      let changed = false;
      const next = { ...prev };
      for (const [period, baseAcc] of Object.entries(prev)) {
        const model = models.find((mdl) => mdl.id === period);
        if (model && Math.abs(model.accuracy - baseAcc) > 1e-9) {
          delete next[period];
          changed = true;
        }
      }
      return changed ? next : prev;
    });
  }, [models, evaluatingIds]);

  const handleRefresh = async () => {
    try {
      await Promise.all([refetchHistory(), refetchJobs()]);
      message.success('已刷新');
    } catch {
      message.error('刷新失败');
    }
  };

  const handleRunTraining = async (
    period: TrainingPeriod,
    mutation: ReturnType<typeof useRunTraining>
  ) => {
    try {
      await mutation.mutateAsync(period);
      message.success('训练已触发，将在后台异步执行');
    } catch (err) {
      // 后端业务错误（如「该模型正在训练中」）直接透出具体原因，其余按网络/未知失败处理
      if (err instanceof ApiError && err.message) {
        message.error(err.message);
      } else {
        message.error('触发训练失败');
      }
    }
  };

  const handleEvaluate = async (id: string) => {
    const model = models.find((mdl) => mdl.id === id);
    // 记录触发前的准确率，用于评估完成（准确率变化）后自动复位该行的「评估」按钮
    setEvaluatingIds((prev) => ({ ...prev, [id]: model?.accuracy ?? -1 }));
    try {
      await evaluateMutation.mutateAsync(id);
      message.success('评估已触发，将在后台执行');
      queryClient.invalidateQueries({ queryKey: ['admin', 'models'] });
    } catch {
      setEvaluatingIds((prev) => {
        const next = { ...prev };
        delete next[id];
        return next;
      });
      message.error('评估失败');
    }
  };

  const handlePredict = async (id: string) => {
    try {
      await predictMutation.mutateAsync(id);
      message.success('预测任务已触发');
      queryClient.invalidateQueries({ queryKey: ['admin', 'models'] });
    } catch {
      message.error('触发失败');
    }
  };

  const handleParamsOpen = (model: ModelInfo) => {
    setEditingModel(model);
    const form: Record<string, number> = {};
    for (const [key, value] of Object.entries(model.params)) {
      form[key] = value as number;
    }
    setParamsForm(form);
    setParamsVisible(true);
  };

  const handleParamsSave = async () => {
    if (!editingModel) return;
    try {
      await paramsMutation.mutateAsync({
        id: editingModel.id,
        params: paramsForm,
      });
      message.success('参数已保存');
      setParamsVisible(false);
      queryClient.invalidateQueries({ queryKey: ['admin', 'models'] });
    } catch {
      message.error('保存失败');
    }
  };

  // 回滚
  const handleRollbackOpen = (model: ModelInfo) => {
    setRollbackModel(model);
    setRollbackVisible(true);
  };

  const handleRollback = async (version: string) => {
    if (!rollbackModel) return;
    try {
      await rollbackMutation.mutateAsync({
        period: PERIOD_TO_API[rollbackModel.period] as string,
        version,
      });
      message.success(`已回滚到 ${version}`);
      setRollbackVisible(false);
    } catch {
      message.error('回滚失败');
    }
  };

  const renderParamFields = () => {
    if (!editingModel) return null;

    if (editingModel.period === 'short') {
      const p = paramsForm as unknown as ShortModelParams;
      return (
        <>
          <div className="admin-form-item">
            <ParamLabel
              label="hidden_size"
              tip="LSTM 隐藏层单元数：越大模型拟合能力越强，但训练更慢、更占内存；过小容易欠拟合。建议 64~256。"
            />
            <InputNumber
              value={p.hidden_size}
              onChange={(v) => setParamsForm({ ...paramsForm, hidden_size: v ?? 128 })}
              min={32}
              max={512}
              style={{ width: '100%' }}
            />
          </div>
          <div className="admin-form-item">
            <ParamLabel
              label="num_layers"
              tip="LSTM 网络层数：层数增加可捕捉更复杂的时序模式，但训练明显变慢、更易过拟合。建议 1~3。"
            />
            <InputNumber
              value={p.num_layers}
              onChange={(v) => setParamsForm({ ...paramsForm, num_layers: v ?? 2 })}
              min={1}
              max={8}
              style={{ width: '100%' }}
            />
          </div>
          <div className="admin-form-item">
            <ParamLabel
              label="dropout"
              tip="丢弃率（0~1）：训练时随机丢弃节点以缓解过拟合；过大（>0.5）会导致欠拟合，过小（=0）易过拟合。仅在训练阶段生效，不影响预测。建议 0.1~0.4。"
            />
            <InputNumber
              value={p.dropout}
              onChange={(v) => setParamsForm({ ...paramsForm, dropout: v ?? 0.2 })}
              min={0}
              max={0.8}
              step={0.05}
              style={{ width: '100%' }}
            />
          </div>
        </>
      );
    }

    if (editingModel.period === 'medium') {
      const p = paramsForm as unknown as MediumModelParams;
      return (
        <>
          <div className="admin-form-item">
            <ParamLabel
              label="XGBoost max_depth"
              tip="XGBoost 决策树最大深度：深度越大拟合越强，但越容易过拟合、训练越慢。建议 3~8。"
            />
            <InputNumber
              value={p.xgb_max_depth}
              onChange={(v) => setParamsForm({ ...paramsForm, xgb_max_depth: v ?? 6 })}
              min={2}
              max={15}
              style={{ width: '100%' }}
            />
          </div>
          <div className="admin-form-item">
            <ParamLabel
              label="XGBoost learning_rate"
              tip="XGBoost 学习率：越小收敛越稳定、精度通常越高，但需要更多树、训练更慢；过大会导致训练不稳定。建议 0.01~0.1。"
            />
            <InputNumber
              value={p.xgb_learning_rate}
              onChange={(v) => setParamsForm({ ...paramsForm, xgb_learning_rate: v ?? 0.05 })}
              min={0.001}
              max={1}
              step={0.01}
              style={{ width: '100%' }}
            />
          </div>
          <div className="admin-form-item">
            <ParamLabel
              label="LightGBM num_leaves"
              tip="LightGBM 叶子节点数：越大模型越复杂、拟合越强，但更慢且更易过拟合。建议 16~63（通常随 max_bin 增大而增大）。"
            />
            <InputNumber
              value={p.lgb_num_leaves}
              onChange={(v) => setParamsForm({ ...paramsForm, lgb_num_leaves: v ?? 31 })}
              min={7}
              max={255}
              style={{ width: '100%' }}
            />
          </div>
        </>
      );
    }

    const p = paramsForm as unknown as LongModelParams;
    return (
      <>
        <div className="admin-form-item">
          <ParamLabel
            label="d_model"
            tip="Transformer 特征维度：越大表达能力越强，但训练显著变慢、内存占用越高；过小则参数不足、易欠拟合。建议 128~512，通常取 64 的倍数。"
          />
          <InputNumber
            value={p.d_model}
            onChange={(v) => setParamsForm({ ...paramsForm, d_model: v ?? 256 })}
            min={64}
            max={1024}
            step={64}
            style={{ width: '100%' }}
          />
        </div>
        <div className="admin-form-item">
          <ParamLabel
            label="nhead"
            tip="多头注意力头数：头数越多越能捕捉不同的依赖模式，但计算量增大；必须能被 d_model 整除（如 d_model=256、nhead=8）。建议 4~16。"
          />
          <InputNumber
            value={p.nhead}
            onChange={(v) => setParamsForm({ ...paramsForm, nhead: v ?? 8 })}
            min={1}
            max={32}
            style={{ width: '100%' }}
          />
        </div>
        <div className="admin-form-item">
          <ParamLabel
            label="num_layers"
            tip="Transformer 编码器层数：层数越多表达力越强，但训练更慢、更容易过拟合。建议 2~6。"
          />
          <InputNumber
            value={p.num_layers}
            onChange={(v) => setParamsForm({ ...paramsForm, num_layers: v ?? 4 })}
            min={1}
            max={12}
            style={{ width: '100%' }}
          />
        </div>
      </>
    );
  };

  const modelColumns: ColumnsType<ModelInfo> = [
    {
      title: '模型名称',
      dataIndex: 'name',
      key: 'name',
      width: 180,
    },
    {
      title: '周期',
      dataIndex: 'period',
      key: 'period',
      width: 80,
      render: (p: string) => <Tag>{PERIOD_LABELS[p] || p}</Tag>,
    },
    {
      title: '版本',
      dataIndex: 'version',
      key: 'version',
      width: 100,
    },
    {
      title: '准确率',
      dataIndex: 'accuracy',
      key: 'accuracy',
      width: 100,
      align: 'right',
      render: (v: number) => `${((v ?? 0) * 100).toFixed(1)}%`,
    },
    {
      title: '最后训练',
      dataIndex: 'last_train_time',
      key: 'last_train_time',
      width: 160,
      render: (ts: string | null) => formatTimeLike(ts),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 90,
      render: (s: string) => {
        if (s === 'ready') return <Tag color="green">就绪</Tag>;
        if (s === 'error') return <Tag color="red">异常</Tag>;
        return <Tag>{s}</Tag>;
      },
    },
    {
      title: '操作',
      key: 'actions',
      width: 400,
      render: (_: unknown, record: ModelInfo) => {
        const training = runningPeriods.has(PERIOD_TO_API[record.period]);
        const evaluating = Boolean(evaluatingIds[record.id]);
        return (
          <Space wrap>
            <Button
              type="link"
              icon={<PlayCircleOutlined />}
              onClick={() => handleRunTraining(PERIOD_TO_API[record.period], runTrainingOne)}
              loading={training}
              disabled={training}
            >
              训练
            </Button>
            <Button
              type="link"
              icon={<ExperimentOutlined />}
              onClick={() => handleEvaluate(record.id)}
              loading={evaluating}
              disabled={evaluating}
            >
              评估
            </Button>
            <Button
              type="link"
              icon={<SettingOutlined />}
              onClick={() => handleParamsOpen(record)}
            >
              参数
            </Button>
            <Button
              type="link"
              icon={<ThunderboltOutlined />}
              onClick={() => handlePredict(record.id)}
              loading={predictMutation.isPending}
            >
              预测
            </Button>
            <Button
              type="link"
              icon={<UndoOutlined />}
              onClick={() => handleRollbackOpen(record)}
            >
              回滚
            </Button>
          </Space>
        );
      },
    },
  ];

  const scheduleColumns: ColumnsType<TrainingJob> = [
    {
      title: '任务名称',
      key: 'name',
      width: 200,
      render: (_: unknown, record: TrainingJob) => displayJobName(record),
    },
    {
      title: '触发规则',
      dataIndex: 'trigger',
      key: 'trigger',
      width: 160,
    },
    {
      title: '下次执行',
      dataIndex: 'next_run',
      key: 'next_run',
      render: (ts: string | null) => formatTimeLike(ts),
    },
  ];

  const historyColumns: ColumnsType<TrainingLog> = [
    {
      title: '时间',
      dataIndex: 'started_at',
      key: 'started_at',
      width: 160,
      render: (ts: string | null) => formatTimeLike(ts),
    },
    {
      title: '周期',
      dataIndex: 'period',
      key: 'period',
      width: 90,
      render: (p: string) => PERIOD_LABELS_API[p] || p,
    },
    {
      title: '版本',
      dataIndex: 'version',
      key: 'version',
      width: 110,
    },
    {
      title: '准确率',
      dataIndex: 'accuracy',
      key: 'accuracy',
      width: 90,
      align: 'right',
      render: (v: number) => `${((v ?? 0) * 100).toFixed(2)}%`,
    },
    {
      title: '触发方式',
      dataIndex: 'trigger_type',
      key: 'trigger_type',
      width: 90,
      render: (t: string) => TRIGGER_LABELS[t] || t,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 90,
      render: (s: string) => {
        const info = LOG_STATUS[s] || { text: s, color: 'default' };
        return <Tag color={info.color}>{info.text}</Tag>;
      },
    },
    {
      title: '耗时',
      dataIndex: 'duration_sec',
      key: 'duration_sec',
      width: 90,
      align: 'right',
      render: (v: number | null | undefined, record: TrainingLog) =>
        record.status === 'running' ? '-' : formatDuration(v),
    },
    {
      title: '错误信息',
      dataIndex: 'error_message',
      key: 'error_message',
      ellipsis: true,
      render: (m?: string | null) => m || '-',
    },
  ];

  return (
    <div>
      {/* 顶部操作区 */}
      <div className="admin-section">
        <div className="admin-section-header">
          <h3>训练中心</h3>
          <Space>
            <Button
              type="primary"
              icon={<ThunderboltFilled />}
              onClick={() => handleRunTraining('all', runTrainingAll)}
              loading={runTrainingAll.isPending || runningPeriods.size > 0}
              disabled={runningPeriods.size > 0}
            >
              全量训练
            </Button>
            <Button icon={<ReloadOutlined />} onClick={handleRefresh}>
              刷新
            </Button>
          </Space>
        </div>
      </div>

      {/* 训练调度清单 */}
      <div className="admin-section">
        <div className="admin-section-header">
          <h3>训练调度清单</h3>
        </div>
        {jobs.length === 0 && !jobsLoading ? (
          <Empty description="暂无调度任务" />
        ) : (
          <Table
            columns={scheduleColumns}
            dataSource={jobs}
            rowKey="id"
            loading={jobsLoading}
            pagination={false}
            size="small"
            locale={{ emptyText: '暂无数据' }}
          />
        )}
      </div>

      {/* 模型表格 */}
      <div className="admin-section">
        <div className="admin-section-header">
          <h3>模型</h3>
        </div>
        {degraded ? (
          <Alert
            type="warning"
            showIcon
            message="模型服务暂不可用"
            description="ML 服务处于 degraded 状态，模型列表暂不可用，请稍后重试。"
          />
        ) : (
          <Table
            columns={modelColumns}
            dataSource={models}
            rowKey="id"
            loading={modelsLoading}
            pagination={false}
            size="middle"
            locale={{ emptyText: '暂无数据' }}
          />
        )}
      </div>

      {/* 训练历史 */}
      <div className="admin-section">
        <div className="admin-section-header">
          <h3>训练历史</h3>
        </div>
        <Table
          columns={historyColumns}
          dataSource={history}
          rowKey="id"
          loading={historyLoading}
          pagination={{ pageSize: 10 }}
          size="middle"
          locale={{ emptyText: '暂无数据' }}
        />
      </div>

      {/* 参数配置弹窗 */}
      <Modal
        title={`${editingModel?.name || ''} - 参数配置`}
        open={paramsVisible}
        onOk={handleParamsSave}
        onCancel={() => setParamsVisible(false)}
        confirmLoading={paramsMutation.isPending}
        destroyOnClose
        width={480}
      >
        <div className="admin-form">
          {paramsVisible && Object.keys(paramsForm).length === 0 ? (
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description="暂无可编辑参数"
            />
          ) : (
            renderParamFields()
          )}
        </div>
      </Modal>

      {/* 回滚弹窗 */}
      <Modal
        title={`${rollbackModel?.name || ''} - 版本回滚`}
        open={rollbackVisible}
        onCancel={() => setRollbackVisible(false)}
        footer={null}
        width={560}
      >
        {rollingLogs.length === 0 ? (
          <Empty description="暂无该周期的历史训练记录" />
        ) : (
          <List
            size="small"
            dataSource={rollingLogs}
            renderItem={(log) => {
              const info = LOG_STATUS[log.status] || {
                text: log.status,
                color: 'default',
              };
              return (
                <List.Item
                  actions={[
                    <Button
                      key="rollback"
                      type="link"
                      size="small"
                      icon={<UndoOutlined />}
                      disabled={log.status !== 'success'}
                      loading={rollbackMutation.isPending}
                      onClick={() => handleRollback(log.version)}
                    >
                      回滚到此版本
                    </Button>,
                  ]}
                >
                  <List.Item.Meta
                    title={
                      <Space>
                        <span>{log.version}</span>
                        <Tag color={info.color}>{info.text}</Tag>
                      </Space>
                    }
                    description={
                      <Space split="·" size={4}>
                        <span>{formatTimeLike(log.started_at)}</span>
                        <span>准确率 {((log.accuracy ?? 0) * 100).toFixed(2)}%</span>
                      </Space>
                    }
                  />
                </List.Item>
              );
            }}
          />
        )}
      </Modal>
    </div>
  );
}