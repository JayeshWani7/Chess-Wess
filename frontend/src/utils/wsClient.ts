export type WSMessageType =
  | "move"
  | "rewind"
  | "timeline_created"
  | "timeline_renamed"
  | "timeline_switched"
  | "switch_timeline"
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

    this.ws.onmessage = async (event) => {
      try {
        if (typeof event.data === "string") {
          const msg: WSMessage = JSON.parse(event.data);
          this.handlers.forEach((h) => h(msg));
          return;
        }

        if (event.data instanceof Blob) {
          const text = await event.data.text();
          const msg: WSMessage = JSON.parse(text);
          this.handlers.forEach((h) => h(msg));
          return;
        }

        if (event.data instanceof ArrayBuffer) {
          const text = new TextDecoder().decode(event.data);
          const msg: WSMessage = JSON.parse(text);
          this.handlers.forEach((h) => h(msg));
          return;
        }

        if (event.data && typeof event.data === "object") {
          this.handlers.forEach((h) => h(event.data as WSMessage));
          return;
        }

        console.warn("[WS] unknown message payload", event.data);
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

  sendMove(uci: string, san: string, fen: string, timelineId?: string | null, parentNodeId?: string | null) {
    this.send({
      type: "move",
      payload: { uci, san, fen, timeline_id: timelineId ?? undefined, parent_node_id: parentNodeId ?? undefined },
    });
  }

  sendRewind(nodeId: string) {
    this.send({ type: "rewind", payload: { node_id: nodeId } });
  }

  switchTimeline(timelineId: string) {
    this.send({ type: "switch_timeline", payload: { timeline_id: timelineId } });
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
