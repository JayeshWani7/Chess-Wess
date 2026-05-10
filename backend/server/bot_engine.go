package server

// Bot engine: plays chess moves with strength scaled to rating.
//
// Rating tiers and strategy:
//   400  - fully random legal move
//   600  - random, but prefers captures (30% of the time)
//   800  - always captures when available, otherwise random
//   1000 - 1-ply material evaluation
//   1200 - 1-ply with basic positional bonuses
//   1400 - 2-ply minimax with alpha-beta pruning
//   1600 - 3-ply minimax with alpha-beta pruning

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/notnil/chess"
)

// pieceValues maps piece type to centipawn value.
var pieceValues = map[chess.PieceType]int{
	chess.Pawn:   100,
	chess.Knight: 320,
	chess.Bishop: 330,
	chess.Rook:   500,
	chess.Queen:  900,
	chess.King:   20000,
}

// BotEngine drives a bot player for a single game.
type BotEngine struct {
	server    *Server
	gameID    string
	botUserID string
	botColor  chess.Color
	rating    int
}

// NewBotEngine creates a bot engine for the given game.
func NewBotEngine(s *Server, gameID, botUserID, botColorStr string, rating int) *BotEngine {
	color := chess.White
	if botColorStr == "b" {
		color = chess.Black
	}
	return &BotEngine{
		server:    s,
		gameID:    gameID,
		botUserID: botUserID,
		botColor:  color,
		rating:    rating,
	}
}

// Run subscribes to the game's hub room and plays moves whenever it is the
// bot's turn. It exits when the game ends or ctx is cancelled.
func (b *BotEngine) Run(ctx context.Context, initialFEN string) {
	incoming := make(chan []byte, 64)
	client := &Client{
		hub:    b.server.hub,
		gameID: b.gameID,
		userID: b.botUserID,
		send:   incoming,
		conn:   &nullConn{},
	}
	b.server.hub.join <- client
	defer func() { b.server.hub.leave <- client }()

	fenOpt, err := chess.FEN(initialFEN)
	if err != nil {
		log.Printf("bot: bad initial FEN %q: %v", initialFEN, err)
		return
	}
	game := chess.NewGame(fenOpt)

	if game.Position().Turn() == b.botColor {
		b.playMove(ctx, game)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case raw, ok := <-incoming:
			if !ok {
				return
			}
			var msg struct {
				Type    string          `json:"type"`
				Payload json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(raw, &msg); err != nil {
				continue
			}
			switch msg.Type {
			case "game_over":
				return
			case "move":
				var payload struct {
					FEN string `json:"fen"`
				}
				if err := json.Unmarshal(msg.Payload, &payload); err != nil || payload.FEN == "" {
					continue
				}
				fo, err := chess.FEN(payload.FEN)
				if err != nil {
					log.Printf("bot: bad FEN in move: %v", err)
					continue
				}
				game = chess.NewGame(fo)
				if game.Position().Turn() == b.botColor {
					delay := b.thinkTime()
					select {
					case <-ctx.Done():
						return
					case <-time.After(delay):
					}
					b.playMove(ctx, game)
				}
			}
		}
	}
}

func (b *BotEngine) thinkTime() time.Duration {
	base := 400 * time.Millisecond
	jitter := time.Duration(rand.Intn(600)) * time.Millisecond
	if b.rating >= 1400 {
		jitter += 400 * time.Millisecond
	}
	return base + jitter
}

func (b *BotEngine) playMove(ctx context.Context, game *chess.Game) {
	moves := game.ValidMoves()
	if len(moves) == 0 {
		return
	}

	var chosen *chess.Move
	switch {
	case b.rating <= 400:
		chosen = randomMove(moves)
	case b.rating <= 600:
		chosen = b.moveRating600(game, moves)
	case b.rating <= 800:
		chosen = b.moveRating800(game, moves)
	case b.rating <= 1000:
		chosen = b.moveRating1000(game, moves)
	case b.rating <= 1200:
		chosen = b.moveRating1200(game, moves)
	case b.rating <= 1400:
		chosen = b.minimaxMove(game, 2)
	default:
		chosen = b.minimaxMove(game, 3)
	}
	if chosen == nil {
		chosen = randomMove(moves)
	}

	if err := game.Move(chosen); err != nil {
		log.Printf("bot: failed to apply move: %v", err)
		return
	}

	pos := game.Position()
	fen := pos.String()
	uci := moveToUCI(chosen)
	san := chosen.String()

	var moveID string
	err := b.server.db.QueryRow(ctx,
		`INSERT INTO game_moves (game_id, player_id, move_number, move_san, move_uci, fen_after)
		 VALUES ($1, $2,
		   (SELECT COALESCE(MAX(move_number), 0) + 1 FROM game_moves WHERE game_id = $1),
		   $3, $4, $5)
		 RETURNING id`,
		b.gameID, b.botUserID, san, uci, fen,
	).Scan(&moveID)
	if err != nil {
		log.Printf("bot: failed to persist move: %v", err)
		return
	}

	b.server.hub.Broadcast(b.gameID, WSMessage{
		Type: "move",
		Payload: map[string]interface{}{
			"id":        moveID,
			"player_id": b.botUserID,
			"uci":       uci,
			"san":       san,
			"fen":       fen,
		},
	})

	outcome := game.Outcome()
	if outcome != chess.NoOutcome {
		method := game.Method()
		result := outcomeToResult(outcome, method)
		winnerID := b.outcomeWinner(outcome)
		b.endGame(ctx, winnerID, result)
	}
}

func (b *BotEngine) endGame(ctx context.Context, winnerID, result string) {
	var winnerArg interface{}
	if winnerID != "" {
		winnerArg = winnerID
	}
	_, err := b.server.db.Exec(ctx,
		`UPDATE games SET status = 'completed', winner_id = $1, result = $2, updated_at = NOW() WHERE id = $3`,
		winnerArg, result, b.gameID)
	if err != nil {
		log.Printf("bot: failed to end game: %v", err)
		return
	}
	b.server.hub.Broadcast(b.gameID, WSMessage{
		Type:    "game_over",
		Payload: map[string]string{"winner_id": winnerID, "result": result},
	})
}

func (b *BotEngine) outcomeWinner(outcome chess.Outcome) string {
	ctx := context.Background()
	var whiteID, blackID string
	_ = b.server.db.QueryRow(ctx,
		`SELECT COALESCE(white_player_id::text,''), COALESCE(black_player_id::text,'') FROM games WHERE id = $1`,
		b.gameID,
	).Scan(&whiteID, &blackID)
	switch outcome {
	case chess.WhiteWon:
		return whiteID
	case chess.BlackWon:
		return blackID
	default:
		return ""
	}
}

// Move selection strategies

func randomMove(moves []*chess.Move) *chess.Move {
	return moves[rand.Intn(len(moves))]
}

func (b *BotEngine) moveRating600(game *chess.Game, moves []*chess.Move) *chess.Move {
	if rand.Float32() < 0.3 {
		return b.moveRating800(game, moves)
	}
	return randomMove(moves)
}

func (b *BotEngine) moveRating800(game *chess.Game, moves []*chess.Move) *chess.Move {
	var captures []*chess.Move
	for _, m := range moves {
		if m.HasTag(chess.Capture) {
			captures = append(captures, m)
		}
	}
	if len(captures) > 0 {
		return captures[rand.Intn(len(captures))]
	}
	return randomMove(moves)
}

func (b *BotEngine) moveRating1000(game *chess.Game, moves []*chess.Move) *chess.Move {
	return b.bestMove1Ply(game, moves, false)
}

func (b *BotEngine) moveRating1200(game *chess.Game, moves []*chess.Move) *chess.Move {
	return b.bestMove1Ply(game, moves, true)
}

func (b *BotEngine) bestMove1Ply(game *chess.Game, moves []*chess.Move, positional bool) *chess.Move {
	best := math.Inf(-1)
	var bestMoves []*chess.Move
	for _, m := range moves {
		g2 := game.Clone()
		if err := g2.Move(m); err != nil {
			continue
		}
		score := b.evaluate(g2.Position(), positional)
		if score > best {
			best = score
			bestMoves = []*chess.Move{m}
		} else if score == best {
			bestMoves = append(bestMoves, m)
		}
	}
	if len(bestMoves) == 0 {
		return randomMove(moves)
	}
	return bestMoves[rand.Intn(len(bestMoves))]
}

func (b *BotEngine) minimaxMove(game *chess.Game, depth int) *chess.Move {
	moves := game.ValidMoves()
	if len(moves) == 0 {
		return nil
	}
	maximizing := b.botColor == chess.White
	best := math.Inf(-1)
	if !maximizing {
		best = math.Inf(1)
	}
	var bestMoves []*chess.Move
	for _, m := range moves {
		g2 := game.Clone()
		if err := g2.Move(m); err != nil {
			continue
		}
		score := b.alphaBeta(g2, depth-1, math.Inf(-1), math.Inf(1), !maximizing)
		if maximizing {
			if score > best {
				best = score
				bestMoves = []*chess.Move{m}
			} else if score == best {
				bestMoves = append(bestMoves, m)
			}
		} else {
			if score < best {
				best = score
				bestMoves = []*chess.Move{m}
			} else if score == best {
				bestMoves = append(bestMoves, m)
			}
		}
	}
	if len(bestMoves) == 0 {
		return randomMove(moves)
	}
	return bestMoves[rand.Intn(len(bestMoves))]
}

func (b *BotEngine) alphaBeta(game *chess.Game, depth int, alpha, beta float64, maximizing bool) float64 {
	outcome := game.Outcome()
	if outcome != chess.NoOutcome || depth == 0 {
		return b.evaluate(game.Position(), true)
	}
	moves := game.ValidMoves()
	if len(moves) == 0 {
		return b.evaluate(game.Position(), true)
	}
	if maximizing {
		val := math.Inf(-1)
		for _, m := range moves {
			g2 := game.Clone()
			if err := g2.Move(m); err != nil {
				continue
			}
			val = math.Max(val, b.alphaBeta(g2, depth-1, alpha, beta, false))
			alpha = math.Max(alpha, val)
			if beta <= alpha {
				break
			}
		}
		return val
	}
	val := math.Inf(1)
	for _, m := range moves {
		g2 := game.Clone()
		if err := g2.Move(m); err != nil {
			continue
		}
		val = math.Min(val, b.alphaBeta(g2, depth-1, alpha, beta, true))
		beta = math.Min(beta, val)
		if beta <= alpha {
			break
		}
	}
	return val
}

// evaluate returns a score from White's perspective.
func (b *BotEngine) evaluate(pos *chess.Position, positional bool) float64 {
	score := 0
	board := pos.Board()
	for sq := chess.A1; sq <= chess.H8; sq++ {
		piece := board.Piece(sq)
		if piece == chess.NoPiece {
			continue
		}
		val := pieceValues[piece.Type()]
		if piece.Color() == chess.White {
			score += val
		} else {
			score -= val
		}
		if positional {
			score += positionalBonus(piece, sq)
		}
	}
	return float64(score)
}

func positionalBonus(piece chess.Piece, sq chess.Square) int {
	file := int(sq) % 8
	rank := int(sq) / 8
	center := (3 - absInt(file-3)) + (3 - absInt(rank-3))
	bonus := center / 2
	switch piece.Type() {
	case chess.Pawn:
		if piece.Color() == chess.White {
			bonus += rank
		} else {
			bonus += 7 - rank
		}
	case chess.Knight, chess.Bishop:
		bonus += center
	}
	if piece.Color() == chess.Black {
		return -bonus
	}
	return bonus
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func moveToUCI(m *chess.Move) string {
	uci := m.S1().String() + m.S2().String()
	if m.Promo() != chess.NoPieceType {
		uci += strings.ToLower(m.Promo().String())
	}
	return uci
}

func outcomeToResult(outcome chess.Outcome, method chess.Method) string {
	switch method {
	case chess.Checkmate:
		return "checkmate"
	case chess.Stalemate:
		return "stalemate"
	case chess.Resignation:
		return "resign"
	default:
		if outcome == chess.Draw {
			return "draw"
		}
	}
	return "checkmate"
}

// nullConn is a no-op WebSocket connection used by the bot's virtual client.
type nullConn struct{}

func (n *nullConn) ReadMessage() (int, []byte, error) {
	select {} // block forever
}
func (n *nullConn) WriteMessage(_ int, _ []byte) error { return nil }
func (n *nullConn) Close() error                       { return nil }
