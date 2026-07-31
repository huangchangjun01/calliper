import { useEffect, useRef, useCallback } from 'react';
import type { StockQuote, WsMessage } from '@/types';
import useWebSocket from '@/hooks/useWebSocket';
import api from '@/services/api';

interface UseStockQuoteReturn {
  stocks: Map<string, StockQuote>;
  previousStocks: Map<string, StockQuote>;
  changedSymbols: Set<string>;
}

// 将后端 MarketData 格式映射为前端 StockQuote 格式
function mapMarketData(data: Record<string, unknown>): StockQuote {
  const ts = data.timestamp as string;
  return {
    symbol: (data.symbol as string) ?? '',
    name: (data.name as string) ?? '',
    price: (data.price as number) ?? 0,
    open: (data.open as number) ?? 0,
    high: (data.high as number) ?? 0,
    low: (data.low as number) ?? 0,
    preClose: (data.pre_close as number) ?? 0,
    volume: (data.volume as number) ?? 0,
    amount: (data.amount as number) ?? 0,
    change: (data.change as number) ?? 0,
    changePercent: (data.change_percent as number) ?? 0,
    timestamp: ts ? new Date(ts).getTime() : Date.now(),
  };
}

export default function useStockQuote(
  symbols: string[]
): UseStockQuoteReturn {
  const stocksRef = useRef<Map<string, StockQuote>>(new Map());
  const previousStocksRef = useRef<Map<string, StockQuote>>(new Map());
  const changedSymbolsRef = useRef<Set<string>>(new Set());

  const channels = symbols.map((s) => `stock:${s}`);

  const handleMessage = useCallback((message: WsMessage) => {
    if (message.type !== 'quote' || !message.data) return;

    const quote = message.data as StockQuote;
    if (!quote?.symbol) return;

    const prev = stocksRef.current.get(quote.symbol);
    if (prev) {
      previousStocksRef.current.set(quote.symbol, prev);
    }

    stocksRef.current.set(quote.symbol, quote);

    changedSymbolsRef.current = new Set(changedSymbolsRef.current);
    changedSymbolsRef.current.add(quote.symbol);

    setTimeout(() => {
      changedSymbolsRef.current = new Set(
        [...changedSymbolsRef.current].filter((s) => s !== quote.symbol)
      );
    }, 500);
  }, []);

  useWebSocket(channels, {
    autoConnect: true,
    onMessage: handleMessage,
  });

  // REST API 回退：定时拉取行情数据（分批，每批最多 50 只）
  useEffect(() => {
    if (symbols.length === 0) return;

    const BATCH_SIZE = 50;

    const fetchQuotes = async () => {
      try {
        // 分批请求
        for (let i = 0; i < symbols.length; i += BATCH_SIZE) {
          const batch = symbols.slice(i, i + BATCH_SIZE);
          const data = await api.post<{ count: number; data: Record<string, unknown>[] }>(
            '/market/realtime/batch',
            { symbols: batch }
          );
          if (data?.data) {
            for (const item of data.data) {
              const quote = mapMarketData(item);
              const prev = stocksRef.current.get(quote.symbol);
              if (prev) {
                previousStocksRef.current.set(quote.symbol, prev);
              }
              stocksRef.current.set(quote.symbol, quote);
              changedSymbolsRef.current = new Set(changedSymbolsRef.current);
              changedSymbolsRef.current.add(quote.symbol);
            }
          }
        }
      } catch {
        // 静默失败
      }
    };

    fetchQuotes(); // 立即拉取一次
    const interval = setInterval(fetchQuotes, 10000); // 每 10 秒拉取
    return () => clearInterval(interval);
  }, [symbols.join(',')]);

  useEffect(() => {
    return () => {
      stocksRef.current.clear();
      previousStocksRef.current.clear();
      changedSymbolsRef.current.clear();
    };
  }, []);

  return {
    get stocks() {
      return new Map(stocksRef.current);
    },
    get previousStocks() {
      return new Map(previousStocksRef.current);
    },
    get changedSymbols() {
      return new Set(changedSymbolsRef.current);
    },
  };
}