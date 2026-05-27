import { useEffect, useState, useCallback } from "react";
import { Link, useNavigate } from "react-router-dom";
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

const BOT_LEVELS = [
  { label: "Beginner", rating: 400, emoji: "🐣", description: "Plays random moves" },
  { label: "Novice", rating: 600, emoji: "🐥", description: "Occasionally captures" },
  { label: "Casual", rating: 800, emoji: "🐦", description: "Prefers captures" },
  { label: "Intermediate", rating: 1000, emoji: "🦅", description: "Basic tactics" },
  { label: "Advanced", rating: 1200, emoji: "🦁", description: "Positional play" },
  { label: "Expert", rating: 1400, emoji: "🐉", description: "2-ply search" },
  { label: "Master", rating: 1600, emoji: "👑", description: "3-ply search" },
];

export default function LobbyPage() {
  const { token, userId } = useAuthStore();
  const setActiveGame = useGameStore((s) => s.setActiveGame);
  const navigate = useNavigate();

  const [games, setGames] = useState<GameInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [creating, setCreating] = useState(false);
  const [timeControl, setTimeControl] = useState(600);
  const [color, setColor] = useState<"white" | "black">("white");
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<"human" | "bot">("bot");
  const [botRating, setBotRating] = useState(800);
  const [botColor, setBotColor] = useState<"white" | "black">("white");
  const [botTimeControl, setBotTimeControl] = useState(600);
  const [creatingBot, setCreatingBot] = useState(false);

  const fetchGames = useCallback(async () => {
    if (!token) return;
    try {
      const list = await api.listGames(token);
      setGames(list ?? []);
    } catch {
    }
  }, [token]);

  useEffect(() => {
    void fetchGames();
    const interval = setInterval(fetchGames, 5000);
    return () => clearInterval(interval);
  }, [fetchGames]);

  async function handleCreate() {
    if (!token || !userId) return;
    setCreating(true);
    setError(null);
    try {
      const game = await api.createGame(token, timeControl, color);
      const playerColor = game.black_player_id === userId ? "b" : "w";
      setActiveGame(game.id, game as Parameters<typeof setActiveGame>[1], playerColor);
      navigate(`/game/${game.id}`);
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
      navigate(`/game/${updated.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to join game");
    } finally {
      setLoading(false);
    }
  }

  async function handlePlayBot() {
    if (!token || !userId) return;
    setCreatingBot(true);
    setError(null);
    try {
      const game = await api.createBotGame(token, botTimeControl, botRating, botColor);
      const playerColor = game.black_player_id === userId ? "b" : "w";
      setActiveGame(game.id, game as Parameters<typeof setActiveGame>[1], playerColor);
      navigate(`/game/${game.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to start bot game");
    } finally {
      setCreatingBot(false);
    }
  }


  function formatTime(seconds: number) {
    if (seconds === 0) return "∞";
    const m = Math.floor(seconds / 60);
    const s = seconds % 60;
    return s === 0 ? `${m}m` : `${m}m ${s}s`;
  }

  return (
    <div className="space-y-6">
      <header className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
        <div>
          <p className="text-xs uppercase tracking-[0.3em] text-moss">Play Lobby</p>
          <h1 className="text-3xl font-display text-ink">Choose your match</h1>
        </div>
        <Link to="/history" className="btn-outline text-sm">
          View history
        </Link>
      </header>

      <div className="flex gap-2">
        <button
          onClick={() => setActiveTab("bot")}
          className={`px-4 py-2 rounded-full text-sm font-semibold transition-colors border ${
            activeTab === "bot"
              ? "border-gold bg-mist text-pine"
              : "border-line text-ink/70 hover:text-ink hover:bg-mist"
          }`}
        >
          🤖 Play vs Bot
        </button>
        <button
          onClick={() => setActiveTab("human")}
          className={`px-4 py-2 rounded-full text-sm font-semibold transition-colors border ${
            activeTab === "human"
              ? "border-gold bg-mist text-pine"
              : "border-line text-ink/70 hover:text-ink hover:bg-mist"
          }`}
        >
          👥 Play vs Human
        </button>
      </div>

      {activeTab === "bot" && (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          <div className="card">
            <h2 className="font-display text-lg mb-4">Choose Difficulty</h2>
            <div className="md:hidden">
              <select
                className="input text-sm"
                value={botRating}
                onChange={(e) => setBotRating(Number(e.target.value))}
              >
                {BOT_LEVELS.map((bot) => (
                  <option key={bot.rating} value={bot.rating}>
                    {bot.label} ({bot.rating})
                  </option>
                ))}
              </select>
              <p className="text-xs text-moss mt-2">
                {BOT_LEVELS.find((b) => b.rating === botRating)?.description}
              </p>
            </div>
            <div className="hidden md:block space-y-2">
              {BOT_LEVELS.map((bot) => (
                <button
                  key={bot.rating}
                  onClick={() => setBotRating(bot.rating)}
                  className={`w-full flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors border ${
                    botRating === bot.rating
                      ? "border-gold bg-mist text-pine"
                      : "border-line text-ink/70 hover:text-ink hover:bg-mist"
                  }`}
                >
                  <span className="text-xl">{bot.emoji}</span>
                  <div className="flex-1 text-left">
                    <span className="font-semibold">{bot.label}</span>
                    <span className="text-xs opacity-70 ml-2">({bot.rating})</span>
                  </div>
                  <span className="text-xs opacity-60">{bot.description}</span>
                </button>
              ))}
            </div>
          </div>

          <div className="card">
            <h2 className="font-display text-lg mb-4">Game Options</h2>
            <div className="space-y-4">
              <div>
                <label className="block text-xs text-moss mb-2">Time Control</label>
                <div className="md:hidden">
                  <select
                    className="input text-sm"
                    value={botTimeControl}
                    onChange={(e) => setBotTimeControl(Number(e.target.value))}
                  >
                    {TIME_CONTROLS.map((tc) => (
                      <option key={tc.value} value={tc.value}>
                        {tc.label}
                      </option>
                    ))}
                  </select>
                </div>
                <div className="hidden md:block space-y-1">
                  {TIME_CONTROLS.map((tc) => (
                    <button
                      key={tc.value}
                      onClick={() => setBotTimeControl(tc.value)}
                      className={`w-full text-left px-3 py-2 rounded-lg text-sm transition-colors border ${
                        botTimeControl === tc.value
                          ? "border-gold bg-mist text-pine"
                          : "border-line text-ink/70 hover:text-ink hover:bg-mist"
                      }`}
                    >
                      {tc.label}
                    </button>
                  ))}
                </div>
              </div>

              <div>
                <label className="block text-xs text-moss mb-2">Play as</label>
                <div className="md:hidden">
                  <select
                    className="input text-sm"
                    value={botColor}
                    onChange={(e) => setBotColor(e.target.value as "white" | "black")}
                  >
                    <option value="white">White</option>
                    <option value="black">Black</option>
                  </select>
                </div>
                <div className="hidden md:flex gap-2">
                  {(["white", "black"] as const).map((c) => (
                    <button
                      key={c}
                      onClick={() => setBotColor(c)}
                      className={`flex-1 py-2 rounded-lg text-sm font-semibold capitalize transition-colors border ${
                        botColor === c
                          ? "border-gold bg-mist text-pine"
                          : "border-line text-ink/70 hover:text-ink hover:bg-mist"
                      }`}
                    >
                      {c === "white" ? "⬜ White" : "⬛ Black"}
                    </button>
                  ))}
                </div>
              </div>

              {error && <p className="text-rust text-xs">{error}</p>}

              <button
                onClick={handlePlayBot}
                disabled={creatingBot}
                className="btn-primary w-full"
              >
                {creatingBot
                  ? "Starting..."
                  : `Play vs ${BOT_LEVELS.find((b) => b.rating === botRating)?.label ?? "Bot"}`}
              </button>
            </div>
          </div>
        </div>
      )}

      {activeTab === "human" && (
        <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
          <div className="card md:col-span-1">
            <h2 className="font-display text-lg mb-4">New Game</h2>

            <div className="space-y-4">
              <div>
                <label className="block text-xs text-moss mb-2">Time Control</label>
                <div className="md:hidden">
                  <select
                    className="input text-sm"
                    value={timeControl}
                    onChange={(e) => setTimeControl(Number(e.target.value))}
                  >
                    {TIME_CONTROLS.map((tc) => (
                      <option key={tc.value} value={tc.value}>
                        {tc.label}
                      </option>
                    ))}
                  </select>
                </div>
                <div className="hidden md:block space-y-1">
                  {TIME_CONTROLS.map((tc) => (
                    <button
                      key={tc.value}
                      onClick={() => setTimeControl(tc.value)}
                      className={`w-full text-left px-3 py-2 rounded-lg text-sm transition-colors border ${
                        timeControl === tc.value
                          ? "border-gold bg-mist text-pine"
                          : "border-line text-ink/70 hover:text-ink hover:bg-mist"
                      }`}
                    >
                      {tc.label}
                    </button>
                  ))}
                </div>
              </div>

              <div>
                <label className="block text-xs text-moss mb-2">Play as</label>
                <div className="md:hidden">
                  <select
                    className="input text-sm"
                    value={color}
                    onChange={(e) => setColor(e.target.value as "white" | "black")}
                  >
                    <option value="white">White</option>
                    <option value="black">Black</option>
                  </select>
                </div>
                <div className="hidden md:flex gap-2">
                  {(["white", "black"] as const).map((c) => (
                    <button
                      key={c}
                      onClick={() => setColor(c)}
                      className={`flex-1 py-2 rounded-lg text-sm font-semibold capitalize transition-colors border ${
                        color === c
                          ? "border-gold bg-mist text-pine"
                          : "border-line text-ink/70 hover:text-ink hover:bg-mist"
                      }`}
                    >
                      {c === "white" ? "⬜ White" : "⬛ Black"}
                    </button>
                  ))}
                </div>
              </div>

              {error && (
                <p className="text-rust text-xs">{error}</p>
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

          <div className="card md:col-span-2">
            <div className="flex items-center justify-between mb-4">
              <h2 className="font-display text-lg">Open Games</h2>
              <button onClick={fetchGames} className="text-xs text-moss hover:text-ink">
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
                        className="flex items-center justify-between rounded-lg px-4 py-3 border border-line bg-paper"
                      >
                        <div>
                          <p className="text-sm font-semibold text-ink">
                            {game.white_player_id ? "⬜ Waiting for Black" : "⬛ Waiting for White"}
                          </p>
                          <p className="text-xs text-moss">
                            {formatTime(game.time_control)} · {game.id.slice(0, 8)}
                          </p>
                        </div>
                        {isOwn ? (
                          <span className="text-xs text-pine">Your game</span>
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
      )}
    </div>
  );
}
