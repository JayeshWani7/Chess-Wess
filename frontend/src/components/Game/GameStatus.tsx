import { useGameStore } from "../../store/gameStore";

export default function GameStatus() {
  const { chess, status, playerColor } = useGameStore();

  if (status !== "active") return null;

  const turn = chess.turn();
  const isMyTurn = turn === playerColor;
  const inCheck = chess.inCheck();

  let message = isMyTurn ? "Your turn" : "Opponent's turn";
  if (inCheck && isMyTurn) message = "⚠ You are in check!";
  else if (inCheck && !isMyTurn) message = "Opponent is in check";

  return (
    <div
      className={`text-center text-sm font-semibold py-2 px-4 rounded-lg ${
        isMyTurn
          ? "bg-chrono-accent/20 text-chrono-accent border border-chrono-accent/40"
          : "bg-chrono-surface text-gray-400 border border-chrono-border"
      }`}
    >
      {message}
    </div>
  );
}
