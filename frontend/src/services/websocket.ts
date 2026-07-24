import type { WsMessage, WsSubscribePayload } from '@/types';

type MessageHandler = (message: WsMessage) => void;
type EventType = 'open' | 'close' | 'error' | 'message';

class WebSocketClient {
  private ws: WebSocket | null = null;
  private url: string;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = Infinity;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null;
  private pingTimer: ReturnType<typeof setTimeout> | null = null;
  private handlers: Map<EventType, Set<MessageHandler>> = new Map();
  private subscribedChannels: Set<string> = new Set();
  private intentionalClose = false;
  private isConnected = false;

  constructor(url: string) {
    this.url = url;
  }

  connect(): void {
    if (this.ws?.readyState === WebSocket.OPEN) return;

    this.intentionalClose = false;
    this.ws = new WebSocket(this.url);

    this.ws.onopen = () => {
      this.isConnected = true;
      this.reconnectAttempts = 0;
      this.startHeartbeat();
      this.emit('open', {
        channel: 'system',
        type: 'data',
        data: { status: 'connected' },
        timestamp: Date.now(),
      });

      // 重新订阅之前的频道
      for (const channel of this.subscribedChannels) {
        this.sendSubscribe(channel);
      }
    };

    this.ws.onmessage = (event: MessageEvent) => {
      try {
        const message: WsMessage = JSON.parse(event.data as string);

        // 处理心跳响应
        if (message.type === 'heartbeat') {
          return;
        }

        this.emit('message', message);
      } catch {
        // 忽略解析错误
      }
    };

    this.ws.onerror = () => {
      this.emit('error', {
        channel: 'system',
        type: 'error',
        data: { message: 'WebSocket 连接错误' },
        timestamp: Date.now(),
      });
    };

    this.ws.onclose = () => {
      this.isConnected = false;
      this.stopHeartbeat();
      this.emit('close', {
        channel: 'system',
        type: 'data',
        data: { status: 'disconnected' },
        timestamp: Date.now(),
      });

      if (!this.intentionalClose) {
        this.scheduleReconnect();
      }
    };
  }

  disconnect(): void {
    this.intentionalClose = true;
    this.stopHeartbeat();
    this.cancelReconnect();
    this.subscribedChannels.clear();
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this.isConnected = false;
  }

  subscribe(channel: string): void {
    this.subscribedChannels.add(channel);
    if (this.isConnected && this.ws?.readyState === WebSocket.OPEN) {
      this.sendSubscribe(channel);
    }
  }

  unsubscribe(channel: string): void {
    this.subscribedChannels.delete(channel);
    if (this.isConnected && this.ws?.readyState === WebSocket.OPEN) {
      const payload: WsSubscribePayload = { channel };
      this.send({ type: 'unsubscribe', ...payload });
    }
  }

  on(event: EventType, handler: MessageHandler): () => void {
    if (!this.handlers.has(event)) {
      this.handlers.set(event, new Set());
    }
    this.handlers.get(event)!.add(handler);

    return () => {
      this.handlers.get(event)?.delete(handler);
    };
  }

  get connectionState(): number {
    return this.ws?.readyState ?? WebSocket.CLOSED;
  }

  private sendSubscribe(channel: string): void {
    const payload: WsSubscribePayload = { channel };
    this.send({ type: 'subscribe', ...payload });
  }

  private send(data: Record<string, unknown>): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(data));
    }
  }

  private emit(event: EventType, message: WsMessage): void {
    this.handlers.get(event)?.forEach((handler) => {
      try {
        handler(message);
      } catch {
        // 忽略 handler 内部错误
      }
    });
  }

  private startHeartbeat(): void {
    this.stopHeartbeat();

    // 每 30 秒发送心跳
    this.heartbeatTimer = setInterval(() => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        this.ws.send(JSON.stringify({ type: 'heartbeat', timestamp: Date.now() }));
      }
    }, 30000);

    // 心跳超时检测（10 秒内没收到任何响应则重连）
    this.resetPingTimer();
  }

  private resetPingTimer(): void {
    if (this.pingTimer) {
      clearTimeout(this.pingTimer);
    }
    this.pingTimer = setTimeout(() => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        this.ws.close();
      }
    }, 40000);
  }

  private stopHeartbeat(): void {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = null;
    }
    if (this.pingTimer) {
      clearTimeout(this.pingTimer);
      this.pingTimer = null;
    }
  }

  private scheduleReconnect(): void {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) return;

    // 指数退避，最大 30 秒
    const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30000);
    this.reconnectAttempts++;

    this.reconnectTimer = setTimeout(() => {
      this.connect();
    }, delay);
  }

  private cancelReconnect(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.reconnectAttempts = 0;
  }
}

// 创建单例
const wsClient = new WebSocketClient(
  `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/ws`
);

export default wsClient;