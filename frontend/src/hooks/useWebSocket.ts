import { useEffect, useRef, useCallback, useState } from 'react';
import type { WsMessage } from '@/types';
import wsClient from '@/services/websocket';

interface UseWebSocketOptions {
  onMessage?: (message: WsMessage) => void;
  autoConnect?: boolean;
  /** 暂停接收实时消息：暂停期间不更新 React 状态，也不触发 onMessage，用于「暂停刷新」场景 */
  paused?: boolean;
}

interface UseWebSocketReturn {
  messages: WsMessage[];
  latestMessage: WsMessage | null;
  isConnected: boolean;
  subscribe: (channel: string) => void;
  unsubscribe: (channel: string) => void;
}

/** 保留最近的消息条数 */
const MAX_MESSAGES = 100;

export default function useWebSocket(
  channels: string[],
  options: UseWebSocketOptions = {}
): UseWebSocketReturn {
  const { onMessage, autoConnect = true } = options;
  const [messages, setMessages] = useState<WsMessage[]>([]);
  const [latestMessage, setLatestMessage] = useState<WsMessage | null>(null);
  const [isConnected, setIsConnected] = useState(false);
  const subscribedRef = useRef<Set<string>>(new Set());
  const onMessageRef = useRef(onMessage);
  onMessageRef.current = onMessage;

  // 暂停刷新：暂停期间忽略实时消息，不触发任何 setState / onMessage
  const pausedRef = useRef(!!options.paused);
  pausedRef.current = !!options.paused;

  useEffect(() => {
    if (!autoConnect) return;

    const handleOpen = () => {
      setIsConnected(true);
    };

    const handleClose = () => {
      setIsConnected(false);
    };

    // 批量合并：将一帧内到达的多条消息合并为一次 React 状态更新
    const pendingRef = { current: [] as WsMessage[] };
    let rafId: number | null = null;

    const flush = () => {
      rafId = null;
      const batch = pendingRef.current;
      pendingRef.current = [];
      if (batch.length === 0) return;
      setMessages((prev) => {
        if (batch.length >= MAX_MESSAGES) return batch.slice(-MAX_MESSAGES);
        return [...prev.slice(-(MAX_MESSAGES - batch.length)), ...batch];
      });
      setLatestMessage(batch[batch.length - 1]);
    };

    const handleMessage = (message: WsMessage) => {
      if (pausedRef.current) return; // 暂停期间丢弃实时消息（恢复时做一次全量同步）
      onMessageRef.current?.(message);
      pendingRef.current.push(message);
      if (rafId === null) {
        rafId = requestAnimationFrame(flush);
      }
    };

    const unsubOpen = wsClient.on('open', handleOpen);
    const unsubClose = wsClient.on('close', handleClose);
    const unsubMessage = wsClient.on('message', handleMessage);

    wsClient.connect();

    return () => {
      if (rafId !== null) cancelAnimationFrame(rafId);
      unsubOpen();
      unsubClose();
      unsubMessage();
    };
  }, [autoConnect]);

  const subscribe = useCallback((channel: string) => {
    if (!subscribedRef.current.has(channel)) {
      subscribedRef.current.add(channel);
      wsClient.subscribe(channel);
    }
  }, []);

  const unsubscribe = useCallback((channel: string) => {
    if (subscribedRef.current.has(channel)) {
      subscribedRef.current.delete(channel);
      wsClient.unsubscribe(channel);
    }
  }, []);

  useEffect(() => {
    if (!autoConnect) return;

    for (const channel of channels) {
      subscribe(channel);
    }

    return () => {
      for (const channel of channels) {
        unsubscribe(channel);
      }
    };
  }, [channels.join(','), autoConnect, subscribe, unsubscribe]);

  return {
    messages,
    latestMessage,
    isConnected,
    subscribe,
    unsubscribe,
  };
}