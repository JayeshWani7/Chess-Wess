import { useCallback } from "react";
import { useGameStore } from "../../store/gameStore";
import { wsClient } from "../../utils/wsClient";
import { useAuthStore } from "../../store/authStore";
import Square from "./Square";
const FILES = ["a", "b", "c", "d", "e", "f", "g", "h"];
const RANKS = ["8", "7", "6", "5", "4", "3", "2", "1"];

function buildBoardMatrix(flipped: boolean) {
  const ranks = flipped ? [...RANKS].reverse() : RANKS;
  const files = flipped ? [...FILES].reverse() : FILES;
  return { ranks, files };
}

export default function ChessBoard() {
  const { chess, selectedSquare, legalMoves, playerColor, moves, status } =
    useGameStore();
  const selectSquare = useGameStore((s) => s.selectSquare);
  const applyMove = useGameStore((s) => s.applyMove);
  const token = useAuthStore((s) => s.token);

  const flipped = playerColor === "b";
  const { ranks, files } = buildBoardMatrix(flipped);

  // Last move squares for highlighting
  const lastMove = moves[moves.length - 1];
  const lastMoveSquares = lastMove
    ? [lastMove.moveUci.slice(0, 2), lastMove.moveUci.slice(2, 4)]
    : [];

  const handleSquareClick = useCallback(
    (square: string) => {
      if (status !== "active") return;
      if (chess.turn() !== playerColor) return;

      // If a square is already selected and this is a legal target → make the move
      if (selectedSquare && legalMoves.includes(square)) {
        const piece = chess.get(selectedSquare as Parameters<typeof chess.get>[0]);

        // Handle pawn promotion (auto-queen for now)
        let promotion: "q" | undefined;
        if (
          piece?.type === "p" &&
          ((playerColor === "w" && square[1] === "8") ||
            (playerColor === "b" && square[1] === "1"))
        ) {
          promotion = "q";
        }

        const move = chess.move({
          from: selectedSquare,
          to: square,
          promotion,
        });

        if (move) {
          const fen = chess.fen();
          // Undo the move — applyMove will re-apply it via store
          chess.undo();
          applyMove({ from: selectedSquare, to: square, promotion } as Parameters<typeof applyMove>[0], fen);
          wsClient.sendMove(
            move.from + move.to + (move.promotion ?? ""),
            move.san,
            fen
          );
          selectSquare(null);
        }
        return;
      }

      selectSquare(square);
    },
    [chess, selectedSquare, legalMoves, playerColor, status, applyMove, selectSquare, token]
  );

  return (
    <div className="relative select-none">
      {/* Board grid */}
      <div
        className="grid border-2 border-chrono-border rounded-sm overflow-hidden shadow-2xl"
        style={{ gridTemplateColumns: "repeat(8, 1fr)", width: "min(80vw, 560px)", height: "min(80vw, 560px)" }}
        role="grid"
        aria-label="Chess board"
      >
        {ranks.map((rank) =>
          files.map((file) => {
            const square = `${file}${rank}`;
            const piece = chess.get(square as Parameters<typeof chess.get>[0]);
            const isLight = (FILES.indexOf(file) + RANKS.indexOf(rank)) % 2 === 0;
            const isSelected = selectedSquare === square;
            const isLegal = legalMoves.includes(square);
            const isLastMove = lastMoveSquares.includes(square);

            return (
              <Square
                key={square}
                square={square}
                piece={piece ?? null}
                isLight={isLight}
                isSelected={isSelected}
                isLegal={isLegal}
                isLastMove={isLastMove}
                onClick={handleSquareClick}
              />
            );
          })
        )}
      </div>

      {/* File labels */}
      <div className="flex mt-1" style={{ width: "min(80vw, 560px)" }}>
        {files.map((f) => (
          <span key={f} className="flex-1 text-center text-xs text-gray-500">
            {f}
          </span>
        ))}
      </div>

      {/* Rank labels */}
      <div
        className="absolute top-0 left-0 flex flex-col"
        style={{ height: "min(80vw, 560px)", transform: "translateX(-16px)" }}
      >
        {ranks.map((r) => (
          <span key={r} className="flex-1 flex items-center text-xs text-gray-500">
            {r}
          </span>
        ))}
      </div>
    </div>
  );
}
