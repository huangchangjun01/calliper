import { useState, useCallback } from 'react';
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

export default function TradingPanel() {
  const [activeTab, setActiveTab] = useState('real');
  const queryClient = useQueryClient();

  // ========== 真实交易数据 ==========

  const { data: orders, isLoading: ordersLoading } = useQuery({
    queryKey: ['orders'],
    queryFn: async () => {
      const data = await api.get<{ orders: Order[]; total: number; limit: number; offset: number }>('/trading/orders');
      return data.orders;
    },
    enabled: activeTab === 'real',
    refetchInterval: 10000,
  });

  const { data: positions, isLoading: positionsLoading } = useQuery({
    queryKey: ['positions'],
    queryFn: async () => {
      const data = await api.get<{ positions: Position[] }>('/trading/positions');
      return data.positions;
    },
    enabled: activeTab === 'real',
    refetchInterval: 10000,
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
    refetchInterval: 10000,
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

  const { data: simStatus, isLoading: simLoading } = useQuery({
    queryKey: ['simStatus'],
    queryFn: () => api.get<SimStatus>('/trading/sim/status'),
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
      queryClient.invalidateQueries({ queryKey: ['simStatus'] });
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