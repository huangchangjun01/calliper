import { useEffect, useState } from 'react';
import wsClient from '@/services/websocket';

export type WsStatus = 'connecting' | 'connected' | 'reconnecting' | 'disconnected';

interface WsConnectionStatus {
  status: WsStatus;
  isConnected: boolean;
  isReconnecting: boolean;
}

function initialStatus(): WsStatus {
  const state = wsClient.connectionState;
  if (state === WebSocket.OPEN) return 'connected';
  if (state === WebSocket.CONNECTING) return 'connecting';
  return 'disconnected';
}

/**
 * 订阅 WebSocket 单例的连接状态，用于在布局头部展示「已连接 / 重连中 / 已断开」状态指示。
 */
export default function useWsConnectionStatus(): WsConnectionStatus {
  const [status, setStatus] = useState<WsStatus>(initialStatus);

  useEffect(() => {
    const handleOpen = () => setStatus('connected');
    const handleClose = () =>
      setStatus((prev) => (prev === 'connected' ? 'reconnecting' : 'disconnected'));
    const handleError = () =>
      setStatus((prev) => (prev === 'connected' ? 'reconnecting' : prev));

    const unsubOpen = wsClient.on('open', handleOpen);
    const unsubClose = wsClient.on('close', handleClose);
    const unsubError = wsClient.on('error', handleError);

    return () => {
      unsubOpen();
      unsubClose();
      unsubError();
    };
  }, []);

  return {
    status,
    isConnected: status === 'connected',
    isReconnecting: status === 'reconnecting',
  };
}