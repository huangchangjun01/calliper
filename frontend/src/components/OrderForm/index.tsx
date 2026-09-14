import { useState, useEffect, useCallback } from 'react';
import { Input, Radio, Button, Modal, message, Select, Spin } from 'antd';
import { SearchOutlined } from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';
import api from '@/services/api';
import type { StockSearchItem } from '@/types';

/** 下单请求体（对齐后端 /trading/orders 接口字段） */
export interface OrderRequestBody {
  symbol: string;
  action: 'buy' | 'sell';
  order_type: 'market' | 'limit';
  price?: string;
  quantity: number;
  is_real: boolean;
  trade_password?: string;
}

interface OrderFormProps {
  isReal: boolean;
  onSubmit: (order: OrderRequestBody) => Promise<void>;
}

const ORDER_SIDE_OPTIONS = [
  { label: '买入', value: 'buy' },
  { label: '卖出', value: 'sell' },
];

const ORDER_TYPE_OPTIONS = [
  { label: '限价', value: 'limit' },
  { label: '市价', value: 'market' },
];

export default function OrderForm({ isReal, onSubmit }: OrderFormProps) {
  const [symbol, setSymbol] = useState('');
  const [searchInput, setSearchInput] = useState('');
  const [side, setSide] = useState<'buy' | 'sell'>('buy');
  const [orderType, setOrderType] = useState<'limit' | 'market'>('limit');
  const [price, setPrice] = useState<string>('');
  const [quantity, setQuantity] = useState<string>('');
  const [password, setPassword] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [searchOpen, setSearchOpen] = useState(false);

  const { data: searchResults, isFetching: searchLoading } = useQuery({
    queryKey: ['stockSearch', symbol],
    queryFn: async () => {
      if (!symbol || symbol.length < 1) return [];
      const data = await api.get<{ stocks: StockSearchItem[] }>('/stocks/search', { q: symbol });
      return data.stocks;
    },
    enabled: symbol.length >= 1,
    staleTime: 60000,
  });

  // 股票搜索防抖：输入 300ms 后再触发查询
  useEffect(() => {
    const t = setTimeout(() => setSymbol(searchInput), 300);
    return () => clearTimeout(t);
  }, [searchInput]);

  const handleSymbolSelect = useCallback(async (value: string) => {
    setSymbol(value);
    setSearchInput(value);
    setSearchOpen(false);

    // 选股后按 symbol 调行情接口回填实时价；取不到则留空由用户手输
    try {
      const data = await api.post<{ count: number; data: Record<string, unknown>[] }>(
        '/market/realtime/batch',
        { symbols: [value] }
      );
      const quote = data?.data?.[0];
      const priceVal = quote?.price as number | undefined;
      setPrice(priceVal && priceVal > 0 ? priceVal.toFixed(2) : '');
    } catch {
      setPrice('');
    }
  }, []);

  const validate = (): string | null => {
    if (!symbol.trim()) return '请输入股票代码';
    if (!quantity || Number(quantity) <= 0) return '请输入有效的数量';
    if (orderType === 'limit' && (!price || Number(price) <= 0)) return '请输入有效的价格';
    if (isReal && !password) return '请输入交易密码';
    return null;
  };

  const handleSubmit = async () => {
    if (submitting) return; // 防重复提交：请求进行中时忽略再次点击
    const error = validate();
    if (error) {
      message.warning(error);
      return;
    }

    Modal.confirm({
      title: '确认下单',
      content: (
        <div style={{ lineHeight: 2 }}>
          <p>股票：{symbol}</p>
          <p>方向：{side === 'buy' ? '买入' : '卖出'}</p>
          <p>类型：{orderType === 'limit' ? '限价' : '市价'}</p>
          {orderType === 'limit' && <p>价格：{price}</p>}
          <p>数量：{quantity}</p>
          {orderType === 'limit' && <p>预计金额：{(Number(price) * Number(quantity)).toFixed(2)}</p>}
        </div>
      ),
      okText: '确认提交',
      cancelText: '取消',
      onOk: async () => {
        if (submitting) return;
        setSubmitting(true);
        try {
          const order: OrderRequestBody = {
            symbol: symbol.trim(),
            action: side,
            order_type: orderType,
            quantity: Number(quantity),
            is_real: isReal,
          };
          if (orderType === 'limit') {
            order.price = String(price);
          }
          if (isReal) {
            order.trade_password = password;
          }
          await onSubmit(order);
          message.success('下单成功');
          handleReset();
        } catch {
          message.error('下单失败，请重试');
        } finally {
          setSubmitting(false);
        }
      },
    });
  };

  const handleReset = () => {
    setSymbol('');
    setSearchInput('');
    setPrice('');
    setQuantity('');
    setPassword('');
  };

  const estimatedAmount = orderType === 'limit' && price && quantity
    ? (Number(price) * Number(quantity)).toFixed(2)
    : '--';

  return (
    <div className="order-form">
      <h3 className="order-form-title">下单面板</h3>

      <div className="order-form-field">
        <label className="order-form-label">股票代码</label>
        <Select
          showSearch
          value={symbol || undefined}
          placeholder="输入股票代码或名称搜索"
          filterOption={false}
          onSearch={(val) => { setSearchInput(val); setSearchOpen(true); }}
          onSelect={handleSymbolSelect}
          onBlur={() => setTimeout(() => setSearchOpen(false), 200)}
          onFocus={() => { if (searchResults && searchResults.length > 0) setSearchOpen(true); }}
          open={searchOpen}
          notFoundContent={searchLoading ? <Spin size="small" /> : '无匹配结果'}
          suffixIcon={<SearchOutlined />}
          style={{ width: '100%' }}
          options={(searchResults || []).map((item) => ({
            value: item.symbol,
            label: (
              <div className="order-form-search-option">
                <span className="order-form-search-symbol">{item.symbol}</span>
                <span className="order-form-search-name">{item.name}</span>
                <span className="order-form-search-price">{item.price?.toFixed(2)}</span>
              </div>
            ),
          }))}
        />
      </div>

      <div className="order-form-field">
        <label className="order-form-label">买卖方向</label>
        <Radio.Group
          value={side}
          onChange={(e) => setSide(e.target.value)}
          optionType="button"
          buttonStyle="solid"
          options={ORDER_SIDE_OPTIONS}
        />
      </div>

      <div className="order-form-field">
        <label className="order-form-label">订单类型</label>
        <Radio.Group
          value={orderType}
          onChange={(e) => setOrderType(e.target.value)}
          optionType="button"
          buttonStyle="solid"
          options={ORDER_TYPE_OPTIONS}
        />
      </div>

      {orderType === 'limit' && (
        <div className="order-form-field">
          <label className="order-form-label">价格</label>
          <Input
            value={price}
            onChange={(e) => setPrice(e.target.value)}
            placeholder="请输入限价"
            type="number"
            min={0}
            step={0.01}
          />
        </div>
      )}

      <div className="order-form-field">
        <label className="order-form-label">数量</label>
        <Input
          value={quantity}
          onChange={(e) => setQuantity(e.target.value)}
          placeholder="请输入数量（股）"
          type="number"
          min={100}
          step={100}
        />
      </div>

      <div className="order-form-field order-form-estimate">
        <span className="order-form-label">预计金额</span>
        <span className="order-form-estimate-value">¥ {estimatedAmount}</span>
      </div>

      {isReal && (
        <div className="order-form-field">
          <label className="order-form-label">交易密码</label>
          <Input.Password
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="请输入交易密码"
          />
        </div>
      )}

      <div className="order-form-actions">
        <Button
          type="primary"
          block
          loading={submitting}
          disabled={submitting}
          onClick={handleSubmit}
          danger={side === 'sell'}
        >
          {side === 'buy' ? '确认买入' : '确认卖出'}
        </Button>
        <Button block onClick={handleReset}>
          重置
        </Button>
      </div>
    </div>
  );
}