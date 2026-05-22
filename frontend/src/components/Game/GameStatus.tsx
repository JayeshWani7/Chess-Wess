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
      className={`text-center text-sm font-semibold py-3 px-4 rounded-xl ${
        isMyTurn
          ? "bg-mist text-pine border border-gold/40"
          : "glass text-ink"
      }`}
    >
      {message}
    </div>
  );
}
