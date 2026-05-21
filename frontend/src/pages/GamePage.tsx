import { useCallback, useEffect, useRef, useState } from "react";
import { useGameStore } from "../store/gameStore";
import { useAuthStore } from "../store/authStore";
import { wsClient, type WSMessage } from "../utils/wsClient";
import { api } from "../utils/api";
import { useEnergy } from "../hooks/useEnergy";
import ChessBoard from "../components/Board/ChessBoard";
import MoveHistory from "../components/Game/MoveHistory";
import PlayerClock from "../components/Game/PlayerClock";
import GameStatus from "../components/Game/GameStatus";
import GameOverModal from "../components/Game/GameOverModal";
import TimelinePanel from "../components/Timeline/TimelinePanel";
import { EnergyPanel, OpponentEnergyPanel } from "../components/Energy/EnergyPanel";

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
    setPlayerColor,
    setPlayerEnergy,
    setOpponentEnergy,
  } = useGameStore();

  const { rewindTimeline, jumpTimeline } = useEnergy();

  const { token, userId, username } = useAuthStore();
  const [resigning, setResigning] = useState(false);
  const [opponentName, setOpponentName] = useState("Opponent");
  const [opponentId, setOpponentId] = useState<string | null>(null);
  const [isOpponentBot, setIsOpponentBot] = useState(false);
  const [opponentBotRating, setOpponentBotRating] = useState(0);
  const [timelineNodeLimit, setTimelineNodeLimit] = useState<number | null>(200);
  const timelineLimitRef = useRef<number | null>(200);
  const connectedRef = useRef(false);

  useEffect(() => {
    timelineLimitRef.current = timelineNodeLimit;
  }, [timelineNodeLimit]);

  const refreshTimeline = useCallback(async () => {
    if (!token || !activeGameId) return;
    try {
      const limit = timelineLimitRef.current ?? undefined;
      const data = await api.getGameTimeline(token, activeGameId, limit);
      setTimelineData(data.timelines, data.active_timeline_id ?? null);
    } catch {
    }
  }, [token, activeGameId, setTimelineData]);

  useEffect(() => {
    if (!activeGameId || !token || connectedRef.current) return;
    connectedRef.current = true;

    wsClient.connect(activeGameId, token);

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
        case "timeline_renamed": {
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

  useEffect(() => {
    if (!gameInfo || !userId || !token) return;
    const expectedColor = gameInfo.black_player_id === userId ? "b" : "w";
    setPlayerColor(expectedColor);
    const oppId =
      expectedColor === "w" ? gameInfo.black_player_id : gameInfo.white_player_id;
    if (!oppId) return;

    setOpponentId(oppId);

    api.getUser(token, oppId)
      .then((u) => {
        setOpponentName(u.username);
        setIsOpponentBot(u.is_bot);
        if (u.is_bot) {
          setOpponentBotRating(u.rating || 0);
        }
      })
      .catch(() => setOpponentName("Opponent"));

    Promise.all([
      api.getPlayerEnergy(token, gameInfo.id),
      api.getOpponentEnergy(token, gameInfo.id, oppId),
    ])
      .then(([playerEnergy, opponentEnergy]) => {
        setPlayerEnergy(playerEnergy);
        setOpponentEnergy(opponentEnergy);
      })
      .catch(() => {
      });
  }, [gameInfo, userId, token, setPlayerColor, setPlayerEnergy, setOpponentEnergy]);

  useEffect(() => {
    if (status !== "active") return;
    if (chess.isCheckmate()) {
      const loserColor = chess.turn();
      const winnerId =
        loserColor === "w" ? gameInfo?.black_player_id : gameInfo?.white_player_id;
      setGameOver("checkmate", winnerId ?? null);
    } else if (chess.isStalemate() || chess.isDraw()) {
      setGameOver("stalemate", null);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [chess]);

  useEffect(() => {
    refreshTimeline();
  }, [refreshTimeline]);

  async function handleResign() {
    if (!token || !activeGameId || resigning) return;
    setResigning(true);
    try {
      await api.resignGame(token, activeGameId);
    } catch {
    } finally {
      setResigning(false);
    }
  }

  function handleLobby() {
    wsClient.disconnect();
    leaveGame();
  }

  async function handleSwitchTimeline(timelineId: string) {
    if (!timelineId) return;
    if (timelineId !== activeTimelineId) {
      const ok = await jumpTimeline(timelineId);
      if (!ok) return;
    }
    setActiveTimelineId(timelineId);
    wsClient.switchTimeline(timelineId);
  }

  async function handleRewind(nodeId: string) {
    if (!nodeId) return;
    const targetNode = nodesById[nodeId];
    const activeNode = activeTimelineLatestNodeId
      ? nodesById[activeTimelineLatestNodeId]
      : null;
    if (!targetNode || !activeNode) return;

    const turnsBack = Math.max(0, activeNode.turn_number - targetNode.turn_number);
    if (turnsBack > 0) {
      const ok = await rewindTimeline(turnsBack, targetNode.timeline_id);
      if (!ok) return;
    }

    wsClient.sendRewind(nodeId);
  }

  async function handleRenameTimeline(timelineId: string, name: string) {
    if (!token || !activeGameId) return;
    const trimmed = name.trim();
    if (!timelineId || !trimmed) return;
    try {
      await api.renameTimeline(token, activeGameId, timelineId, trimmed);
      refreshTimeline();
    } catch {
    }
  }

  function handleLoadMoreGraph() {
    setTimelineNodeLimit((prev) => {
      if (prev == null) return null;
      return prev + 200;
    });
  }

  function handleLoadFullGraph() {
    setTimelineNodeLimit(null);
  }

  const whiteName = playerColor === "w" ? (username ?? "You") : opponentName;
  const blackName = playerColor === "b" ? (username ?? "You") : opponentName;

  return (
    <div className="min-h-screen">
      <div className="mx-auto w-full max-w-6xl px-4 py-6 space-y-6">
        <header className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <div>
            <p className="text-xs uppercase tracking-[0.3em] text-slate-500">Multiverse Chess</p>
            <h1 className="text-2xl font-semibold text-chrono-accent">ChessWess Arena</h1>
          </div>
          <div className="flex gap-2">
          <button
            onClick={handleResign}
            disabled={resigning || status !== "active"}
            className="btn-ghost text-sm text-rose-300 border-rose-800 hover:bg-rose-900/20"
          >
            {resigning ? "Resigning..." : "Resign"}
          </button>
          <button onClick={handleLobby} className="btn-ghost text-sm">
            ← Lobby
          </button>
          </div>
        </header>

        <main className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_320px] items-start">
          <section className="space-y-4">
            <div className="w-full" style={{ width: "min(92vw, 620px)" }}>
              <OpponentEnergyPanel
                opponentName={opponentName}
                isBot={isOpponentBot}
                botRating={opponentBotRating}
              />
            </div>

            <div className="w-full" style={{ width: "min(92vw, 620px)" }}>
              <PlayerClock
                color={playerColor === "w" ? "b" : "w"}
                username={playerColor === "w" ? blackName : whiteName}
              />
            </div>

            <ChessBoard />

            <div className="w-full" style={{ width: "min(92vw, 620px)" }}>
              <PlayerClock
                color={playerColor ?? "w"}
                username={username ?? "You"}
              />
            </div>

            <div className="w-full" style={{ width: "min(92vw, 620px)" }}>
              <EnergyPanel />
            </div>
          </section>

          <aside className="flex flex-col gap-4 w-full">
            <GameStatus />
            <MoveHistory />

            <div className="card text-xs text-slate-400 space-y-1">
              <p>
                <span className="text-slate-500">Game ID:</span>{" "}
                {activeGameId?.slice(0, 8)}
              </p>
              <p>
                <span className="text-slate-500">You play:</span>{" "}
                {playerColor === "w" ? "⬜ White" : "⬛ Black"}
              </p>
              <p>
                <span className="text-slate-500">Status:</span> {status}
              </p>
            </div>
          </aside>
        </main>

        {status === "completed" && (
          <GameOverModal onRematch={handleLobby} onLobby={handleLobby} />
        )}

        <section className="w-full">
          <TimelinePanel
            timelines={timelines}
            activeTimelineId={activeTimelineId}
            activeTimelineLatestNodeId={activeTimelineLatestNodeId}
            selectedNodeId={selectedTimelineNodeId}
            nodesById={nodesById}
            onSelectNode={selectTimelineNode}
            onRewind={handleRewind}
            onSwitchTimeline={handleSwitchTimeline}
            onRenameTimeline={handleRenameTimeline}
            onLoadMoreGraph={handleLoadMoreGraph}
            onLoadFullGraph={handleLoadFullGraph}
            nodeLimit={timelineNodeLimit}
          />
        </section>
      </div>
    </div>
  );
}
