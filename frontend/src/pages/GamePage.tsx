import { useCallback, useEffect, useRef, useState, useMemo } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { useGameStore } from "../store/gameStore";
import { useAuthStore } from "../store/authStore";
import { wsClient, type WSMessage } from "../utils/wsClient";
import { api } from "../utils/api";
import { useEnergy } from "../hooks/useEnergy";
import { calculateRewindCost } from "../utils/energy";
import ChessBoard from "../components/Board/ChessBoard";
import MoveHistory from "../components/Game/MoveHistory";
import PlayerClock from "../components/Game/PlayerClock";
import GameStatus from "../components/Game/GameStatus";
import GameOverModal from "../components/Game/GameOverModal";
import RulesModal from "../components/Game/RulesModal";
import TimelinePanel from "../components/Timeline/TimelinePanel";
import { EnergyPanel, EnergyNotification, OpponentEnergyPanel } from "../components/Energy/EnergyPanel";
import BoardDiffPanel from "../components/Game/BoardDiffPanel";


export default function GamePage() {
  const {
    activeGameId,
    gameInfo,
    chess,
    status,
    playerColor,
    leaveGame,
    applyMove,
    setGameOver,
    setActiveGame,
    setTimelineData,
    addTimelineNode,
    addTimeline,
    renameTimelineLocal,
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
    playerEnergy,
    merges,
    sandboxMode,
    sandboxMoves,
    toggleSandboxMode,
    manifestSandbox,
    manifestNextQueueItem,
    addMergeLocal,
    selectedCompareNodeId,
    selectCompareNode,
    addAnnotationLocal,
  } = useGameStore();

  const { rewindTimeline, jumpTimeline } = useEnergy();

  const { token, userId, username } = useAuthStore();
  const { gameId } = useParams();
  const navigate = useNavigate();
  const [resigning, setResigning] = useState(false);
  const [opponentName, setOpponentName] = useState("Opponent");
  const [isOpponentBot, setIsOpponentBot] = useState(false);
  const [opponentBotRating, setOpponentBotRating] = useState(0);
  const [timelineNodeLimit, setTimelineNodeLimit] = useState<number | null>(200);
  const [showRules, setShowRules] = useState(false);
  const [energyToast, setEnergyToast] = useState<{
    message: string;
    type: "warning" | "error" | "info";
  } | null>(null);
  const timelineLimitRef = useRef<number | null>(200);
  const connectedRef = useRef(false);
  const toastTimerRef = useRef<number | null>(null);

  useEffect(() => {
    timelineLimitRef.current = timelineNodeLimit;
  }, [timelineNodeLimit]);

  useEffect(() => {
    if (!gameId || !token) return;
    if (activeGameId === gameId && gameInfo) return;

    let cancelled = false;

    api.getGame(token, gameId)
      .then((game) => {
        if (cancelled || !game?.id) return;
        if (userId && game.white_player_id !== userId && game.black_player_id !== userId) {
          navigate("/lobby", { replace: true });
          return;
        }
        const playerColor = game.black_player_id === userId ? "b" : "w";
        setActiveGame(game.id, game as Parameters<typeof setActiveGame>[1], playerColor);
      })
      .catch(() => {
        if (!cancelled) {
          navigate("/lobby", { replace: true });
        }
      });

    return () => {
      cancelled = true;
    };
  }, [gameId, token, userId, activeGameId, gameInfo, setActiveGame, navigate]);

  useEffect(() => {
    if (!activeGameId) return;
    const key = `chesswess.rules.v1.seen:${userId ?? "anon"}`;
    if (window.localStorage.getItem(key) !== "true") {
      setShowRules(true);
    }
  }, [activeGameId, userId]);

  useEffect(() => {
    if (!energyToast) return;
    if (toastTimerRef.current) {
      window.clearTimeout(toastTimerRef.current);
    }
    toastTimerRef.current = window.setTimeout(() => {
      setEnergyToast(null);
      toastTimerRef.current = null;
    }, 3200);

    return () => {
      if (toastTimerRef.current) {
        window.clearTimeout(toastTimerRef.current);
        toastTimerRef.current = null;
      }
    };
  }, [energyToast]);

  const refreshTimeline = useCallback(async () => {
    if (!token || !activeGameId) return;
    try {
      const limit = timelineLimitRef.current ?? undefined;
      const data = await api.getGameTimeline(token, activeGameId, limit);
      setTimelineData(data.timelines, data.active_timeline_id ?? null, (data as any).merges);
    } catch {
    }
  }, [token, activeGameId, setTimelineData]);

  const syncFullState = useCallback(async () => {
    if (!token || !activeGameId || !gameInfo) return;
    try {
      const limit = timelineLimitRef.current ?? undefined;
      const oppId = playerColor === "w" ? gameInfo.black_player_id : gameInfo.white_player_id;

      const [timelineData, game, playerEnergy, opponentEnergy] = await Promise.all([
        api.getGameTimeline(token, activeGameId, limit),
        api.getGame(token, activeGameId),
        api.getPlayerEnergy(token, activeGameId),
        oppId ? api.getOpponentEnergy(token, activeGameId, oppId) : Promise.resolve(null),
      ]);

      if (game) {
        setActiveGame(game.id, game as Parameters<typeof setActiveGame>[1], playerColor || "w");
      }
      if (timelineData) {
        setTimelineData(timelineData.timelines, timelineData.active_timeline_id ?? null, (timelineData as any).merges);
      }
      if (playerEnergy) {
        setPlayerEnergy(playerEnergy);
      }
      if (opponentEnergy) {
        setOpponentEnergy(opponentEnergy);
      }
      console.log("[Sync] full state sync complete");
    } catch (err) {
      console.error("[Sync] failed to sync full state", err);
    }
  }, [token, activeGameId, gameInfo, playerColor, setActiveGame, setTimelineData, setPlayerEnergy, setOpponentEnergy]);

  const syncFullStateRef = useRef(syncFullState);
  useEffect(() => {
    syncFullStateRef.current = syncFullState;
  }, [syncFullState]);

  useEffect(() => {
    if (!activeGameId || !token || connectedRef.current) return;
    connectedRef.current = true;

    wsClient.connect(activeGameId, token);

    const unsub = wsClient.onMessage((msg: WSMessage) => {
      switch (msg.type) {
        case "move": {
          const p = msg.payload as {
            id: string;
            player_id: string;
            uci: string;
            san: string;
            fen: string;
            timeline_id: string;
            parent_node_id: string;
            turn_number: number;
            created_at: string;
          };
          if (activeGameId && p.timeline_id) {
            addTimelineNode({
              id: p.id,
              game_id: activeGameId,
              timeline_id: p.timeline_id,
              parent_node_id: p.parent_node_id || null,
              move: { uci: p.uci, san: p.san },
              board_state: p.fen,
              turn_number: p.turn_number,
              created_by_user: p.player_id,
              created_at: p.created_at,
            });
          }
          if (p.player_id !== userId) {
            const from = p.uci.slice(0, 2);
            const to = p.uci.slice(2, 4);
            const promotion = p.uci[4] as "q" | "r" | "b" | "n" | undefined;
            applyMove({ from, to, promotion } as Parameters<typeof applyMove>[0], p.fen);
          } else {
            manifestNextQueueItem(p.id, p.timeline_id);
          }
          break;
        }
        case "timeline_created": {
          const p = msg.payload as {
            timeline_id: string;
            timeline_name?: string;
            root_node_id: string;
            board_state: string;
            turn_number: number;
            created_by_user: string;
            created_at: string;
          };
          if (activeGameId && p?.timeline_id) {
            addTimeline(p.timeline_id, p.timeline_name, {
              id: p.root_node_id,
              game_id: activeGameId,
              timeline_id: p.timeline_id,
              parent_node_id: null,
              move: null,
              board_state: p.board_state,
              turn_number: p.turn_number,
              created_by_user: p.created_by_user,
              created_at: p.created_at,
            });
            setActiveTimelineId(p.timeline_id);
            wsClient.switchTimeline(p.timeline_id);

            if (p.created_by_user === userId) {
              manifestNextQueueItem(p.root_node_id, p.timeline_id);
            }
          }
          break;
        }
        case "timeline_renamed": {
          const p = msg.payload as { timeline_id: string; timeline_name: string };
          if (p?.timeline_id && p.timeline_name) {
            renameTimelineLocal(p.timeline_id, p.timeline_name);
          }
          break;
        }
        case "timeline_switched": {
          const p = msg.payload as { timeline_id: string };
          if (p?.timeline_id) {
            setActiveTimelineId(p.timeline_id);
          }
          break;
        }
        case "timeline_merged": {
          const p = msg.payload as { source_node_id: string; target_node_id: string };
          if (p?.source_node_id && p.target_node_id) {
            addMergeLocal(p.source_node_id, p.target_node_id);
            refreshTimeline();
          }
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
        case "node_annotated": {
          const p = msg.payload as {
            node_id: string;
            user_id: string;
            username: string;
            annotation: string;
            label_tag: string | null;
          };
          if (p?.node_id) {
            addAnnotationLocal(p.node_id, p.user_id, p.username, p.annotation, p.label_tag);
            refreshTimeline();
          }
          break;
        }
        case "resync": {
          console.log("[WS] server requested full state resync");
          syncFullStateRef.current();
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
  }, [activeGameId, token, userId, applyMove, setGameOver, addTimelineNode, addTimeline, renameTimelineLocal, setActiveTimelineId, manifestNextQueueItem, addMergeLocal, refreshTimeline, addAnnotationLocal]);

  useEffect(() => {
    if (!gameInfo || !userId || !token) return;
    const expectedColor = gameInfo.black_player_id === userId ? "b" : "w";
    setPlayerColor(expectedColor);
    const oppId =
      expectedColor === "w" ? gameInfo.black_player_id : gameInfo.white_player_id;
    if (!oppId) return;

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
  }, [refreshTimeline, timelineNodeLimit]);

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
    navigate("/lobby");
  }

  function handleCloseRules() {
    const key = `chesswess.rules.v1.seen:${userId ?? "anon"}`;
    window.localStorage.setItem(key, "true");
    setShowRules(false);
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
      const cost = calculateRewindCost(turnsBack);
      if (playerEnergy && playerEnergy.energy_remaining < cost) {
        setEnergyToast({
          message: `Insufficient energy to rewind ${turnsBack} turn${turnsBack === 1 ? "" : "s"}. Need ${cost}, have ${playerEnergy.energy_remaining}.`,
          type: "warning",
        });
        return;
      }
      const ok = await rewindTimeline(turnsBack, targetNode.timeline_id);
      if (!ok) {
        setEnergyToast({
          message: "Unable to rewind and branch right now. Please try again.",
          type: "error",
        });
        return;
      }
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

  // Keyboard Hotkeys & Shortcut HUD
  const [showHud, setShowHud] = useState(false);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement | null;
      if (
        target &&
        (target.tagName === "INPUT" ||
          target.tagName === "TEXTAREA" ||
          target.isContentEditable)
      ) {
        return;
      }

      if (e.key === "?" && !e.ctrlKey && !e.altKey && !e.metaKey) {
        e.preventDefault();
        setShowHud((prev) => !prev);
        return;
      }

      if (e.ctrlKey && (e.key === "ArrowLeft" || e.key === "ArrowRight")) {
        e.preventDefault();
        const currentNodeId = selectedTimelineNodeId || activeTimelineLatestNodeId;
        if (!currentNodeId) return;

        const currentNode = nodesById[currentNodeId];
        if (!currentNode) return;

        const timelineId = currentNode.timeline_id;
        const list = timelines.find((t) => t.timeline_id === timelineId)?.nodes || [];
        const idx = list.findIndex((n) => n.id === currentNodeId);

        if (e.key === "ArrowLeft") {
          if (idx > 0) {
            selectTimelineNode(list[idx - 1].id);
          } else if (currentNode.parent_node_id) {
            selectTimelineNode(currentNode.parent_node_id);
          }
        } else {
          if (idx >= 0 && idx < list.length - 1) {
            selectTimelineNode(list[idx + 1].id);
          }
        }
        return;
      }

      if (e.ctrlKey && (e.key === "ArrowUp" || e.key === "ArrowDown")) {
        e.preventDefault();
        if (timelines.length <= 1 || !activeTimelineId) return;

        const idx = timelines.findIndex((t) => t.timeline_id === activeTimelineId);
        if (idx === -1) return;

        let nextIdx = e.key === "ArrowUp" ? idx - 1 : idx + 1;
        if (nextIdx < 0) nextIdx = timelines.length - 1;
        if (nextIdx >= timelines.length) nextIdx = 0;

        handleSwitchTimeline(timelines[nextIdx].timeline_id);
        return;
      }

      if (e.altKey && e.key >= "1" && e.key <= "9") {
        e.preventDefault();
        const num = parseInt(e.key, 10);
        const targetIdx = num - 1;
        if (targetIdx < timelines.length) {
          handleSwitchTimeline(timelines[targetIdx].timeline_id);
        }
        return;
      }

      if (e.ctrlKey && e.key.toLowerCase() === "d") {
        e.preventDefault();
        selectCompareNode(null);
        return;
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => {
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [
    activeTimelineId,
    activeTimelineLatestNodeId,
    selectedTimelineNodeId,
    nodesById,
    timelines,
    selectTimelineNode,
    handleSwitchTimeline,
    selectCompareNode,
  ]);

  const mergeSuggestion = useMemo(() => {
    if (!token || !activeGameId) return null;
    const targetNode = selectedTimelineNodeId ? nodesById[selectedTimelineNodeId] : (activeTimelineLatestNodeId ? nodesById[activeTimelineLatestNodeId] : null);
    if (!targetNode) return null;

    const targetFen = targetNode.board_state;
    const targetTimelineId = targetNode.timeline_id;

    for (const timeline of timelines) {
      if (timeline.timeline_id === targetTimelineId || timeline.timeline_id === "sandbox") {
        continue;
      }
      for (const node of timeline.nodes) {
        if (node.board_state === targetFen) {
          const exists = merges.some(
            (m) =>
              (m.source_node_id === targetNode.id && m.target_node_id === node.id) ||
              (m.source_node_id === node.id && m.target_node_id === targetNode.id)
          );
          if (!exists) {
            return {
              sourceNode: targetNode,
              targetNode: node,
              targetTimelineName: timeline.timeline_name ?? `Timeline ${timeline.timeline_id.slice(0, 8)}`,
            };
          }
        }
      }
    }
    return null;
  }, [token, activeGameId, selectedTimelineNodeId, activeTimelineLatestNodeId, nodesById, timelines, merges]);

  const whiteName = playerColor === "w" ? (username ?? "You") : opponentName;
  const blackName = playerColor === "b" ? (username ?? "You") : opponentName;

  return (
    <div className="relative space-y-6">
      {energyToast && (
        <div className="fixed right-6 top-20 z-50 w-[320px] max-w-[90vw]">
          <EnergyNotification
            message={energyToast.message}
            type={energyToast.type}
            onDismiss={() => setEnergyToast(null)}
          />
        </div>
      )}
      <header className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div>
          <p className="text-xs uppercase tracking-[0.3em] text-moss">Active Game</p>
          <h1 className="text-2xl font-display text-ink">ChessWess Arena</h1>
          <p className="text-sm text-moss">
            Timeline: {activeTimelineId ? activeTimelineId.slice(0, 8) : "-"}
          </p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={() => setShowRules(true)}
            className="btn-outline text-sm"
          >
            Rules
          </button>
          <button
            onClick={handleResign}
            disabled={resigning || status !== "active"}
            className="btn-danger text-sm"
          >
            {resigning ? "Resigning..." : "Resign"}
          </button>
          <button onClick={handleLobby} className="btn-ghost text-sm">
            Return to Lobby
          </button>
        </div>
      </header>

      {mergeSuggestion && (
        <div className="card border-purple-500 bg-purple-50/50 flex flex-col md:flex-row md:items-center justify-between gap-3 p-4">
          <div className="flex items-center gap-3">
            <span className="flex h-8 w-8 items-center justify-center rounded-full bg-purple-100 text-purple-600">
              ⚡
            </span>
            <div>
              <h4 className="text-sm font-semibold text-purple-900">Timeline Merge Suggestion</h4>
              <p className="text-xs text-purple-700">
                The board state here matches a node in <strong>{mergeSuggestion.targetTimelineName}</strong> (Turn {mergeSuggestion.targetNode.turn_number}). Merge them to converge the timelines!
              </p>
            </div>
          </div>
          <button
            onClick={async () => {
              try {
                await api.mergeTimelines(
                  token!,
                  activeGameId!,
                  mergeSuggestion.sourceNode.id,
                  mergeSuggestion.targetNode.id
                );
              } catch (err) {
                console.error("Failed to merge timelines:", err);
                setEnergyToast({
                  message: "Failed to merge timelines. Ensure FENs match exactly.",
                  type: "error",
                });
              }
            }}
            className="btn bg-purple-600 hover:bg-purple-700 text-white text-xs px-4 py-2 rounded-lg font-medium transition-colors"
          >
            Merge Timelines
          </button>
        </div>
      )}

      <main className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_340px] items-start">
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

          {selectedCompareNodeId && (
            <div className="w-full" style={{ width: "min(92vw, 620px)" }}>
              <BoardDiffPanel
                fenA={nodesById[selectedCompareNodeId]?.board_state ?? ""}
                fenB={nodesById[selectedTimelineNodeId || activeTimelineLatestNodeId || ""]?.board_state ?? ""}
                labelA={`Compare Node (Turn ${nodesById[selectedCompareNodeId]?.turn_number ?? 0})`}
                labelB={
                  selectedTimelineNodeId
                    ? `Selected Node (Turn ${nodesById[selectedTimelineNodeId]?.turn_number ?? 0})`
                    : `Active Node (Turn ${nodesById[activeTimelineLatestNodeId ?? ""]?.turn_number ?? 0})`
                }
                onClose={() => selectCompareNode(null)}
              />
            </div>
          )}
        </section>

        <aside className="flex flex-col gap-4 w-full">
          <GameStatus />
          <MoveHistory />

          {/* Sandbox Mode Control Panel */}
          {status === "active" && (
            <div className="card space-y-3">
              <h3 className="text-sm font-semibold text-ink flex items-center justify-between">
                <span>Branch Sandbox</span>
                {sandboxMode && (
                  <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-yellow-100 text-yellow-800 animate-pulse">
                    Active
                  </span>
                )}
              </h3>
              <p className="text-xs text-moss">
                {sandboxMode
                  ? "Make temporary moves on the board. They will appear yellow in the graph. Manifest them to commit them permanently."
                  : "Enable sandbox mode to experiment with moves without committing them."}
              </p>
              <div className="flex gap-2">
                {!sandboxMode ? (
                  <button
                    onClick={() => toggleSandboxMode(true)}
                    className="btn-outline text-xs w-full"
                  >
                    Start Sandbox Mode
                  </button>
                ) : (
                  <>
                    <button
                      onClick={manifestSandbox}
                      disabled={sandboxMoves.length === 0}
                      className="btn bg-yellow-500 hover:bg-yellow-600 text-white text-xs flex-1 py-1.5 rounded-lg font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                      Manifest ({sandboxMoves.length})
                    </button>
                    <button
                      onClick={() => toggleSandboxMode(false)}
                      className="btn-outline text-xs flex-1 py-1.5"
                    >
                      Clear
                    </button>
                  </>
                )}
              </div>
            </div>
          )}

          <div className="card text-xs text-moss space-y-1">
            <p>
              <span className="text-ink">Game ID:</span> {activeGameId?.slice(0, 8)}
            </p>
            <p>
              <span className="text-ink">You play:</span>{" "}
              {playerColor === "w" ? "⬜ White" : "⬛ Black"}
            </p>
            <p>
              <span className="text-ink">Status:</span> {status}
            </p>
          </div>
        </aside>
      </main>

      {status === "completed" && (
        <GameOverModal onRematch={handleLobby} onLobby={handleLobby} />
      )}

      {showRules && <RulesModal onClose={handleCloseRules} />}

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
          merges={merges}
          sandboxMoves={sandboxMoves}
        />
      </section>

      {/* Floating Hotkeys Help Button */}
      <div className="fixed bottom-6 right-6 z-40">
        <button
          onClick={() => setShowHud(true)}
          className="flex h-10 w-10 items-center justify-center rounded-full bg-purple-600 text-white shadow-lg hover:bg-purple-700 hover:scale-105 transition-all font-bold text-lg"
          title="Keyboard Shortcuts (?)"
        >
          ?
        </button>
      </div>

      {/* Shortcuts HUD Modal */}
      {showHud && (
        <div className="fixed inset-0 bg-ink/50 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="card max-w-md w-full border border-purple-500 bg-white/95 shadow-2xl p-6 relative">
            <button
              onClick={() => setShowHud(false)}
              className="absolute top-4 right-4 text-moss hover:text-ink text-lg"
            >
              ✕
            </button>
            <div className="flex items-center gap-2 mb-4">
              <span className="flex h-7 w-7 items-center justify-center rounded-full bg-purple-100 text-purple-600 text-sm">
                ⌨
              </span>
              <h3 className="text-lg font-bold text-ink">Keyboard Shortcuts</h3>
            </div>
            
            <div className="space-y-3.5 text-sm text-ink">
              <div className="flex items-center justify-between border-b border-line pb-2">
                <span>Step Turn Backward / Forward</span>
                <span className="flex gap-1">
                  <kbd className="px-1.5 py-0.5 rounded border border-line bg-paper text-xs shadow-sm font-mono font-bold">Ctrl</kbd>
                  <span>+</span>
                  <kbd className="px-1.5 py-0.5 rounded border border-line bg-paper text-xs shadow-sm font-mono font-bold">←</kbd>
                  <span>/</span>
                  <kbd className="px-1.5 py-0.5 rounded border border-line bg-paper text-xs shadow-sm font-mono font-bold">→</kbd>
                </span>
              </div>

              <div className="flex items-center justify-between border-b border-line pb-2">
                <span>Switch Timeline Branch</span>
                <span className="flex gap-1">
                  <kbd className="px-1.5 py-0.5 rounded border border-line bg-paper text-xs shadow-sm font-mono font-bold">Ctrl</kbd>
                  <span>+</span>
                  <kbd className="px-1.5 py-0.5 rounded border border-line bg-paper text-xs shadow-sm font-mono font-bold">↑</kbd>
                  <span>/</span>
                  <kbd className="px-1.5 py-0.5 rounded border border-line bg-paper text-xs shadow-sm font-mono font-bold">↓</kbd>
                </span>
              </div>

              <div className="flex items-center justify-between border-b border-line pb-2">
                <span>Jump to Timeline (1-9)</span>
                <span className="flex gap-1">
                  <kbd className="px-1.5 py-0.5 rounded border border-line bg-paper text-xs shadow-sm font-mono font-bold">Alt</kbd>
                  <span>+</span>
                  <kbd className="px-1.5 py-0.5 rounded border border-line bg-paper text-xs shadow-sm font-mono font-bold">1-9</kbd>
                </span>
              </div>

              <div className="flex items-center justify-between border-b border-line pb-2">
                <span>Close Comparison Diff View</span>
                <span className="flex gap-1">
                  <kbd className="px-1.5 py-0.5 rounded border border-line bg-paper text-xs shadow-sm font-mono font-bold">Ctrl</kbd>
                  <span>+</span>
                  <kbd className="px-1.5 py-0.5 rounded border border-line bg-paper text-xs shadow-sm font-mono font-bold">D</kbd>
                </span>
              </div>

              <div className="flex items-center justify-between">
                <span>Toggle Shortcut Guide</span>
                <kbd className="px-1.5 py-0.5 rounded border border-line bg-paper text-xs shadow-sm font-mono font-bold">?</kbd>
              </div>
            </div>

            <button
              onClick={() => setShowHud(false)}
              className="btn bg-purple-600 hover:bg-purple-700 text-white w-full text-xs py-2 rounded-lg font-medium transition-colors mt-6"
            >
              Got it
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
