import { useState, useEffect, useCallback, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { Input, Select, Spin, Table } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { SearchOutlined, CaretUpOutlined, CaretDownOutlined, MinusOutlined } from '@ant-design/icons';
import useStockQuote from '@/hooks/useStockQuote';
import api from '@/services/api';
import type { Stock } from '@/types';
import '@/pages/Market.css';

interface StockRow {
  symbol: string;
  name: string;
  exchange: string;
  price: number;
  change: number;
  changePercent: number;
  volume: number;
  amount: number;
  high: number;
  low: number;
  open: number;
  preClose: number;
}

export default function Market() {
  const navigate = useNavigate();
  const [searchText, setSearchText] = useState('');
  const [exchangeFilter, setExchangeFilter] = useState<string>('all');
  const [stockList, setStockList] = useState<Stock[]>([]);
  const [loading, setLoading] = useState(true);
  const [flashingCells, setFlashingCells] = useState<Set<string>>(new Set());

  const symbols = useMemo(() => stockList.map((s) => s.symbol), [stockList]);
  const { stocks, changedSymbols } = useStockQuote(symbols);

  useEffect(() => {
    api.get<Stock[]>('/stocks')
      .then((data) => setStockList(data))
      .catch(() => {
        const mockStocks: Stock[] = [
          { symbol: '600519.SH', name: '贵州茅台', exchange: '上交所', industry: '白酒', marketCap: 2.35e12, listingDate: '2001-08-27', description: '' },
          { symbol: '000858.SZ', name: '五粮液', exchange: '深交所', industry: '白酒', marketCap: 8.2e11, listingDate: '1998-04-27', description: '' },
          { symbol: '300750.SZ', name: '宁德时代', exchange: '深交所', industry: '电池', marketCap: 9.8e11, listingDate: '2018-06-11', description: '' },
          { symbol: '601318.SH', name: '中国平安', exchange: '上交所', industry: '保险', marketCap: 7.5e11, listingDate: '2007-03-01', description: '' },
          { symbol: '000333.SZ', name: '美的集团', exchange: '深交所', industry: '家电', marketCap: 4.2e11, listingDate: '2013-09-18', description: '' },
          { symbol: '600036.SH', name: '招商银行', exchange: '上交所', industry: '银行', marketCap: 8.9e11, listingDate: '2002-04-09', description: '' },
          { symbol: '002415.SZ', name: '海康威视', exchange: '深交所', industry: '安防', marketCap: 3.1e11, listingDate: '2010-05-28', description: '' },
          { symbol: '600276.SH', name: '恒瑞医药', exchange: '上交所', industry: '医药', marketCap: 2.8e11, listingDate: '2000-10-18', description: '' },
          { symbol: '000651.SZ', name: '格力电器', exchange: '深交所', industry: '家电', marketCap: 2.1e11, listingDate: '1996-11-18', description: '' },
          { symbol: '601888.SH', name: '中国中免', exchange: '上交所', industry: '旅游', marketCap: 1.9e11, listingDate: '2009-10-15', description: '' },
          { symbol: '002594.SZ', name: '比亚迪', exchange: '深交所', industry: '汽车', marketCap: 6.8e11, listingDate: '2011-06-30', description: '' },
          { symbol: '600900.SH', name: '长江电力', exchange: '上交所', industry: '电力', marketCap: 5.2e11, listingDate: '2003-11-18', description: '' },
          { symbol: '000001.SZ', name: '平安银行', exchange: '深交所', industry: '银行', marketCap: 2.5e11, listingDate: '1991-04-03', description: '' },
          { symbol: '600030.SH', name: '中信证券', exchange: '上交所', industry: '证券', marketCap: 3.3e11, listingDate: '2003-01-06', description: '' },
          { symbol: '300059.SZ', name: '东方财富', exchange: '深交所', industry: '互联网', marketCap: 2.7e11, listingDate: '2010-03-19', description: '' },
        ];
        setStockList(mockStocks);
      })
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    if (changedSymbols.size === 0) return;
    setFlashingCells(new Set(changedSymbols));
    const timer = setTimeout(() => setFlashingCells(new Set()), 500);
    return () => clearTimeout(timer);
  }, [changedSymbols]);

  const handleRowClick = useCallback(
    (record: StockRow) => {
      navigate(`/stocks/${record.symbol}`);
    },
    [navigate]
  );

  const dataSource: StockRow[] = useMemo(() => {
    return stockList
      .filter((s) => {
        if (exchangeFilter !== 'all' && s.exchange !== exchangeFilter) return false;
        if (searchText) {
          const lower = searchText.toLowerCase();
          return (
            s.symbol.toLowerCase().includes(lower) ||
            s.name.toLowerCase().includes(lower) ||
            s.symbol.includes(searchText)
          );
        }
        return true;
      })
      .map((s) => {
        const quote = stocks.get(s.symbol);
        return {
          symbol: s.symbol,
          name: s.name,
          exchange: s.exchange,
          price: quote?.price ?? 0,
          change: quote?.change ?? 0,
          changePercent: quote?.changePercent ?? 0,
          volume: quote?.volume ?? 0,
          amount: quote?.amount ?? 0,
          high: quote?.high ?? 0,
          low: quote?.low ?? 0,
          open: quote?.open ?? 0,
          preClose: quote?.preClose ?? 0,
        };
      });
  }, [stockList, stocks, exchangeFilter, searchText]);

  const formatVolume = (vol: number) => {
    if (!vol || vol === 0) return '--';
    if (vol >= 1e8) return `${(vol / 1e8).toFixed(2)}亿`;
    if (vol >= 1e4) return `${(vol / 1e4).toFixed(2)}万`;
    return vol.toString();
  };

  const formatAmount = (amt: number) => {
    if (!amt || amt === 0) return '--';
    if (amt >= 1e8) return `${(amt / 1e8).toFixed(2)}亿`;
    if (amt >= 1e4) return `${(amt / 1e4).toFixed(2)}万`;
    return amt.toFixed(0);
  };

  const renderChange = (val: number, record: StockRow) => {
    const isFlashing = flashingCells.has(record.symbol);
    if (val === 0 && record.price === 0) return <span className="cell-na">--</span>;

    const isUp = val > 0;
    const isDown = val < 0;
    const cls = `cell-change ${isUp ? 'up' : isDown ? 'down' : 'flat'} ${isFlashing ? 'cell-flash' : ''}`;
    const icon = isUp ? <CaretUpOutlined /> : isDown ? <CaretDownOutlined /> : <MinusOutlined />;

    return (
      <span className={cls}>
        {icon}
        {val > 0 ? '+' : ''}{val.toFixed(2)}
      </span>
    );
  };

  const renderPercent = (val: number, record: StockRow) => {
    const isFlashing = flashingCells.has(record.symbol);
    if (val === 0 && record.price === 0) return <span className="cell-na">--</span>;

    const isUp = val > 0;
    const isDown = val < 0;
    const cls = `cell-change ${isUp ? 'up' : isDown ? 'down' : 'flat'} ${isFlashing ? 'cell-flash' : ''}`;

    return (
      <span className={cls}>
        {val > 0 ? '+' : ''}{val.toFixed(2)}%
      </span>
    );
  };

  const renderPrice = (price: number, record: StockRow) => {
    const isFlashing = flashingCells.has(record.symbol);
    if (!price || price === 0) return <span className="cell-na">--</span>;

    const change = record.change;
    const cls = change > 0 ? 'up' : change < 0 ? 'down' : 'flat';

    return (
      <span className={`cell-price ${cls} ${isFlashing ? 'cell-flash' : ''}`}>
        {price.toFixed(2)}
      </span>
    );
  };

  const columns: ColumnsType<StockRow> = [
    {
      title: '代码',
      dataIndex: 'symbol',
      key: 'symbol',
      width: 120,
      render: (text: string) => <span className="cell-symbol">{text}</span>,
    },
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      width: 120,
      render: (text: string) => <span className="cell-name">{text}</span>,
    },
    {
      title: '市场',
      dataIndex: 'exchange',
      key: 'exchange',
      width: 80,
      render: (text: string) => <span className="cell-exchange">{text}</span>,
    },
    {
      title: '最新价',
      dataIndex: 'price',
      key: 'price',
      width: 100,
      align: 'right',
      render: renderPrice,
      sorter: (a, b) => a.price - b.price,
    },
    {
      title: '涨跌幅',
      dataIndex: 'changePercent',
      key: 'changePercent',
      width: 100,
      align: 'right',
      render: renderPercent,
      sorter: (a, b) => a.changePercent - b.changePercent,
    },
    {
      title: '涨跌额',
      dataIndex: 'change',
      key: 'change',
      width: 100,
      align: 'right',
      render: renderChange,
      sorter: (a, b) => a.change - b.change,
    },
    {
      title: '成交量',
      dataIndex: 'volume',
      key: 'volume',
      width: 100,
      align: 'right',
      render: (val: number) => <span className="cell-volume">{formatVolume(val)}</span>,
      sorter: (a, b) => a.volume - b.volume,
    },
    {
      title: '成交额',
      dataIndex: 'amount',
      key: 'amount',
      width: 100,
      align: 'right',
      render: (val: number) => <span className="cell-amount">{formatAmount(val)}</span>,
      sorter: (a, b) => a.amount - b.amount,
    },
    {
      title: '最高',
      dataIndex: 'high',
      key: 'high',
      width: 90,
      align: 'right',
      render: (val: number) =>
        val ? <span className="cell-high">{val.toFixed(2)}</span> : <span className="cell-na">--</span>,
    },
    {
      title: '最低',
      dataIndex: 'low',
      key: 'low',
      width: 90,
      align: 'right',
      render: (val: number) =>
        val ? <span className="cell-low">{val.toFixed(2)}</span> : <span className="cell-na">--</span>,
    },
  ];

  const exchanges = useMemo(() => {
    const set = new Set(stockList.map((s) => s.exchange));
    return [...set].filter(Boolean);
  }, [stockList]);

  return (
    <div className="market-page">
      <div className="market-page-header">
        <h1 className="market-page-title">实时行情</h1>
        <div className="market-page-filters">
          <Input
            placeholder="搜索代码/名称"
            prefix={<SearchOutlined />}
            value={searchText}
            onChange={(e) => setSearchText(e.target.value)}
            className="market-search-input"
            allowClear
          />
          <Select
            value={exchangeFilter}
            onChange={setExchangeFilter}
            className="market-exchange-select"
            options={[
              { value: 'all', label: '全部市场' },
              ...exchanges.map((ex) => ({ value: ex, label: ex })),
            ]}
          />
        </div>
      </div>

      <div className="market-page-table">
        {loading ? (
          <div className="market-page-loading">
            <Spin size="large" />
            <span>加载行情数据...</span>
          </div>
        ) : (
          <Table<StockRow>
            columns={columns}
            dataSource={dataSource}
            rowKey="symbol"
            size="small"
            pagination={{ pageSize: 50, showSizeChanger: true, showTotal: (total) => `共 ${total} 只` }}
            onRow={(record) => ({
              onClick: () => handleRowClick(record),
              style: { cursor: 'pointer' },
            })}
            scroll={{ x: 1100 }}
            locale={{ emptyText: '暂无数据' }}
          />
        )}
      </div>
    </div>
  );
}