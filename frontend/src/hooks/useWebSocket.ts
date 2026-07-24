import { useEffect, useRef, useCallback, useState } from 'react';
import type { WsMessage } from '@/types';
import wsClient from '@/services/websocket';

interface UseWebSocketOptions {
  onMessage?: (message: WsMessage) => void;
  autoConnect?: boolean;
}

interface UseWebSocketReturn {
  messages: WsMessage[];
  latestMessage: WsMessage | null;
  isConnected: boolean;
  subscribe: (channel: string) => void;
  unsubscribe: (channel: string) => void;
}

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

  useEffect(() => {
    if (!autoConnect) return;

    const handleOpen = () => {
      setIsConnected(true);
    };

    const handleClose = () => {
      setIsConnected(false);
    };

    const handleMessage = (message: WsMessage) => {
      setMessages((prev) => [...prev.slice(-99), message]);
      setLatestMessage(message);
      onMessageRef.current?.(message);
    };

    const unsubOpen = wsClient.on('open', handleOpen);
    const unsubClose = wsClient.on('close', handleClose);
    const unsubMessage = wsClient.on('message', handleMessage);

    wsClient.connect();

    return () => {
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