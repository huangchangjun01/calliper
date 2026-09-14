import { useCallback } from 'react';
import { useSearchParams } from 'react-router-dom';
import { Tabs, Spin } from 'antd';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import api from '@/services/api';
import OrderForm, { type OrderRequestBody } from '@/components/OrderForm';
import OrderList from '@/components/OrderList';
import PositionList from '@/components/PositionList';
import AccountOverview from '@/components/AccountOverview';
import SimTradePanel from '@/components/SimTradePanel';
import type { Order, Position, AccountInfo, SimStatus } from '@/types';
import './TradingPanel.css';

const TAB_ITEMS = [
  { key: 'real', label: '真实交易' },
  { key: 'sim', label: '模拟交易' },
];

/** 后端 /trading/sim/status 返回的原始结构（snake_case） */
interface SimStatusResponse {
  running: boolean;
  account: {
    total_asset: number;
    available_cash: number;
    market_value: number;
    today_profit: number;
    today_profit_percent: number;
    total_profit: number;
    total_profit_percent: number;
    initial_capital: number;
    start_date: string;
    is_running: boolean;
  };
  decisions: Array<{
    id: number;
    symbol: string;
    name: string;
    side: 'buy' | 'sell';
    price: number;
    quantity: number;
    confidence: number;
    reason: string;
    created_at: string;
  }>;
  records: Array<{
    id: number;
    symbol: string;
    name: string;
    side: 'buy' | 'sell';
    price: number;
    quantity: number;
    profit: number;
    profit_percent: number;
    created_at: string;
  }>;
  risk_control: {
    max_daily_loss: number;
    current_daily_loss: number;
    max_position_ratio: number;
    current_position_ratio: number;
    max_single_stock_ratio: number;
    status: 'normal' | 'warning' | 'danger';
  };
}

export default function TradingPanel() {
  const [searchParams, setSearchParams] = useSearchParams();
  // tab 状态写入 URL，离开/刷新后仍保留
  const activeTab = searchParams.get('tab') === 'sim' ? 'sim' : 'real';
  const queryClient = useQueryClient();

  const setActiveTab = useCallback((key: string) => {
    const p = new URLSearchParams(searchParams);
    if (key === 'sim') p.set('tab', 'sim');
    else p.delete('tab');
    setSearchParams(p, { replace: true });
  }, [searchParams, setSearchParams]);

  // ========== 真实交易数据 ==========

  const { data: orders, isLoading: ordersLoading } = useQuery({
    queryKey: ['orders'],
    queryFn: async () => {
      const data = await api.get<{ orders: Order[]; total: number; limit: number; offset: number }>('/trading/orders');
      return data.orders;
    },
    enabled: activeTab === 'real',
    refetchInterval: 30000,
  });

  const { data: positions, isLoading: positionsLoading } = useQuery({
    queryKey: ['positions'],
    queryFn: async () => {
      const data = await api.get<{ positions: Position[] }>('/trading/positions');
      return data.positions;
    },
    enabled: activeTab === 'real',
    refetchInterval: 30000,
  });

  const { data: account, isLoading: accountLoading } = useQuery({
    queryKey: ['account'],
    queryFn: async () => {
      const data = await api.get<{
        total_assets: number;
        available_cash: number;
        frozen_cash?: number;
        market_value: number;
        total_pnl: number;
        today_pnl: number;
        today_return: number;
      }>('/trading/account');
      return {
        totalAsset: data.total_assets,
        availableCash: data.available_cash,
        marketValue: data.market_value,
        totalProfit: data.total_pnl,
        todayProfit: data.today_pnl,
        todayProfitPercent: data.today_return,
      } as AccountInfo;
    },
    enabled: activeTab === 'real',
    refetchInterval: 30000,
  });

  const placeOrderMutation = useMutation({
    mutationFn: (order: OrderRequestBody) => api.post('/trading/order', order),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['orders'] });
      queryClient.invalidateQueries({ queryKey: ['account'] });
      queryClient.invalidateQueries({ queryKey: ['positions'] });
    },
  });

  const cancelOrderMutation = useMutation({
    mutationFn: (orderId: string) => api.delete(`/trading/order/${orderId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['orders'] });
    },
  });

  const handlePlaceOrder = useCallback(async (order: OrderRequestBody) => {
    await placeOrderMutation.mutateAsync(order);
  }, [placeOrderMutation]);

  const handleCancelOrder = useCallback(async (orderId: string) => {
    await cancelOrderMutation.mutateAsync(orderId);
  }, [cancelOrderMutation]);

  // ========== 模拟交易数据 ==========

  const {
    data: simStatus,
    isLoading: simLoading,
    refetch: refetchSimStatus,
  } = useQuery({
    queryKey: ['simStatus'],
    queryFn: async () => {
      const data = await api.get<SimStatusResponse>('/trading/sim/status');
      return {
        running: data.running,
        account: {
          totalAsset: data.account.total_asset,
          availableCash: data.account.available_cash,
          marketValue: data.account.market_value,
          todayProfit: data.account.today_profit,
          todayProfitPercent: data.account.today_profit_percent,
          totalProfit: data.account.total_profit,
          totalProfitPercent: data.account.total_profit_percent,
          riskLevel: '',
        } as AccountInfo,
        decisions: data.decisions.map((d) => ({
          id: String(d.id),
          symbol: d.symbol,
          name: d.name,
          side: d.side,
          price: d.price,
          quantity: d.quantity,
          confidence: d.confidence,
          reason: d.reason,
          createdAt: d.created_at,
        })),
        records: data.records.map((r) => ({
          id: String(r.id),
          symbol: r.symbol,
          name: r.name,
          side: r.side,
          price: r.price,
          quantity: r.quantity,
          profit: r.profit,
          profitPercent: r.profit_percent,
          createdAt: r.created_at,
        })),
        riskControl: {
          maxDailyLoss: data.risk_control.max_daily_loss,
          currentDailyLoss: data.risk_control.current_daily_loss,
          maxPositionRatio: data.risk_control.max_position_ratio,
          currentPositionRatio: data.risk_control.current_position_ratio,
          maxSingleStockRatio: data.risk_control.max_single_stock_ratio,
          status: data.risk_control.status,
        },
      } as SimStatus;
    },
    enabled: activeTab === 'sim',
    refetchInterval: 15000,
  });

  const toggleSimMutation = useMutation({
    mutationFn: () => {
      // 根据当前状态切换 start/stop
      const isRunning = simStatus?.running;
      return isRunning
        ? api.post('/trading/sim/stop')
        : api.post('/trading/sim/start');
    },
    onSuccess: () => {
      // 启停成功后立即拉取最新状态，避免等待 15s 轮询才更新「运行中/已停止」显示
      refetchSimStatus();
    },
  });

  const handleToggleSim = useCallback(async () => {
    await toggleSimMutation.mutateAsync();
  }, [toggleSimMutation]);

  // ========== 渲染 ==========

  return (
    <div className="trading-panel">
      <div className="trading-panel-header">
        <h1>交易面板</h1>
        <p className="page-description">提交买卖订单，管理持仓和查看成交记录</p>
      </div>

      <Tabs
        activeKey={activeTab}
        onChange={setActiveTab}
        items={TAB_ITEMS}
        className="trading-panel-tabs"
      />

      {activeTab === 'real' && (
        <div className="trading-panel-real">
          <div className="trading-panel-left">
            <OrderForm
              isReal={true}
              onSubmit={handlePlaceOrder}
            />
            <AccountOverview
              account={account || null}
              loading={accountLoading}
            />
          </div>
          <div className="trading-panel-right">
            <OrderList
              orders={orders || []}
              loading={ordersLoading}
              onCancelOrder={handleCancelOrder}
            />
            <PositionList
              positions={positions || []}
              loading={positionsLoading}
            />
          </div>
        </div>
      )}

      {activeTab === 'sim' && (
        <div className="trading-panel-sim">
          {simLoading && !simStatus ? (
            <div className="trading-panel-sim-loading">
              <Spin size="large" tip="加载模拟交易数据..." />
            </div>
          ) : (
            <SimTradePanel
              simStatus={simStatus || null}
              loading={simLoading}
              onToggle={handleToggleSim}
            />
          )}
        </div>
      )}
    </div>
  );
}