import { useState, useCallback, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Pagination } from 'antd';
import type { StockSearchItem, PaginatedData } from '@/types';
import api from '@/services/api';
import useStockQuote from '@/hooks/useStockQuote';
import MarketTabs from '@/components/MarketTabs';
import SearchBox from '@/components/SearchBox';
import StockTable from '@/components/StockTable';
import styles from './StockSearch.module.css';

const PAGE_SIZE = 20;

async function fetchStocks(params: {
  keyword: string;
  exchange: string;
  page: number;
  pageSize: number;
}): Promise<PaginatedData<StockSearchItem>> {
  const data = await api.get<{ stocks: Record<string, unknown>[]; total: number; limit: number; offset: number }>('/stocks/search', {
    q: params.keyword || undefined,
    market: params.exchange || undefined,
    limit: params.pageSize,
    offset: (params.page - 1) * params.pageSize,
  });

  // Map backend snake_case to frontend camelCase
  // 注意：price / changePercent 不在 /stocks/search 返回中，由 useStockQuote 提供
  const items: StockSearchItem[] = (data.stocks ?? []).map((s: Record<string, unknown>) => ({
    symbol: (s.symbol as string) ?? '',
    name: (s.name as string) ?? '',
    exchange: (s.exchange as string) ?? '',
    industry: (s.industry as string) ?? '',
    marketCap: (s.market_cap as number) ?? 0,
    price: 0,
    changePercent: 0,
  }));

  return {
    items,
    total: data.total ?? 0,
    page: params.page,
    pageSize: params.pageSize,
  };
}

export default function StockSearch() {
  const [keyword, setKeyword] = useState('');
  const [market, setMarket] = useState('');
  const [page, setPage] = useState(1);

  const { data, isLoading } = useQuery({
    queryKey: ['stocks', 'search', keyword, market, page],
    queryFn: () =>
      fetchStocks({
        keyword,
        exchange: market,
        page,
        pageSize: PAGE_SIZE,
      }),
    placeholderData: (prev) => prev,
  });

  const baseStocks = data?.items ?? [];
  const total = data?.total ?? 0;

  // 将股票列表的 symbols 传给 useStockQuote，获取实时行情（最新价、涨跌幅等）
  const symbols = useMemo(() => baseStocks.map((s) => s.symbol), [baseStocks]);
  const { stocks: quoteMap } = useStockQuote(symbols);

  // 合并实时行情数据：price / changePercent 取自 useStockQuote 返回的 stocks Map
  const stocks = useMemo(() => {
    return baseStocks.map((s) => {
      const quote = quoteMap.get(s.symbol);
      return {
        ...s,
        price: quote?.price ?? 0,
        changePercent: quote?.changePercent ?? 0,
      };
    });
  }, [baseStocks, quoteMap]);

  const handleMarketChange = useCallback((newMarket: string) => {
    setMarket(newMarket);
    setPage(1);
  }, []);

  const handleKeywordChange = useCallback((newKeyword: string) => {
    setKeyword(newKeyword);
    setPage(1);
  }, []);

  const handlePageChange = useCallback((newPage: number) => {
    setPage(newPage);
  }, []);

  return (
    <div className={styles.container}>
      <h1 className={styles.title}>股票检索</h1>
      <p className={styles.description}>搜索和筛选股票，查看实时行情与历史数据</p>

      <MarketTabs activeMarket={market} onChange={handleMarketChange} />

      <SearchBox
        value={keyword}
        onChange={handleKeywordChange}
        placeholder="输入股票代码或名称搜索..."
      />

      <StockTable
        stocks={stocks}
        loading={isLoading}
      />

      {total > PAGE_SIZE && (
        <div className={styles.pagination}>
          <Pagination
            current={page}
            pageSize={PAGE_SIZE}
            total={total}
            onChange={handlePageChange}
            showSizeChanger={false}
            showQuickJumper
            showTotal={(t) => `共 ${t} 条结果`}
          />
        </div>
      )}
    </div>
  );
}