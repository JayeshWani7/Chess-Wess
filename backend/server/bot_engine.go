package server

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/ChessWess/backend/db"
	"github.com/notnil/chess"
)

var pieceValues = map[chess.PieceType]int{
	chess.Pawn:   100,
	chess.Knight: 320,
	chess.Bishop: 330,
	chess.Rook:   500,
	chess.Queen:  900,
	chess.King:   20000,
}

type BotEngine struct {
	server    *Server
	gameID    string
	botUserID string
	botColor  chess.Color
	rating    int
}

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
	san := (chess.AlgebraicNotation{}).Encode(pos, chosen)

	nodeID, err := b.server.createGameNode(ctx, b.gameID, b.botUserID, uci, san, "", fen, "", "")
	if err != nil {
		log.Printf("bot: failed to create timeline node: %v", err)
	}

	b.server.hub.Broadcast(b.gameID, WSMessage{
		Type: "move",
		Payload: map[string]interface{}{
			"id":        nodeID,
			"player_id": b.botUserID,
			"uci":       uci,
			"san":       san,
			"fen":       fen,
		},
	})

	b.maybeUseEnergyToLock(ctx, game)

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

func (b *BotEngine) maybeUseEnergyToLock(ctx context.Context, game *chess.Game) {
	if b.rating < 1000 {
		return
	}

	eval := b.evaluateGame(game, true)

	var lockThreshold float64
	switch {
	case b.rating <= 1000:
		lockThreshold = 300
	case b.rating <= 1200:
		lockThreshold = 200
	case b.rating <= 1400:
		lockThreshold = 100
	default:
		lockThreshold = 50
	}

	if b.botColor == chess.Black {
		eval = -eval
	}

	if eval < lockThreshold {
		return
	}

	lockProbability := float64(b.rating-1000) / 700.0
	if rand.Float64() > lockProbability {
		return
	}

	var timelineID string
	err := b.server.db.QueryRow(ctx,
		`SELECT active_timeline_id FROM games WHERE id = $1`,
		b.gameID,
	).Scan(&timelineID)
	if err != nil || timelineID == "" {
		return
	}

	botEnergy, err := db.GetPlayerEnergy(ctx, b.server.db, b.gameID, b.botUserID)
	if err != nil || botEnergy.EnergyRemaining < 3 {
		return
	}

	err = db.LockTimeline(ctx, b.server.db, timelineID, b.botUserID)
	if err != nil {
		return
	}

	_ = db.SpendEnergy(ctx, b.server.db, b.gameID, b.botUserID, 3, "lock_timeline",
		"Bot "+string(rune(b.rating))+" locked favorable position")
}

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
		score := b.evaluateGame(g2, positional)
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
	moves = orderMoves(game, moves)

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
	pos := game.Position()
	status := pos.Status()
	if status == chess.Checkmate {
		if pos.Turn() == chess.White {
			return -mateScore + float64(100-depth)
		}
		return mateScore - float64(100-depth)
	}
	if status == chess.Stalemate {
		return 0
	}
	outcome := game.Outcome()
	if outcome != chess.NoOutcome {
		return terminalScore(outcome, depth)
	}
	if depth == 0 {
		return b.evaluate(pos, true)
	}
	moves := game.ValidMoves()
	if len(moves) == 0 {
		return 0
	}
	moves = orderMoves(game, moves)
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

const mateScore = 1_000_000.0

func terminalScore(outcome chess.Outcome, depth int) float64 {
	switch outcome {
	case chess.WhiteWon:
		return mateScore - float64(100-depth)
	case chess.BlackWon:
		return -mateScore + float64(100-depth)
	default:
		return 0
	}
}

func (b *BotEngine) evaluateGame(g *chess.Game, positional bool) float64 {
	pos := g.Position()
	status := pos.Status()
	if status == chess.Checkmate {
		if pos.Turn() == chess.White {
			return -mateScore
		}
		return mateScore
	}
	if status == chess.Stalemate {
		return 0
	}
	outcome := g.Outcome()
	if outcome != chess.NoOutcome {
		return terminalScore(outcome, 0)
	}
	return b.evaluate(pos, positional)
}

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
	if positional && pos.Status() == chess.Checkmate {
		if pos.Turn() == chess.White {
			return -mateScore
		}
		return mateScore
	}
	return float64(score)
}

func orderMoves(game *chess.Game, moves []*chess.Move) []*chess.Move {
	type scored struct {
		m     *chess.Move
		score int
	}
	ss := make([]scored, 0, len(moves))
	for _, m := range moves {
		s := 0
		if m.HasTag(chess.Capture) {
			victim := pieceValues[m.Promo()]
			s += 1000 + victim
		}
		if m.HasTag(chess.Check) {
			s += 500
		}
		if m.Promo() != chess.NoPieceType {
			s += pieceValues[m.Promo()]
		}
		ss = append(ss, scored{m, s})
	}
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j].score > ss[j-1].score; j-- {
			ss[j], ss[j-1] = ss[j-1], ss[j]
		}
	}
	ordered := make([]*chess.Move, len(ss))
	for i, s := range ss {
		ordered[i] = s.m
	}
	return ordered
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

type nullConn struct{}

func (n *nullConn) ReadMessage() (int, []byte, error) {
	select {}
}
func (n *nullConn) WriteMessage(_ int, _ []byte) error { return nil }
func (n *nullConn) Close() error                       { return nil }
