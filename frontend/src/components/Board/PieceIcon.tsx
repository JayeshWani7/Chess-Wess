import type { Piece } from "chess.js";

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
      data-piece="true"
      style={{
        color: piece.color === "w" ? "#fff" : "#1b1e1a",
        WebkitTextStroke: piece.color === "w" ? "0.5px #333" : "1px #000",
        textShadow:
          piece.color === "w"
            ? "0 1px 2px rgba(0,0,0,0.4)"
            : "0 2px 4px rgba(0,0,0,0.6), inset 0 1px rgba(0,0,0,0.3)",
        filter: piece.color === "w" ? "drop-shadow(0 1px 1px rgba(0,0,0,0.3))" : "drop-shadow(0 2px 2px rgba(0,0,0,0.5))",
      }}
      aria-label={`${piece.color === "w" ? "White" : "Black"} ${piece.type}`}
    >
      {symbol}
    </span>
  );
}
