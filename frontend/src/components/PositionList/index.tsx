import { useMemo } from 'react';
import { Table } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import type { Position } from '@/types';

interface PositionListProps {
  positions: Position[];
  loading: boolean;
}

export default function PositionList({ positions, loading }: PositionListProps) {
  const columns: ColumnsType<Position> = useMemo(() => [
    {
      title: '股票代码',
      dataIndex: 'symbol',
      key: 'symbol',
      width: 120,
      render: (val: string) => <span className="position-symbol">{val}</span>,
    },
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      width: 100,
    },
    {
      title: '持仓数量',
      dataIndex: 'quantity',
      key: 'quantity',
      width: 100,
      align: 'right',
      render: (val: number) => val.toLocaleString(),
    },
    {
      title: '成本价',
      dataIndex: 'avgCost',
      key: 'avgCost',
      width: 100,
      align: 'right',
      render: (val: number) => val?.toFixed(2),
    },
    {
      title: '现价',
      dataIndex: 'currentPrice',
      key: 'currentPrice',
      width: 100,
      align: 'right',
      render: (val: number) => val?.toFixed(2),
    },
    {
      title: '市值',
      dataIndex: 'marketValue',
      key: 'marketValue',
      width: 120,
      align: 'right',
      render: (val: number) => val?.toFixed(2) || '--',
    },
    {
      title: '盈亏',
      dataIndex: 'profit',
      key: 'profit',
      width: 120,
      align: 'right',
      render: (val: number) => {
        if (val === undefined || val === null) return '--';
        const className = val > 0 ? 'profit-up' : val < 0 ? 'profit-down' : '';
        return <span className={className}>{val > 0 ? '+' : ''}{val.toFixed(2)}</span>;
      },
    },
    {
      title: '盈亏比例',
      dataIndex: 'profitPercent',
      key: 'profitPercent',
      width: 100,
      align: 'right',
      render: (val: number) => {
        if (val === undefined || val === null) return '--';
        const className = val > 0 ? 'profit-up' : val < 0 ? 'profit-down' : '';
        return <span className={className}>{val > 0 ? '+' : ''}{val.toFixed(2)}%</span>;
      },
    },
  ], []);

  return (
    <div className="position-list">
      <div className="position-list-header">
        <h3 className="position-list-title">持仓列表</h3>
      </div>
      <Table
        columns={columns}
        dataSource={positions}
        rowKey="symbol"
        loading={loading}
        size="small"
        pagination={false}
        scroll={{ x: 860 }}
        locale={{ emptyText: '暂无持仓' }}
        summary={() => {
          if (positions.length === 0) return null;
          const totalMarketValue = positions.reduce((sum, p) => sum + (p.marketValue || 0), 0);
          const totalProfit = positions.reduce((sum, p) => sum + (p.profit || 0), 0);
          const totalProfitClass = totalProfit > 0 ? 'profit-up' : totalProfit < 0 ? 'profit-down' : '';

          return (
            <Table.Summary.Row>
              <Table.Summary.Cell index={0} colSpan={5}>
                <strong>合计</strong>
              </Table.Summary.Cell>
              <Table.Summary.Cell index={1} align="right">
                <strong>{totalMarketValue.toFixed(2)}</strong>
              </Table.Summary.Cell>
              <Table.Summary.Cell index={2} align="right">
                <strong className={totalProfitClass}>
                  {totalProfit > 0 ? '+' : ''}{totalProfit.toFixed(2)}
                </strong>
              </Table.Summary.Cell>
              <Table.Summary.Cell index={3} />
            </Table.Summary.Row>
          );
        }}
      />
    </div>
  );
}