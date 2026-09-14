import { useState } from 'react';
import { Button, Table, Switch, Tag, Progress, message, Modal, Segmented } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { PlayCircleOutlined, PauseCircleOutlined, HistoryOutlined } from '@ant-design/icons';
import type { SimStatus, SimDecision, SimRecord } from '@/types';
import api from '@/services/api';
import dayjs from 'dayjs';

interface SimTradePanelProps {
  simStatus: SimStatus | null;
  loading: boolean;
  onToggle: () => Promise<void>;
}

const SIDE_MAP: Record<string, { label: string; className: string }> = {
  buy: { label: '买入', className: 'trade-buy' },
  sell: { label: '卖出', className: 'trade-sell' },
};

const RISK_STATUS_MAP: Record<string, { label: string; color: string }> = {
  normal: { label: '正常', color: 'success' },
  warning: { label: '预警', color: 'warning' },
  danger: { label: '危险', color: 'error' },
};

export default function SimTradePanel({ simStatus, loading, onToggle }: SimTradePanelProps) {
  const [toggling, setToggling] = useState(false);
  const running = simStatus?.running ?? false;

  // 历史决策弹框
  const [historyOpen, setHistoryOpen] = useState(false);
  const [historyDates, setHistoryDates] = useState<string[]>([]);
  const [historyDate, setHistoryDate] = useState<string | null>(null);
  const [historyRecords, setHistoryRecords] = useState<SimRecord[]>([]);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [datesLoading, setDatesLoading] = useState(false);

  const openHistory = async () => {
    setHistoryOpen(true);
    setDatesLoading(true);
    try {
      const data = await api.get<{ dates: string[] }>('/trading/sim/history/dates');
      setHistoryDates(data.dates || []);
      if (data.dates?.length > 0) {
        setHistoryDate(data.dates[0]);
        await loadHistory(data.dates[0]);
      } else {
        setHistoryDate(null);
        setHistoryRecords([]);
      }
    } catch {
      message.error('获取历史决策日期失败');
    } finally {
      setDatesLoading(false);
    }
  };

  const loadHistory = async (date: string) => {
    setHistoryLoading(true);
    try {
      const data = await api.get<{ records: SimRecord[] }>('/trading/sim/history', { date });
      setHistoryRecords(data.records || []);
    } catch {
      message.error('获取历史决策记录失败');
      setHistoryRecords([]);
    } finally {
      setHistoryLoading(false);
    }
  };

  const handleHistoryDateChange = (val: string) => {
    setHistoryDate(val);
    loadHistory(val);
  };

  const doToggle = async () => {
    setToggling(true);
    try {
      await onToggle();
      message.success(running ? '模拟交易已停止' : '模拟交易已启动');
    } catch {
      message.error('操作失败');
    } finally {
      setToggling(false);
    }
  };

  const handleToggle = async () => {
    // 停止模拟交易属于资金/状态变更操作，需二次确认，防止误触
    if (running) {
      Modal.confirm({
        title: '停止模拟交易',
        content: '停止后将暂停策略自动交易与实时决策推送。确定要停止吗？',
        okText: '确认停止',
        cancelText: '取消',
        okButtonProps: { danger: true },
        centered: true,
        onOk: doToggle,
      });
      return;
    }
    await doToggle();
  };

  const decisionColumns: ColumnsType<SimDecision> = [
    {
      title: '时间',
      dataIndex: 'createdAt',
      key: 'createdAt',
      width: 150,
      render: (val: string) => dayjs(val).format('MM-DD HH:mm:ss'),
    },
    {
      title: '股票',
      dataIndex: 'symbol',
      key: 'symbol',
      width: 110,
      render: (val: string, record) => (
        <span>
          <span className="decision-symbol">{val}</span>
          <span className="decision-name">{record.name}</span>
        </span>
      ),
    },
    {
      title: '方向',
      dataIndex: 'side',
      key: 'side',
      width: 70,
      render: (val: string) => {
        const s = SIDE_MAP[val] || { label: val, className: '' };
        return <span className={s.className}>{s.label}</span>;
      },
    },
    {
      title: '价格',
      dataIndex: 'price',
      key: 'price',
      width: 90,
      align: 'right',
      render: (val: number) => val?.toFixed(2),
    },
    {
      title: '数量',
      dataIndex: 'quantity',
      key: 'quantity',
      width: 80,
      align: 'right',
    },
    {
      title: '置信度',
      dataIndex: 'confidence',
      key: 'confidence',
      width: 100,
      render: (val: number) => (
        <Progress
          percent={Math.round(val * 100)}
          size="small"
          strokeColor={val >= 0.7 ? '#52c41a' : val >= 0.5 ? '#faad14' : '#ff4d4f'}
          format={(p) => `${p}%`}
        />
      ),
    },
    {
      title: '原因',
      dataIndex: 'reason',
      key: 'reason',
      ellipsis: true,
    },
  ];

  const recordColumns: ColumnsType<SimRecord> = [
    {
      title: '时间',
      dataIndex: 'createdAt',
      key: 'createdAt',
      width: 150,
      render: (val: string) => dayjs(val).format('MM-DD HH:mm:ss'),
    },
    {
      title: '股票',
      dataIndex: 'symbol',
      key: 'symbol',
      width: 110,
      render: (val: string, record) => (
        <span>
          <span className="decision-symbol">{val}</span>
          <span className="decision-name">{record.name}</span>
        </span>
      ),
    },
    {
      title: '方向',
      dataIndex: 'side',
      key: 'side',
      width: 70,
      render: (val: string) => {
        const s = SIDE_MAP[val] || { label: val, className: '' };
        return <span className={s.className}>{s.label}</span>;
      },
    },
    {
      title: '价格',
      dataIndex: 'price',
      key: 'price',
      width: 90,
      align: 'right',
      render: (val: number) => val?.toFixed(2),
    },
    {
      title: '数量',
      dataIndex: 'quantity',
      key: 'quantity',
      width: 80,
      align: 'right',
    },
    {
      title: '盈亏',
      dataIndex: 'profit',
      key: 'profit',
      width: 100,
      align: 'right',
      render: (val: number) => {
        const className = val > 0 ? 'profit-up' : val < 0 ? 'profit-down' : '';
        return <span className={className}>{val > 0 ? '+' : ''}{val.toFixed(2)}</span>;
      },
    },
    {
      title: '盈亏率',
      dataIndex: 'profitPercent',
      key: 'profitPercent',
      width: 90,
      align: 'right',
      render: (val: number) => {
        const className = val > 0 ? 'profit-up' : val < 0 ? 'profit-down' : '';
        return <span className={className}>{val > 0 ? '+' : ''}{val.toFixed(2)}%</span>;
      },
    },
  ];

  const risk = simStatus?.riskControl;

  return (
    <div className="sim-trade-panel">
      {/* 状态开关 */}
      <div className="sim-trade-status">
        <div className="sim-trade-status-left">
          <span className="sim-trade-status-label">模拟交易状态：</span>
          <Tag color={running ? 'success' : 'default'} icon={running ? <PlayCircleOutlined /> : <PauseCircleOutlined />}>
            {running ? '运行中' : '已停止'}
          </Tag>
        </div>
        <div className="sim-trade-status-right">
          <Button
            className="sim-trade-history-btn"
            icon={<HistoryOutlined />}
            onClick={openHistory}
          >
            历史决策
          </Button>
          <Switch
            checked={running}
            loading={toggling}
            onChange={handleToggle}
            checkedChildren="开"
            unCheckedChildren="关"
          />
        </div>
      </div>

      {/* 模拟账户资产 */}
      {simStatus?.account && (
        <div className="sim-trade-account">
          <div className="sim-trade-account-item">
            <span className="sim-trade-account-label">总资产</span>
            <span className="sim-trade-account-value">¥ {simStatus.account.totalAsset.toFixed(2)}</span>
          </div>
          <div className="sim-trade-account-item">
            <span className="sim-trade-account-label">可用资金</span>
            <span className="sim-trade-account-value">¥ {simStatus.account.availableCash.toFixed(2)}</span>
          </div>
          <div className="sim-trade-account-item">
            <span className="sim-trade-account-label">持仓市值</span>
            <span className="sim-trade-account-value">¥ {simStatus.account.marketValue.toFixed(2)}</span>
          </div>
          <div className="sim-trade-account-item">
            <span className="sim-trade-account-label">今日盈亏</span>
            <span className={`sim-trade-account-value ${simStatus.account.todayProfit >= 0 ? 'profit-up' : 'profit-down'}`}>
              {simStatus.account.todayProfit >= 0 ? '+' : ''}{simStatus.account.todayProfit.toFixed(2)}
            </span>
          </div>
          <div className="sim-trade-account-item">
            <span className="sim-trade-account-label">累计收益</span>
            <span className={`sim-trade-account-value ${simStatus.account.totalProfit >= 0 ? 'profit-up' : 'profit-down'}`}>
              {simStatus.account.totalProfit >= 0 ? '+' : ''}{simStatus.account.totalProfit.toFixed(2)}
            </span>
            <span className={`sim-trade-account-extra ${simStatus.account.totalProfitPercent >= 0 ? 'profit-up' : 'profit-down'}`}>
              {simStatus.account.totalProfitPercent >= 0 ? '+' : ''}{simStatus.account.totalProfitPercent.toFixed(2)}%
            </span>
          </div>
        </div>
      )}

      {/* 风险控制 */}
      {risk && (
        <div className="sim-trade-risk">
          <h4 className="sim-trade-section-title">
            风险控制
            <Tag
              color={RISK_STATUS_MAP[risk.status]?.color}
              style={{ marginLeft: 8 }}
            >
              {RISK_STATUS_MAP[risk.status]?.label}
            </Tag>
          </h4>
          <div className="sim-trade-risk-grid">
            <div className="sim-trade-risk-item">
              <span className="sim-trade-risk-label">单日亏损上限</span>
              <Progress
                percent={Math.min(Math.round((risk.currentDailyLoss / risk.maxDailyLoss) * 100), 100)}
                size="small"
                status={risk.status === 'danger' ? 'exception' : 'active'}
                format={() => `${risk.currentDailyLoss.toFixed(2)} / ${risk.maxDailyLoss.toFixed(2)}`}
              />
            </div>
            <div className="sim-trade-risk-item">
              <span className="sim-trade-risk-label">仓位比例</span>
              <Progress
                percent={Math.min(Math.round(risk.currentPositionRatio * 100), 100)}
                size="small"
                status={risk.status === 'danger' ? 'exception' : 'active'}
                format={() => `${(risk.currentPositionRatio * 100).toFixed(0)}% / ${(risk.maxPositionRatio * 100).toFixed(0)}%`}
              />
            </div>
            <div className="sim-trade-risk-item">
              <span className="sim-trade-risk-label">单只股票上限</span>
              <span className="sim-trade-risk-value">{(risk.maxSingleStockRatio * 100).toFixed(0)}%</span>
            </div>
          </div>
        </div>
      )}

      {/* 今日决策 */}
      <div className="sim-trade-section">
        <h4 className="sim-trade-section-title">今日决策</h4>
        <Table
          columns={decisionColumns}
          dataSource={simStatus?.decisions || []}
          rowKey="id"
          loading={loading}
          size="small"
          pagination={false}
          scroll={{ x: 700 }}
          locale={{ emptyText: '暂无决策' }}
        />
      </div>

      {/* 交易记录 */}
      <div className="sim-trade-section">
        <h4 className="sim-trade-section-title">交易记录</h4>
        <Table
          columns={recordColumns}
          dataSource={simStatus?.records || []}
          rowKey="id"
          loading={loading}
          size="small"
          pagination={{ pageSize: 10, showSizeChanger: false, showTotal: (total) => `共 ${total} 条` }}
          scroll={{ x: 690 }}
          locale={{ emptyText: '暂无交易记录' }}
        />
      </div>

      {/* 历史决策弹框 */}
      <Modal
        title="历史决策"
        open={historyOpen}
        onCancel={() => setHistoryOpen(false)}
        footer={null}
        width={720}
        centered
      >
        <div className="sim-trade-history">
          <div className="sim-trade-history-dates">
            <span className="sim-trade-history-dates-label">选择日期：</span>
            {datesLoading ? (
              <Tag>加载中...</Tag>
            ) : historyDates.length > 0 ? (
              <Segmented
                size="small"
                value={historyDate || undefined}
                options={historyDates.map((d) => ({ label: d, value: d }))}
                onChange={(val) => handleHistoryDateChange(String(val))}
              />
            ) : (
              <Tag>暂无历史决策</Tag>
            )}
          </div>
          <Table
            columns={recordColumns}
            dataSource={historyRecords}
            rowKey="id"
            loading={historyLoading}
            size="small"
            pagination={{ pageSize: 10, showSizeChanger: false, showTotal: (total) => `共 ${total} 条` }}
            scroll={{ x: 690 }}
            locale={{ emptyText: '该日期暂无决策记录' }}
          />
        </div>
      </Modal>
    </div>
  );
}