import { useCallback, useEffect, useRef, useState } from "react";
import { useGameStore } from "../store/gameStore";
import { useAuthStore } from "../store/authStore";
import { wsClient, type WSMessage } from "../utils/wsClient";
import { api } from "../utils/api";
import ChessBoard from "../components/Board/ChessBoard";
import MoveHistory from "../components/Game/MoveHistory";
import PlayerClock from "../components/Game/PlayerClock";
import GameStatus from "../components/Game/GameStatus";
import GameOverModal from "../components/Game/GameOverModal";
import TimelinePanel from "../components/Timeline/TimelinePanel";

export default function GamePage() {
  const {
    activeGameId,
    gameInfo,
    chess,
    status,
    playerColor,
    leaveGame,
    loadMoves,
    applyMove,
    setGameOver,
    setTimelineData,
    selectTimelineNode,
    activeTimelineId,
    activeTimelineLatestNodeId,
    selectedTimelineNodeId,
    nodesById,
    timelines,
    setActiveTimelineId,
  } = useGameStore();

  const { token, userId, username } = useAuthStore();
  const [resigning, setResigning] = useState(false);
  const [opponentName, setOpponentName] = useState("Opponent");
  const connectedRef = useRef(false);

  const refreshTimeline = useCallback(async () => {
    if (!token || !activeGameId) return;
    try {
      const data = await api.getGameTimeline(token, activeGameId);
      setTimelineData(data.timelines, data.active_timeline_id ?? null);
    } catch {
      // Timeline data is optional for basic play; ignore errors for now.
    }
  }, [token, activeGameId, setTimelineData]);

  // Connect WebSocket and load existing moves on mount
  useEffect(() => {
    if (!activeGameId || !token || connectedRef.current) return;
    connectedRef.current = true;

    wsClient.connect(activeGameId, token);

    // Load move history from REST (handles page refresh / late join)
    api.getGameMoves(token, activeGameId).then((moves) => {
      loadMoves(
        moves.map((m) => ({
          id: m.id,
          gameId: m.game_id,
          playerId: m.player_id,
          moveNumber: m.move_number,
          moveSan: m.move_san,
          moveUci: m.move_uci,
          fenAfter: m.fen_after,
          createdAt: m.created_at,
        }))
      );
    });

    refreshTimeline();

    // Subscribe to WebSocket messages
    const unsub = wsClient.onMessage((msg: WSMessage) => {
      switch (msg.type) {
        case "move": {
          const p = msg.payload as {
            id: string;
            player_id: string;
            uci: string;
            san: string;
            fen: string;
          };
          // Only apply moves from the opponent (our own moves are applied optimistically)
          if (p.player_id !== userId) {
            const from = p.uci.slice(0, 2);
            const to = p.uci.slice(2, 4);
            const promotion = p.uci[4] as "q" | "r" | "b" | "n" | undefined;
            applyMove({ from, to, promotion } as Parameters<typeof applyMove>[0], p.fen);
          }
          refreshTimeline();
          break;
        }
        case "timeline_created": {
          const p = msg.payload as { timeline_id: string };
          if (p?.timeline_id) {
            setActiveTimelineId(p.timeline_id);
            wsClient.switchTimeline(p.timeline_id);
          }
          refreshTimeline();
          break;
        }
        case "timeline_switched": {
          const p = msg.payload as { timeline_id: string };
          if (p?.timeline_id) {
            setActiveTimelineId(p.timeline_id);
          }
          refreshTimeline();
          break;
        }
        case "game_over": {
          const p = msg.payload as { winner_id: string; result: string };
          setGameOver(
            p.result as Parameters<typeof setGameOver>[0],
            p.winner_id || null
          );
          break;
        }
        default:
          break;
      }
    });

    return () => {
      unsub();
      wsClient.disconnect();
      connectedRef.current = false;
    };
  }, [activeGameId, token, userId, loadMoves, applyMove, setGameOver, refreshTimeline]);

  // Resolve opponent display name (handles bots like "Bot-800")
  useEffect(() => {
    if (!gameInfo || !userId || !token) return;
    const opponentId =
      playerColor === "w" ? gameInfo.black_player_id : gameInfo.white_player_id;
    if (!opponentId) return;

    api.getUser(token, opponentId)
      .then((u) => setOpponentName(u.username))
      .catch(() => setOpponentName("Opponent"));
  }, [gameInfo, userId, playerColor, token]);

  // Detect checkmate / stalemate / draw locally after every move
  // (chess is a new instance after each applyMove, so this effect re-fires correctly)
  useEffect(() => {
    if (status !== "active") return;
    if (chess.isCheckmate()) {
      const loserColor = chess.turn(); // side to move is the one in checkmate
      const winnerId =
        loserColor === "w" ? gameInfo?.black_player_id : gameInfo?.white_player_id;
      setGameOver("checkmate", winnerId ?? null);
    } else if (chess.isStalemate() || chess.isDraw()) {
      setGameOver("stalemate", null);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [chess]); // chess is a new ref after every move — that's the trigger

  async function handleResign() {
    if (!token || !activeGameId || resigning) return;
    setResigning(true);
    try {
      await api.resignGame(token, activeGameId);
    } catch {
      // game_over will arrive via WebSocket
    } finally {
      setResigning(false);
    }
  }

  function handleLobby() {
    wsClient.disconnect();
    leaveGame();
  }

  function handleSwitchTimeline(timelineId: string) {
    if (!timelineId) return;
    setActiveTimelineId(timelineId);
    wsClient.switchTimeline(timelineId);
  }

  function handleRewind(nodeId: string) {
    if (!nodeId) return;
    wsClient.sendRewind(nodeId);
  }

  // Build display names
  const whiteName = playerColor === "w" ? (username ?? "You") : opponentName;
  const blackName = playerColor === "b" ? (username ?? "You") : opponentName;

  return (
    <div className="min-h-screen flex flex-col items-center justify-center p-4 gap-4">
      {/* Top bar */}
      <div className="flex items-center justify-between w-full max-w-5xl">
        <h1 className="text-xl font-bold text-chrono-accent">♟ ChessWess</h1>
        <div className="flex gap-2">
          <button
            onClick={handleResign}
            disabled={resigning || status !== "active"}
            className="btn-ghost text-sm text-red-400 border-red-800 hover:bg-red-900/20"
          >
            {resigning ? "Resigning..." : "Resign"}
          </button>
          <button onClick={handleLobby} className="btn-ghost text-sm">
            ← Lobby
          </button>
        </div>
      </div>

      {/* Main layout */}
      <div className="flex flex-col lg:flex-row gap-6 w-full max-w-5xl items-start justify-center">
        {/* Board column */}
        <div className="flex flex-col gap-3 items-center">
          {/* Opponent clock (top) */}
          <div className="w-full" style={{ width: "min(80vw, 560px)" }}>
            <PlayerClock
              color={playerColor === "w" ? "b" : "w"}
              username={playerColor === "w" ? blackName : whiteName}
            />
          </div>

          <ChessBoard />

          {/* Player clock (bottom) */}
          <div className="w-full" style={{ width: "min(80vw, 560px)" }}>
            <PlayerClock
              color={playerColor ?? "w"}
              username={username ?? "You"}
            />
          </div>
        </div>

        {/* Sidebar */}
        <div
          className="flex flex-col gap-3 w-full lg:w-64"
          style={{ minHeight: "min(80vw, 560px)" }}
        >
          <GameStatus />
          <MoveHistory />

          {/* Game info */}
          <div className="card text-xs text-gray-500 space-y-1">
            <p>
              <span className="text-gray-400">Game ID:</span>{" "}
              {activeGameId?.slice(0, 8)}
            </p>
            <p>
              <span className="text-gray-400">You play:</span>{" "}
              {playerColor === "w" ? "⬜ White" : "⬛ Black"}
            </p>
            <p>
              <span className="text-gray-400">Status:</span> {status}
            </p>
          </div>
        </div>
      </div>

      {/* Game over overlay */}
      {status === "completed" && (
        <GameOverModal onRematch={handleLobby} onLobby={handleLobby} />
      )}

      {/* Timeline panel */}
      <div className="w-full max-w-5xl">
        <TimelinePanel
          timelines={timelines}
          activeTimelineId={activeTimelineId}
          activeTimelineLatestNodeId={activeTimelineLatestNodeId}
          selectedNodeId={selectedTimelineNodeId}
          nodesById={nodesById}
          onSelectNode={selectTimelineNode}
          onRewind={handleRewind}
          onSwitchTimeline={handleSwitchTimeline}
        />
      </div>
    </div>
  );
}
