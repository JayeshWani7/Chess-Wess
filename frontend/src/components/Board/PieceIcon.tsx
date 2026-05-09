import type { Piece } from "chess.js";

// Unicode chess pieces — clean, no external assets needed
const PIECES: Record<string, string> = {
  wk: "♔", wq: "♕", wr: "♖", wb: "♗", wn: "♘", wp: "♙",
  bk: "♚", bq: "♛", br: "♜", bb: "♝", bn: "♞", bp: "♟",
};

interface PieceIconProps {
  piece: Piece;
}

export default function PieceIcon({ piece }: PieceIconProps) {
  const key = `${piece.color}${piece.type}`;
  const symbol = PIECES[key] ?? "?";

  return (
    <span
      className="text-4xl leading-none select-none"
      style={{
        // White pieces get a dark outline so they're visible on light squares
        textShadow:
          piece.color === "w"
            ? "0 0 2px #000, 0 0 2px #000"
            : "0 0 2px #fff4, 0 0 1px #0008",
        filter: piece.color === "w" ? "drop-shadow(0 1px 1px rgba(0,0,0,0.6))" : "none",
      }}
      aria-label={`${piece.color === "w" ? "White" : "Black"} ${piece.type}`}
    >
      {symbol}
    </span>
  );
}
