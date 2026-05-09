import { useEffect, useState, useCallback } from "react";
import { api, type GameInfo } from "../utils/api";
import { useAuthStore } from "../store/authStore";
import { useGameStore } from "../store/gameStore";
import { motion, AnimatePresence } from "framer-motion";

const TIME_CONTROLS = [
  { label: "Bullet 1+0", value: 60 },
  { label: "Blitz 3+0", value: 180 },
  { label: "Blitz 5+0", value: 300 },
  { label: "Rapid 10+0", value: 600 },
  { label: "Unlimited", value: 0 },
];

export default function LobbyPage() {
  const { token, userId, username, logout } = useAuthStore();
  const setActiveGame = useGameStore((s) => s.setActiveGame);

  const [games, setGames] = useState<GameInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [creating, setCreating] = useState(false);
  const [timeControl, setTimeControl] = useState(600);
  const [color, setColor] = useState<"white" | "black">("white");
  const [error, setError] = useState<string | null>(null);

  const fetchGames = useCallback(async () => {
    if (!token) return;
    try {
      const list = await api.listGames(token);
      setGames(list ?? []);
    } catch {
      // silently ignore polling errors
    }
  }, [token]);

  useEffect(() => {
    void fetchGames();
    const interval = setInterval(fetchGames, 5000);
    return () => clearInterval(interval);
  }, [fetchGames]);

  async function handleCreate() {
    if (!token) return;
    setCreating(true);
    setError(null);
    try {
      const game = await api.createGame(token, timeControl, color);
      const playerColor = color === "white" ? "w" : "b";
      setActiveGame(game.id, game as Parameters<typeof setActiveGame>[1], playerColor);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create game");
    } finally {
      setCreating(false);
    }
  }

  async function handleJoin(game: GameInfo) {
    if (!token || !userId) return;
    setLoading(true);
    setError(null);
    try {
      await api.joinGame(token, game.id);
      const updated = await api.getGame(token, game.id);
      const playerColor = updated.black_player_id === userId ? "b" : "w";
      setActiveGame(updated.id, updated as Parameters<typeof setActiveGame>[1], playerColor);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to join game");
    } finally {
      setLoading(false);
    }
  }

  function formatTime(seconds: number) {
    if (seconds === 0) return "∞";
    const m = Math.floor(seconds / 60);
    const s = seconds % 60;
    return s === 0 ? `${m}m` : `${m}m ${s}s`;
  }

  return (
    <div className="min-h-screen p-6 max-w-4xl mx-auto">
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <h1 className="text-2xl font-bold text-chrono-accent">♟ ChessWess</h1>
        <div className="flex items-center gap-3">
          <span className="text-gray-400 text-sm">@{username}</span>
          <button onClick={logout} className="btn-ghost text-sm">
            Sign out
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        {/* Create game panel */}
        <div className="card md:col-span-1">
          <h2 className="font-semibold text-lg mb-4">New Game</h2>

          <div className="space-y-4">
            <div>
              <label className="block text-xs text-gray-400 mb-2">Time Control</label>
              <div className="space-y-1">
                {TIME_CONTROLS.map((tc) => (
                  <button
                    key={tc.value}
                    onClick={() => setTimeControl(tc.value)}
                    className={`w-full text-left px-3 py-2 rounded-lg text-sm transition-colors ${
                      timeControl === tc.value
                        ? "bg-chrono-accent text-white"
                        : "hover:bg-chrono-border text-gray-300"
                    }`}
                  >
                    {tc.label}
                  </button>
                ))}
              </div>
            </div>

            <div>
              <label className="block text-xs text-gray-400 mb-2">Play as</label>
              <div className="flex gap-2">
                {(["white", "black"] as const).map((c) => (
                  <button
                    key={c}
                    onClick={() => setColor(c)}
                    className={`flex-1 py-2 rounded-lg text-sm font-semibold capitalize transition-colors ${
                      color === c
                        ? "bg-chrono-accent text-white"
                        : "border border-chrono-border text-gray-400 hover:text-white"
                    }`}
                  >
                    {c === "white" ? "⬜ White" : "⬛ Black"}
                  </button>
                ))}
              </div>
            </div>

            {error && (
              <p className="text-red-400 text-xs">{error}</p>
            )}

            <button
              onClick={handleCreate}
              disabled={creating}
              className="btn-primary w-full"
            >
              {creating ? "Creating..." : "Create Game"}
            </button>
          </div>
        </div>

        {/* Open games list */}
        <div className="card md:col-span-2">
          <div className="flex items-center justify-between mb-4">
            <h2 className="font-semibold text-lg">Open Games</h2>
            <button onClick={fetchGames} className="text-xs text-gray-400 hover:text-white">
              ↻ Refresh
            </button>
          </div>

          {games.length === 0 ? (
            <p className="text-gray-500 text-sm text-center py-8">
              No open games. Create one!
            </p>
          ) : (
            <AnimatePresence>
              <div className="space-y-2">
                {games.map((game) => {
                  const isOwn =
                    game.white_player_id === userId ||
                    game.black_player_id === userId;
                  return (
                    <motion.div
                      key={game.id}
                      initial={{ opacity: 0, x: -8 }}
                      animate={{ opacity: 1, x: 0 }}
                      exit={{ opacity: 0 }}
                      className="flex items-center justify-between bg-chrono-bg rounded-lg px-4 py-3 border border-chrono-border"
                    >
                      <div>
                        <p className="text-sm font-semibold">
                          {game.white_player_id ? "⬜ Waiting for Black" : "⬛ Waiting for White"}
                        </p>
                        <p className="text-xs text-gray-400">
                          {formatTime(game.time_control)} · {game.id.slice(0, 8)}
                        </p>
                      </div>
                      {isOwn ? (
                        <span className="text-xs text-chrono-accent">Your game</span>
                      ) : (
                        <button
                          onClick={() => handleJoin(game)}
                          disabled={loading}
                          className="btn-primary text-sm"
                        >
                          Join
                        </button>
                      )}
                    </motion.div>
                  );
                })}
              </div>
            </AnimatePresence>
          )}
        </div>
      </div>
    </div>
  );
}
