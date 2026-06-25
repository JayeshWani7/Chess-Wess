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
  | "timeline_merged"
  | "node_annotated"
  | "resync"
  | "pong";

export interface WSMessage<T = unknown> {
  type: WSMessageType;
  payload: T;
  seq?: number;
}

type MessageHandler = (msg: WSMessage) => void;

class ChessWSClient {
  private ws: WebSocket | null = null;
  private handlers: Set<MessageHandler> = new Set();
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private gameId: string | null = null;
  private token: string | null = null;
  private pingInterval: ReturnType<typeof setInterval> | null = null;
  private lastSeq: number = 0;

  connect(gameId: string, token: string) {
    this.gameId = gameId;
    this.token = token;
    this.lastSeq = 0; // reset on fresh connect
    this.openSocket();
  }

  private openSocket() {
    if (!this.gameId || !this.token) return;

    const base = import.meta.env.VITE_WS_URL ?? `ws://${window.location.host}`;
    let url = `${base}/ws?game_id=${this.gameId}&token=${this.token}`;
    if (this.lastSeq > 0) {
      url += `&last_seq=${this.lastSeq}`;
    }

    this.ws = new WebSocket(url);

    this.ws.onopen = () => {
      console.log("[WS] connected to game", this.gameId, "lastSeq:", this.lastSeq);
      this.startPing();
    };

    this.ws.onmessage = async (event) => {
      try {
        let msg: any = null;
        if (typeof event.data === "string") {
          msg = JSON.parse(event.data);
        } else if (event.data instanceof Blob) {
          const text = await event.data.text();
          msg = JSON.parse(text);
        } else if (event.data instanceof ArrayBuffer) {
          const text = new TextDecoder().decode(event.data);
          msg = JSON.parse(text);
        } else if (event.data && typeof event.data === "object") {
          msg = event.data;
        }

        if (msg) {
          this.handleIncomingMessage(msg);
        } else {
          console.warn("[WS] unknown message payload", event.data);
        }
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

  private handleIncomingMessage(msg: any) {
    if (msg && typeof msg === "object") {
      const seq = typeof msg.seq === "number" ? msg.seq : (msg.seq ? parseInt(msg.seq, 10) : undefined);
      if (seq !== undefined && !isNaN(seq)) {
        if (seq <= this.lastSeq) {
          console.log("[WS] discarding duplicate message", msg.type, "seq:", seq, "lastSeq:", this.lastSeq);
          return;
        }
        this.lastSeq = seq;
      }
      this.handlers.forEach((h) => h(msg as WSMessage));
    }
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

export const wsClient = new ChessWSClient();
