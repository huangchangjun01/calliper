import { useEffect, useRef, useCallback } from 'react';
import type { StockQuote, WsMessage } from '@/types';
import useWebSocket from '@/hooks/useWebSocket';

interface UseStockQuoteReturn {
  stocks: Map<string, StockQuote>;
  previousStocks: Map<string, StockQuote>;
  changedSymbols: Set<string>;
}

export default function useStockQuote(
  symbols: string[]
): UseStockQuoteReturn {
  const stocksRef = useRef<Map<string, StockQuote>>(new Map());
  const previousStocksRef = useRef<Map<string, StockQuote>>(new Map());
  const changedSymbolsRef = useRef<Set<string>>(new Set());

  const channels = symbols.map((s) => `stock:${s}`);

  const handleMessage = useCallback((message: WsMessage) => {
    if (message.type !== 'data' || !message.data) return;

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