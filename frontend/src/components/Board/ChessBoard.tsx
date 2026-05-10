import { useCallback, useState } from "react";
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

// Promotion piece options shown in the picker
const PROMO_PIECES: { piece: "q" | "r" | "b" | "n"; label: string; white: string; black: string }[] = [
  { piece: "q", label: "Queen",  white: "♕", black: "♛" },
  { piece: "r", label: "Rook",   white: "♖", black: "♜" },
  { piece: "b", label: "Bishop", white: "♗", black: "♝" },
  { piece: "n", label: "Knight", white: "♘", black: "♞" },
];

interface PendingPromotion {
  from: string;
  to: string;
}

export default function ChessBoard() {
  const { chess, selectedSquare, legalMoves, playerColor, moves, status } =
    useGameStore();
  const selectSquare = useGameStore((s) => s.selectSquare);
  const applyMove = useGameStore((s) => s.applyMove);
  const token = useAuthStore((s) => s.token);

  // Pending promotion: set when a pawn reaches the back rank
  const [pendingPromo, setPendingPromo] = useState<PendingPromotion | null>(null);

  const flipped = playerColor === "b";
  const { ranks, files } = buildBoardMatrix(flipped);

  // Last move squares for highlighting
  const lastMove = moves[moves.length - 1];
  const lastMoveSquares = lastMove
    ? [lastMove.moveUci.slice(0, 2), lastMove.moveUci.slice(2, 4)]
    : [];

  // Commit a move (with optional promotion piece)
  const commitMove = useCallback(
    (from: string, to: string, promotion?: "q" | "r" | "b" | "n") => {
      // Temporarily apply to get SAN + FEN, then undo so applyMove re-applies
      const move = chess.move({ from, to, promotion });
      if (!move) return;
      const fen = chess.fen();
      chess.undo();

      applyMove({ from, to, promotion } as Parameters<typeof applyMove>[0], fen);
      wsClient.sendMove(
        move.from + move.to + (move.promotion ?? ""),
        move.san,
        fen
      );
      selectSquare(null);
    },
    [chess, applyMove, selectSquare]
  );

  const handleSquareClick = useCallback(
    (square: string) => {
      if (status !== "active") return;
      if (chess.turn() !== playerColor) return;
      if (pendingPromo) return; // ignore clicks while picker is open

      if (selectedSquare && legalMoves.includes(square)) {
        const piece = chess.get(selectedSquare as Parameters<typeof chess.get>[0]);

        const isPromotion =
          piece?.type === "p" &&
          ((playerColor === "w" && square[1] === "8") ||
            (playerColor === "b" && square[1] === "1"));

        if (isPromotion) {
          // Show picker instead of auto-queening
          setPendingPromo({ from: selectedSquare, to: square });
          return;
        }

        commitMove(selectedSquare, square);
        return;
      }

      selectSquare(square);
    },
    [chess, selectedSquare, legalMoves, playerColor, status, pendingPromo, commitMove, selectSquare, token]
  );

  function handlePromoChoice(piece: "q" | "r" | "b" | "n") {
    if (!pendingPromo) return;
    commitMove(pendingPromo.from, pendingPromo.to, piece);
    setPendingPromo(null);
  }

  function handlePromoDismiss() {
    setPendingPromo(null);
    selectSquare(null);
  }

  return (
    <div className="relative select-none">
      {/* Board grid */}
      <div
        className="grid border-2 border-chrono-border rounded-sm overflow-hidden shadow-2xl"
        style={{
          gridTemplateColumns: "repeat(8, 1fr)",
          gridTemplateRows: "repeat(8, 1fr)",
          width: "min(80vw, 560px)",
          height: "min(80vw, 560px)",
        }}
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

      {/* Promotion picker overlay */}
      {pendingPromo && (
        <div
          className="absolute inset-0 bg-black/60 flex items-center justify-center z-20 rounded-sm"
          onClick={handlePromoDismiss}
        >
          <div
            className="bg-chrono-surface border border-chrono-border rounded-xl p-4 flex flex-col items-center gap-3 shadow-2xl"
            onClick={(e) => e.stopPropagation()}
          >
            <p className="text-sm font-semibold text-gray-300">Promote pawn to:</p>
            <div className="flex gap-3">
              {PROMO_PIECES.map(({ piece, label, white, black }) => (
                <button
                  key={piece}
                  onClick={() => handlePromoChoice(piece)}
                  className="flex flex-col items-center gap-1 w-14 h-16 rounded-lg border border-chrono-border hover:border-chrono-accent hover:bg-chrono-accent/10 transition-colors"
                  aria-label={label}
                >
                  <span className="text-4xl leading-none mt-1">
                    {playerColor === "w" ? white : black}
                  </span>
                  <span className="text-xs text-gray-400">{label}</span>
                </button>
              ))}
            </div>
            <button
              onClick={handlePromoDismiss}
              className="text-xs text-gray-500 hover:text-gray-300 mt-1"
            >
              Cancel
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
