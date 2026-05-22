import { useState } from "react";
import { api } from "../utils/api";
import { useAuthStore } from "../store/authStore";
import { motion } from "framer-motion";

type Mode = "login" | "register";

export default function AuthPage() {
  const [mode, setMode] = useState<Mode>("login");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const login = useAuthStore((s) => s.login);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      const res =
        mode === "login"
          ? await api.login(username, password)
          : await api.register(username, password);
      login(res.token, res.user_id, res.username);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="min-h-screen grid lg:grid-cols-[1.2fr_0.8fr]">
      <div className="hidden lg:flex flex-col justify-center px-12">
        <div className="max-w-lg space-y-6">
          <p className="text-xs uppercase tracking-[0.4em] text-moss">ChessWess</p>
          <h1 className="text-4xl font-display text-pine leading-tight">
            Branch your fate.
          </h1>
          <p className="text-lg text-ink/70">
            Every move creates a new universe. Rewind, fork, and win across
            timelines that only you can see.
          </p>
          <div className="grid grid-cols-2 gap-4">
            {[
              "Rewind without regret",
              "Name every timeline",
              "Challenge multiverse AI",
              "Replay critical forks",
            ].map((text) => (
              <div key={text} className="rounded-xl border border-line bg-panel p-4">
                <p className="text-sm font-semibold text-ink">{text}</p>
              </div>
            ))}
          </div>
        </div>
      </div>

      <div className="flex items-center justify-center p-6 lg:p-12">
        <motion.div
          initial={{ opacity: 0, y: 24 }}
          animate={{ opacity: 1, y: 0 }}
          className="card w-full max-w-md"
        >
          <div className="text-center mb-8">
            <p className="text-xs uppercase tracking-[0.3em] text-moss">Welcome</p>
            <h1 className="text-3xl font-display text-pine tracking-tight">
              ChessWess
            </h1>
            <p className="text-moss text-sm mt-1">Chess across timelines</p>
          </div>

          <div className="flex rounded-full overflow-hidden border border-line mb-6">
            {(["login", "register"] as Mode[]).map((m) => (
              <button
                key={m}
                onClick={() => {
                  setMode(m);
                  setError(null);
                }}
                className={`flex-1 py-2 text-sm font-semibold capitalize transition-colors ${
                  mode === m
                    ? "bg-mist text-pine"
                    : "text-moss hover:text-ink"
                }`}
              >
                {m}
              </button>
            ))}
          </div>

          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="block text-xs text-moss mb-1">Username</label>
              <input
                className="input"
                placeholder="player1"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                minLength={3}
                maxLength={32}
                required
                autoFocus
              />
            </div>
            <div>
              <label className="block text-xs text-moss mb-1">Password</label>
              <input
                className="input"
                type="password"
                placeholder="••••••••"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                minLength={8}
                required
              />
            </div>

            {error && (
              <p className="text-rust text-sm bg-rust/10 border border-rust/40 rounded-lg px-3 py-2">
                {error}
              </p>
            )}

            <button
              type="submit"
              disabled={loading}
              className="btn-primary w-full"
            >
              {loading ? "..." : mode === "login" ? "Sign In" : "Create Account"}
            </button>
          </form>
        </motion.div>
      </div>
    </div>
  );
}
