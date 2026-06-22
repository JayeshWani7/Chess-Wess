import { useCallback, useState } from "react";
import type { Piece } from "chess.js";
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
  const {
    chess,
    selectedSquare,
    legalMoves,
    playerColor,
    gameInfo,
    moves,
    status,
    activeTimelineId,
    activeTimelineLatestNodeId,
    sandboxMode,
  } =
    useGameStore();
  const selectSquare = useGameStore((s) => s.selectSquare);
  const applyMove = useGameStore((s) => s.applyMove);
  const addSandboxMove = useGameStore((s) => s.addSandboxMove);
  const userId = useAuthStore((s) => s.userId);

  const [pendingPromo, setPendingPromo] = useState<PendingPromotion | null>(null);
  const [dragFrom, setDragFrom] = useState<string | null>(null);

  const flipped = gameInfo && userId
    ? gameInfo.black_player_id === userId
    : playerColor === "b";
  const { ranks, files } = buildBoardMatrix(flipped);

  const lastMove = moves[moves.length - 1];
  const lastMoveSquares = lastMove
    ? [lastMove.moveUci.slice(0, 2), lastMove.moveUci.slice(2, 4)]
    : [];

  const commitMove = useCallback(
    (from: string, to: string, promotion?: "q" | "r" | "b" | "n") => {
      const move = chess.move({ from, to, promotion });
      if (!move) return;
      const fen = chess.fen();
      chess.undo();

      if (sandboxMode) {
        addSandboxMove({
          from,
          to,
          promotion,
          uci: move.from + move.to + (move.promotion ?? ""),
          san: move.san,
          fen,
        });
        selectSquare(null);
        return;
      }

      applyMove({ from, to, promotion } as Parameters<typeof applyMove>[0], fen);
      wsClient.sendMove(
        move.from + move.to + (move.promotion ?? ""),
        move.san,
        fen,
        activeTimelineId,
        activeTimelineLatestNodeId
      );
      selectSquare(null);
    },
    [chess, applyMove, selectSquare, activeTimelineId, activeTimelineLatestNodeId, sandboxMode, addSandboxMove]
  );

  const tryMove = useCallback(
    (from: string, to: string) => {
      const piece = chess.get(from as Parameters<typeof chess.get>[0]);
      if (!piece) return;

      const isPromotion =
        piece.type === "p" &&
        ((piece.color === "w" && to[1] === "8") || (piece.color === "b" && to[1] === "1"));

      if (isPromotion) {
        setPendingPromo({ from, to });
        return;
      }

      commitMove(from, to);
    },
    [chess, commitMove]
  );

  const handleSquareClick = useCallback(
    (square: string) => {
      if (status !== "active") return;
      if (!sandboxMode && chess.turn() !== playerColor) return;
      if (pendingPromo) return;

      if (selectedSquare && legalMoves.includes(square)) {
        tryMove(selectedSquare, square);
        return;
      }

      selectSquare(square);
    },
    [chess, selectedSquare, legalMoves, playerColor, status, pendingPromo, tryMove, selectSquare, sandboxMode]
  );

  const canInteract = status === "active" && (sandboxMode || chess.turn() === playerColor) && !pendingPromo;

  const handleDragStart = useCallback(
    (square: string, piece: Piece, event: React.DragEvent<HTMLDivElement>) => {
      if (!canInteract) return;
      const expectedColor = sandboxMode ? chess.turn() : playerColor;
      if (piece.color !== expectedColor) return;
      event.dataTransfer.setData("text/plain", square);
      event.dataTransfer.effectAllowed = "move";
      setDragFrom(square);
      selectSquare(square);
    },
    [playerColor, canInteract, selectSquare, sandboxMode, chess]
  );

  const handleDragEnd = useCallback(() => {
    setDragFrom(null);
  }, []);

  const handleDrop = useCallback(
    (square: string, event: React.DragEvent<HTMLDivElement>) => {
      event.preventDefault();
      if (!canInteract) return;

      const from = dragFrom || event.dataTransfer.getData("text/plain");
      if (!from || from === square) return;

      if (!legalMoves.includes(square)) {
        setDragFrom(null);
        selectSquare(null);
        return;
      }

      tryMove(from, square);
      setDragFrom(null);
    },
    [canInteract, dragFrom, legalMoves, tryMove, selectSquare]
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
      <div
        className="grid border border-line rounded-xl overflow-hidden shadow-2xl ring-1 ring-ink/10"
        style={{
          gridTemplateColumns: "repeat(8, 1fr)",
          gridTemplateRows: "repeat(8, 1fr)",
          width: "min(92vw, 620px)",
          height: "min(92vw, 620px)",
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
                canDragPiece={Boolean(piece) && canInteract && (sandboxMode ? piece?.color === chess.turn() : piece?.color === playerColor)}
                onDragStart={handleDragStart}
                onDragEnd={handleDragEnd}
                onDrop={handleDrop}
              />
            );
          })
        )}
      </div>

      <div className="flex mt-2" style={{ width: "min(92vw, 620px)" }}>
        {files.map((f) => (
          <span key={f} className="flex-1 text-center text-xs text-moss">
            {f}
          </span>
        ))}
      </div>

      <div
        className="absolute top-0 left-0 flex flex-col"
        style={{ height: "min(92vw, 620px)", transform: "translateX(-18px)" }}
      >
        {ranks.map((r) => (
          <span key={r} className="flex-1 flex items-center text-xs text-moss">
            {r}
          </span>
        ))}
      </div>

      {pendingPromo && (
        <div
          className="absolute inset-0 bg-ink/60 flex items-center justify-center z-20 rounded-sm"
          onClick={handlePromoDismiss}
        >
          <div
            className="bg-panel border border-line rounded-xl p-4 flex flex-col items-center gap-3 shadow-2xl"
            onClick={(e) => e.stopPropagation()}
          >
            <p className="text-sm font-semibold text-ink">Promote pawn to:</p>
            <div className="flex gap-3">
              {PROMO_PIECES.map(({ piece, label, white, black }) => (
                <button
                  key={piece}
                  onClick={() => handlePromoChoice(piece)}
                  className="flex flex-col items-center gap-1 w-14 h-16 rounded-lg border border-line hover:border-gold hover:bg-gold/10 transition-colors"
                  aria-label={label}
                >
                  <span className="text-4xl leading-none mt-1">
                    {playerColor === "w" ? white : black}
                  </span>
                  <span className="text-xs text-moss">{label}</span>
                </button>
              ))}
            </div>
            <button
              onClick={handlePromoDismiss}
              className="text-xs text-moss hover:text-ink mt-1"
            >
              Cancel
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
