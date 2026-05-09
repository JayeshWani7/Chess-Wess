export type WSMessageType =
  | "move"
  | "player_joined"
  | "player_connected"
  | "player_disconnected"
  | "game_over"
  | "timer_update"
  | "error"
  | "pong";

export interface WSMessage<T = unknown> {
  type: WSMessageType;
  payload: T;
}

type MessageHandler = (msg: WSMessage) => void;

class ChessWSClient {
  private ws: WebSocket | null = null;
  private handlers: Set<MessageHandler> = new Set();
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private gameId: string | null = null;
  private token: string | null = null;
  private pingInterval: ReturnType<typeof setInterval> | null = null;

  connect(gameId: string, token: string) {
    this.gameId = gameId;
    this.token = token;
    this.openSocket();
  }

  private openSocket() {
    if (!this.gameId || !this.token) return;

    const base = import.meta.env.VITE_WS_URL ?? `ws://${window.location.host}`;
    const url = `${base}/ws?game_id=${this.gameId}&token=${this.token}`;

    this.ws = new WebSocket(url);

    this.ws.onopen = () => {
      console.log("[WS] connected to game", this.gameId);
      this.startPing();
    };

    this.ws.onmessage = (event) => {
      try {
        const msg: WSMessage = JSON.parse(event.data as string);
        this.handlers.forEach((h) => h(msg));
      } catch {
        console.warn("[WS] failed to parse message", event.data);
      }
    };

    this.ws.onclose = () => {
      console.log("[WS] disconnected — reconnecting in 2s");
      this.stopPing();
      this.reconnectTimer = setTimeout(() => this.openSocket(), 2000);
    };

    this.ws.onerror = (err) => {
      console.error("[WS] error", err);
    };
  }

  send(msg: WSMessage) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(msg));
    }
  }

  sendMove(uci: string, san: string, fen: string) {
    this.send({ type: "move", payload: { uci, san, fen } });
  }

  onMessage(handler: MessageHandler) {
    this.handlers.add(handler);
    return () => this.handlers.delete(handler);
  }

  disconnect() {
    this.stopPing();
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    this.ws?.close();
    this.ws = null;
    this.handlers.clear();
    this.gameId = null;
    this.token = null;
  }

  private startPing() {
    this.pingInterval = setInterval(() => {
      this.send({ type: "pong", payload: null });
    }, 20_000);
  }

  private stopPing() {
    if (this.pingInterval) {
      clearInterval(this.pingInterval);
      this.pingInterval = null;
    }
  }
}

// Singleton — one connection at a time
export const wsClient = new ChessWSClient();
