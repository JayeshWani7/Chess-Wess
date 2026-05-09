import { motion } from "framer-motion";
import { useGameStore } from "../../store/gameStore";
import { useAuthStore } from "../../store/authStore";

interface GameOverModalProps {
  onRematch: () => void;
  onLobby: () => void;
}

export default function GameOverModal({ onRematch, onLobby }: GameOverModalProps) {
  const { result, winnerId } = useGameStore();
  const userId = useAuthStore((s) => s.userId);

  const didWin = winnerId === userId;
  const isDraw = result === "stalemate" || result === "draw";

  let headline = "Game Over";
  let sub = "";
  let emoji = "♟";

  if (isDraw) {
    headline = "Draw!";
    sub = result === "stalemate" ? "Stalemate" : "Draw agreed";
    emoji = "🤝";
  } else if (winnerId) {
    if (didWin) {
      headline = "You Win!";
      emoji = "🏆";
    } else {
      headline = "You Lose";
      emoji = "💀";
    }
    const resultLabels: Record<string, string> = {
      checkmate: "by checkmate",
      timeout: "on time",
      resign: "by resignation",
    };
    sub = resultLabels[result ?? ""] ?? "";
  }

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      className="fixed inset-0 bg-black/70 flex items-center justify-center z-50 p-4"
    >
      <motion.div
        initial={{ scale: 0.85, y: 20 }}
        animate={{ scale: 1, y: 0 }}
        className="card w-full max-w-sm text-center space-y-4"
      >
        <div className="text-6xl">{emoji}</div>
        <h2 className="text-3xl font-bold">{headline}</h2>
        {sub && <p className="text-gray-400 capitalize">{sub}</p>}

        <div className="flex gap-3 pt-2">
          <button onClick={onLobby} className="btn-ghost flex-1">
            Back to Lobby
          </button>
          <button onClick={onRematch} className="btn-primary flex-1">
            New Game
          </button>
        </div>
      </motion.div>
    </motion.div>
  );
}
