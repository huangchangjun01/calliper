import { useState, useMemo } from 'react';
import { Table, Tag, Button, Radio, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import type { Order, OrderStatus } from '@/types';
import dayjs from 'dayjs';

interface OrderListProps {
  orders: Order[];
  loading: boolean;
  onCancelOrder?: (orderId: string) => Promise<void>;
}

const STATUS_MAP: Record<OrderStatus, { label: string; color: string }> = {
  pending: { label: '待成交', color: 'processing' },
  partial: { label: '部分成交', color: 'warning' },
  filled: { label: '已成交', color: 'success' },
  cancelled: { label: '已撤单', color: 'default' },
  rejected: { label: '已拒绝', color: 'error' },
};

const SIDE_MAP: Record<string, { label: string; className: string }> = {
  buy: { label: '买入', className: 'trade-buy' },
  sell: { label: '卖出', className: 'trade-sell' },
};

const TYPE_MAP: Record<string, string> = {
  market: '市价',
  limit: '限价',
};

const STATUS_FILTER_OPTIONS = [
  { label: '全部', value: 'all' },
  { label: '待成交', value: 'pending' },
  { label: '已成交', value: 'filled' },
  { label: '已撤单', value: 'cancelled' },
];

export default function OrderList({ orders, loading, onCancelOrder }: OrderListProps) {
  const [statusFilter, setStatusFilter] = useState('all');
  const [currentPage, setCurrentPage] = useState(1);
  const pageSize = 10;

  const filteredOrders = useMemo(() => {
    if (statusFilter === 'all') return orders;
    return orders.filter((o) => o.status === statusFilter);
  }, [orders, statusFilter]);

  const handleCancel = async (orderId: string) => {
    if (!onCancelOrder) return;
    try {
      await onCancelOrder(orderId);
      message.success('撤单成功');
    } catch {
      message.error('撤单失败');
    }
  };

  const columns: ColumnsType<Order> = [
    {
      title: '时间',
      dataIndex: 'createdAt',
      key: 'createdAt',
      width: 160,
      render: (val: string) => dayjs(val).format('MM-DD HH:mm:ss'),
    },
    {
      title: '股票',
      dataIndex: 'symbol',
      key: 'symbol',
      width: 120,
      render: (val: string) => <span className="order-symbol">{val}</span>,
    },
    {
      title: '方向',
      dataIndex: 'side',
      key: 'side',
      width: 80,
      render: (val: string) => {
        const s = SIDE_MAP[val] || { label: val, className: '' };
        return <span className={s.className}>{s.label}</span>;
      },
    },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      width: 80,
      render: (val: string) => TYPE_MAP[val] || val,
    },
    {
      title: '价格',
      dataIndex: 'price',
      key: 'price',
      width: 100,
      align: 'right',
      render: (val: number) => val?.toFixed(2) || '--',
    },
    {
      title: '数量',
      dataIndex: 'quantity',
      key: 'quantity',
      width: 100,
      align: 'right',
    },
    {
      title: '已成交',
      dataIndex: 'filledQuantity',
      key: 'filledQuantity',
      width: 100,
      align: 'right',
      render: (val: number) => val || 0,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (val: OrderStatus) => {
        const s = STATUS_MAP[val] || { label: val, color: 'default' };
        return <Tag color={s.color}>{s.label}</Tag>;
      },
    },
    {
      title: '操作',
      key: 'action',
      width: 80,
      render: (_, record) => {
        if (record.status === 'pending' || record.status === 'partial') {
          return (
            <Button
              type="link"
              danger
              size="small"
              onClick={() => handleCancel(record.id)}
            >
              撤单
            </Button>
          );
        }
        return null;
      },
    },
  ];

  return (
    <div className="order-list">
      <div className="order-list-header">
        <h3 className="order-list-title">订单列表</h3>
        <Radio.Group
          value={statusFilter}
          onChange={(e) => { setStatusFilter(e.target.value); setCurrentPage(1); }}
          optionType="button"
          buttonStyle="solid"
          size="small"
          options={STATUS_FILTER_OPTIONS}
        />
      </div>
      <Table
        columns={columns}
        dataSource={filteredOrders}
        rowKey="id"
        loading={loading}
        size="small"
        pagination={{
          current: currentPage,
          pageSize,
          total: filteredOrders.length,
          onChange: (page) => setCurrentPage(page),
          showSizeChanger: false,
          showTotal: (total) => `共 ${total} 条`,
        }}
        scroll={{ x: 820 }}
        locale={{ emptyText: '暂无订单' }}
      />
    </div>
  );
}