import { useState, useCallback } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Pagination } from 'antd';
import type { StockSearchItem, PaginatedData } from '@/types';
import api from '@/services/api';
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
  return api.get<PaginatedData<StockSearchItem>>('/stocks/search', {
    keyword: params.keyword || undefined,
    exchange: params.exchange || undefined,
    page: params.page,
    pageSize: params.pageSize,
  });
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

  const stocks = data?.items ?? [];
  const total = data?.total ?? 0;

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