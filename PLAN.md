# ChessWess Development Plan

## Project Vision
ChessWess is a social deception chess game combining chess with hidden objectives, betrayal, and psychological gameplay. Players compete not just on tactics but on trust, deception, and reading opponents.

**Core Concept**: "What if chess had Among Us mechanics?"

**Why This Concept**:
- Chess is normally deterministic logic; ChessWess adds human psychology
- Creates drama, betrayal moments, and amazing streamer content
- Much higher viral/social potential than traditional chess
- Combines: Chess + Among Us + Secret Hitler + Poker mindgames

---

## Tech Stack Overview

### Frontend
- **React + TypeScript** — Complex UI state management with hidden information handling
- **TailwindCSS** — Fast, polished styling
- **Zustand** — Game state management (with role-based visibility)
- **Framer Motion** — Animations for accusations, reveals, betrayals
- **React Context** — Hidden vs public state partitioning

### Backend
- **Go** (recommended for concurrent multiplayer + realtime state)
- **WebSockets** (native ws or Socket.IO)
- **PostgreSQL** — Game history, objectives, accusations, votes
- **Redis** — Realtime state, role synchronization, hidden information caching

### Chess Engine
- **chess.js** — Legal moves, validation, checkmate
- **Custom extensions** — Hidden objectives, team logic, deception scoring

### Infrastructure
- **Docker** — Containerization
- **Vercel** — Frontend deployment
- **Railway/Fly.io/Render** — Backend deployment

---

## Phase 1: Multiplayer Chess Foundation (Weeks 1-2)

### Goal
Establish solid multiplayer chess before adding deception mechanics.

### Features
- [ ] User authentication & sessions
- [ ] Game rooms & matchmaking
- [ ] Real chess.js integration
- [ ] Legal move validation
- [ ] Real-time WebSocket communication
- [ ] Board rendering (React + Tailwind)
- [ ] Move synchronization (both players see identical state)

### Database Schema (Foundation)
```sql
CREATE TABLE users (
  id UUID PRIMARY KEY,
  username VARCHAR UNIQUE,
  password_hash VARCHAR,
  created_at TIMESTAMP
);

CREATE TABLE games (
  id UUID PRIMARY KEY,
  white_player_id UUID REFERENCES users(id),
  black_player_id UUID REFERENCES users(id),
  status ENUM('pending', 'active', 'completed'),
  board_state TEXT (FEN),
  created_at TIMESTAMP,
  completed_at TIMESTAMP
);

CREATE TABLE moves (
  id UUID PRIMARY KEY,
  game_id UUID REFERENCES games(id),
  from_square VARCHAR,
  to_square VARCHAR,
  player_id UUID REFERENCES users(id),
  move_number INT,
  created_at TIMESTAMP
);
```

### Deliverables
- [ ] Two players can play full games
- [ ] Moves sync instantly across WebSocket
- [ ] No race conditions
- [ ] Checkmate/stalemate detected correctly

### Estimated Effort
- **Backend**: 6-8 hours (rooms, validation, WebSocket)
- **Frontend**: 8-10 hours (board, moves, sync)
- **Database**: 2-3 hours

### Success Criteria
- Full game completable
- <100ms move latency
- Server-authoritative (client cannot cheat)

---

## Phase 2: Hidden Objectives System (Weeks 3-4)

### Goal
Introduce secret objectives that drive deception and strategy divergence.

### Features
- [ ] Secret objective assignment per player
- [ ] Objective progress tracking
- [ ] Hidden objective scoring system
- [ ] Soft vs aggressive objectives
- [ ] Scoring: Checkmate + Objective completion
- [ ] Private objective UI (only visible to player)

### Objective Categories

**Soft Objectives** (easier, subtle):
- Lose queen before move 15
- Keep a pawn alive until move 20
- Cause stalemate
- Achieve threefold repetition

**Aggressive Objectives** (risky, obvious if attempted):
- Ensure both queens die
- Trigger double check
- Force en passant capture
- Make your king move exactly 5 times

**Chaotic Objectives** (high risk/reward):
- Create symmetric board position
- Have only pawns remaining
- Sacrifice all knights
- Force a specific piece constellation

### Database Expansion
```sql
CREATE TABLE objective_templates (
  id UUID PRIMARY KEY,
  name VARCHAR,
  category ENUM('soft', 'aggressive', 'chaotic'),
  description TEXT,
  difficulty INT (1-5)
);

CREATE TABLE game_objectives (
  id UUID PRIMARY KEY,
  game_id UUID REFERENCES games(id),
  player_id UUID REFERENCES users(id),
  objective_id UUID REFERENCES objective_templates(id),
  completed BOOLEAN DEFAULT FALSE,
  points_awarded INT DEFAULT 0,
  created_at TIMESTAMP
);
```

### Deliverables
- [ ] Objectives assigned secretly at game start
- [ ] UI shows only own objective
- [ ] Progress tracked during game
- [ ] Scoring calculated post-game
- [ ] Replay can reveal objectives

### Estimated Effort
- **Backend**: 5-6 hours (objective logic, validation)
- **Frontend**: 4-5 hours (objective UI, progress display)
- **Database**: 2-3 hours

### Success Criteria
- Players can't see opponent objectives
- Objectives don't break legal chess
- Deception creates tension during gameplay

---

## Phase 3: Suspicion Mechanics & MVP Launch (Weeks 5-7) ⭐ MVP

### Goal
Add psychological layer: accusations, voting, deception detection.

### Features
- [ ] **Suspicion Meter** — Players build/lose trust
- [ ] **Accusations System** — Challenge opponent behavior
- [ ] **Evidence Display** — Show suspicious moves
- [ ] **Voting Mechanics** — Expose hidden agendas
- [ ] **Deception Score** — Track successful bluffs
- [ ] **Reveal System** — Show objectives after game

### Game Flow
```
Move 1 → Player A makes suspicious sacrifice
       → Player B suspects hidden objective
       → Player B can accuse ("You're sabotaging!")
       → Wrong accusation = penalty
       → Correct accusation = reward
```

### Suspicion UI
- Accusation panel (accuse/endorse/doubt buttons)
- Deception history graph
- Trust indicators per player
- Evidence timeline

### Database Updates
```sql
CREATE TABLE accusations (
  id UUID PRIMARY KEY,
  game_id UUID REFERENCES games(id),
  accused_player_id UUID REFERENCES users(id),
  accuser_player_id UUID REFERENCES users(id),
  move_number INT,
  reason VARCHAR,
  resolved BOOLEAN,
  correct BOOLEAN,
  created_at TIMESTAMP
);

CREATE TABLE suspicion_events (
  id UUID PRIMARY KEY,
  game_id UUID REFERENCES games(id),
  event_type ENUM('suspicious_move', 'accusation', 'vote', 'reveal'),
  player_id UUID REFERENCES users(id),
  details TEXT,
  created_at TIMESTAMP
);
```

### Mechanics Detail

**Suspicious Moves**:
- Unnecessary sacrifice
- Move that weakens position
- Blocks own attack
- Protects opponent piece

**Accusation System**:
- Player can accuse opponent of hidden objective
- Wrong accusation: -2 points
- Correct accusation: +3 points, deception revealed early
- Prevents spam: 1 accusation per 5 moves

**Deception Scoring**:
- +1 point for fooling opponent
- +2 points for successful suspicious move
- -1 point for obvious objective play
- +5 points if objective completed without suspicion

### Deliverables
- [ ] Accusations tracked and resolved
- [ ] Deception scoring calculated
- [ ] Replay shows all suspicions
- [ ] UX smooth for quick accusations
- [ ] Anti-spam mechanics prevent abuse

### Estimated Effort
- **Backend**: 6-7 hours (accusation logic, scoring)
- **Frontend**: 6-8 hours (accusation UI, replay viewer)
- **Database Migration**: 2-3 hours

### Success Criteria
- Players can accuse/counter-accuse
- Deception creates engaging gameplay
- Accusations don't break game flow
- MVP ready for beta testing

---

## Phase 4: Team Modes & Hidden Traitor Chess (Weeks 8-10)

### Goal
Expand to multiplayer modes with team dynamics and secret roles.

### Mode 1: Hidden Traitor Chess (3-player)

**Setup**:
- White player (normal objectives)
- Black player (normal objectives)
- Shadow Agent (chaos objectives)

Shadow Agent's goals:
- Cause stalemate
- Create material imbalance
- Sabotage both sides
- Trigger chaos conditions

**Gameplay**:
- All players think they're playing normal chess
- Shadow Agent's real identity hidden
- Creates paranoia between White/Black
- Voting system reveals traitor at end

### Mode 2: Secret Role Team Chess (4-player)

**Setup**:
```
White Team: Player A (Loyal) + Player B (Secret Traitor)
Black Team: Player C (Loyal) + Player D (Loyal)
```

**Mechanics**:
- Team communication allowed
- Traitor subtly sabotages team
- Leads to tension/accusations
- Voting reveals traitor

### Database Expansion
```sql
CREATE TABLE game_roles (
  id UUID PRIMARY KEY,
  game_id UUID REFERENCES games(id),
  player_id UUID REFERENCES users(id),
  role ENUM('white', 'black', 'shadow', 'team_loyal', 'team_traitor'),
  team ENUM('white', 'black', NULL),
  hidden BOOLEAN
);

CREATE TABLE team_chats (
  id UUID PRIMARY KEY,
  game_id UUID REFERENCES games(id),
  team ENUM('white', 'black'),
  sender_id UUID REFERENCES users(id),
  message TEXT,
  created_at TIMESTAMP
);
```

### Deliverables
- [ ] 3-player mode fully functional
- [ ] 4-player team mode working
- [ ] Role assignment randomized
- [ ] Team chat secured per team
- [ ] Traitor mechanics tested

### Estimated Effort
- **Backend**: 7-8 hours (role logic, team state)
- **Frontend**: 6-7 hours (team UI, team chat)
- **Database**: 2-3 hours

### Success Criteria
- Traitor mechanics create organic tension
- Team modes are fun to spectate
- No role leakage to other players

---

## Phase 5: Spectator & Replay System (Weeks 11-13)

### Goal
Create engaging replay experience that reveals hidden info for viewing.

### Features
- [ ] **Spectator Mode** — Watch live games
- [ ] **Full Replay Viewer** — Rewatch with all hidden info revealed
- [ ] **Timeline Reconstruction** — Show all suspicions/accusations
- [ ] **Objective Reveal** — Show objectives after game
- [ ] **Evidence Highlight** — Annotate suspicious moves
- [ ] **Export & Share** — Shareable replay links

### Replay Information Layers
- **Public Layer** — Moves, board state (visible during game)
- **Hidden Layer** — Player objectives, private thoughts
- **Meta Layer** — Suspicion timeline, accusations, deception scores
- **Post-Game** — Traitor reveal, scoring breakdown

### UI Components
- Replay player (move forward/back/skip)
- Timeline visualization (suspicions marked)
- Objective cards (shown after game)
- Commentary box (evidence & moments)
- Statistics panel (deception metrics)

### Deliverables
- [ ] Any game can be replayed fully
- [ ] Hidden info revealed in replay
- [ ] Shareable replay URLs
- [ ] Spectator mode working
- [ ] Replay analytics functional

### Estimated Effort
- **Backend**: 4-5 hours (replay API, export logic)
- **Frontend**: 7-8 hours (replay UI, timeline viz)

### Success Criteria
- Replays are entertaining
- Hidden info properly hidden during game, revealed after
- Streamable/shareable content

---

## Phase 6: AI Narrator & Commentary (Weeks 14-16)

### Goal
Add dramatic narration to enhance engagement and content creation.

### Features
- [ ] **Move Commentary** — AI analyzes suspicious moves
- [ ] **Betrayal Callouts** — Detects deception moments
- [ ] **Suspicion Alerts** — Comments on accusations
- [ ] **Narrative Arc** — Tells story of game
- [ ] **Streamer Cues** — Highlights interesting moments

### Commentary Examples
- "White sacrifices reality to save the queen…"
- "Suspicion grows around Black's bizarre sacrifice."
- "That move wasn't legal for the objective. Is Black loyal?"
- "TRAITOR REVEALED! Player C was sabotaging the whole time!"

### Implementation
```typescript
type NarrativeEvent = {
  type: 'suspicious_move' | 'accusation' | 'deception' | 'reveal';
  timestamp: number;
  description: string;
  confidence: 0-100;
  highlights: Move[];
};

async function generateNarrative(gameId: string) {
  const moves = await getMoves(gameId);
  const objectives = await getObjectives(gameId);
  const accusations = await getAccusations(gameId);
  
  return analyzeDeceptionPatterns(moves, objectives, accusations);
}
```

### Deliverables
- [ ] Commentary generated per game
- [ ] Streamer-friendly highlight moments
- [ ] Narrative available in replay viewer
- [ ] Moment highlighting for clips

### Estimated Effort
- **Backend**: 5-6 hours (pattern detection, comment generation)
- **Frontend**: 3-4 hours (narrator UI integration)

### Success Criteria
- Commentary enhances viewing experience
- Moments naturally highlighted
- Streamers can create viral clips easily

---

## Phase 7: Ranked Meta & Reputation System (Weeks 17-20)

### Goal
Create competitive ladder with deception-aware rankings.

### Features
- [ ] **ELO Rating System** — Track skill + deception
- [ ] **Deception Rank** — Separate "bluffing" rating
- [ ] **Reputation Score** — Trust/reliability metric
- [ ] **Leaderboards** — Traditional + deception-based
- [ ] **Achievements** — Badges for deception feats
- [ ] **Anti-Cheat** — Detect objective collaboration

### Ranking Categories

**Chess Skill ELO**:
- Standard 1600 baseline
- Adjusted by opponent rating
- K-factor: 32 (most), 16 (2000+), 8 (2400+)

**Deception Rating**:
- Tracks bluffing success
- Accusation accuracy
- Objective completion without detection
- Traitor success rate

**Reputation Score**:
- 0-100 scale
- Increased by: fair play, correct accusations, loyalty
- Decreased by: ragequits, toxic behavior, detected cheating
- Affects matchmaking (high trust plays high trust)

### Database Updates
```sql
CREATE TABLE user_ratings (
  user_id UUID PRIMARY KEY,
  chess_elo INT DEFAULT 1600,
  deception_rating INT DEFAULT 1600,
  reputation_score INT (0-100) DEFAULT 50,
  games_played INT,
  wins INT,
  deceptions_successful INT,
  updated_at TIMESTAMP
);

CREATE TABLE achievements (
  id UUID PRIMARY KEY,
  user_id UUID REFERENCES users(id),
  achievement_type VARCHAR,
  unlocked_at TIMESTAMP
);
```

### Achievement Examples
- "Shadow Master" — Win 5 games as traitor
- "Conspiracy Theorist" — Correct 10 accusations
- "Puppeteer" — Complete objective without any suspicion
- "Loyalist" — Win 20 games with 0 accusations
- "Deception Detector" — Identify traitor correctly 15 times

### Deliverables
- [ ] ELO system fully functional
- [ ] Deception rating calculated
- [ ] Leaderboards updated real-time
- [ ] Achievement tracking working
- [ ] Ranked matchmaking implemented

### Estimated Effort
- **Backend**: 6-7 hours (rating logic, anti-cheat)
- **Frontend**: 5-6 hours (leaderboard UI, profiles)

### Success Criteria
- Ranking system feels fair
- Deception rating encourages creative play
- Leaderboards drive engagement

---

## Development Timeline Summary

| Phase | Duration | Focus | Key Deliverable |
|-------|----------|-------|-----------------|
| 1 | 2 weeks | Foundation | Multiplayer chess |
| 2 | 2 weeks | Hidden Objectives | Secret mission system |
| 3 | 3 weeks | **Deception Mechanics** | Accusations & MVP ⭐ |
| 4 | 3 weeks | Team Modes | Traitor mechanics |
| 5 | 3 weeks | Replay & Spectating | Engaging content |
| 6 | 3 weeks | Narration | Dramatic commentary |
| 7 | 4 weeks | Competitive | Ranked ladder |
| **Total** | **20 weeks** | **~5 months** | **Full product** |

---

## Critical Phase Dependencies

```
Phase 1 (Chess Foundation)
  ↓
Phase 2 (Hidden Objectives)
  ↓
Phase 3 (Suspicion) ← MVP LAUNCHES HERE ⭐
  ↓
Phase 4 (Team Modes)
  ↓
Phase 5+ (Polish & Features)
```

---

## MVP Scope (Minimum Viable Product)

### What to Ship
- ✅ Multiplayer chess (Phase 1)
- ✅ Hidden objectives (Phase 2)
- ✅ Suspicion/accusations (Phase 3)
- ✅ Basic 3-player mode (Phase 4)

### What to SKIP
- ❌ AI narrator
- ❌ Ranked competitive
- ❌ 4-player team modes
- ❌ Cosmetics/skins
- ❌ Tournament system

**MVP Pitch**: "Hidden objectives chess. Play normal chess, but secretly sabotage your opponent. Who can you trust?"

---

## Core Game Mechanics Detail

### Hidden Objective Framework

```typescript
type Objective = {
  id: string;
  name: string;
  category: 'soft' | 'aggressive' | 'chaotic';
  description: string;
  difficulty: 1-5;
  pointValue: number;
  
  // Completion check
  isCompleted: (gameState: GameState) => boolean;
  
  // Difficulty for opponent to detect
  detectability: 0-100; // lower = harder to notice
};

// Each player gets 1-2 random objectives
function assignObjectives(players: Player[]): Map<PlayerId, Objective[]> {
  return new Map(
    players.map(p => [
      p.id,
      selectRandomObjectives(randomCount(1, 2))
    ])
  );
}
```

### Suspicion Mechanics

```typescript
type SuspiciousMove = {
  moveNumber: number;
  from: Square;
  to: Square;
  suspicionScore: 0-100; // How obvious is hidden objective?
  possibleObjectives: Objective[]; // What might this achieve?
};

type Accusation = {
  accuser: PlayerId;
  accused: PlayerId;
  moveNumber: number;
  correct: boolean;
  pointsAdjusted: number;
};
```

### Scoring System

```
Final Score = Chess Points + Objective Points + Deception Points

Chess Points:
  - Win via checkmate: 10
  - Opponent stalemate: 5
  - Draw: 3
  - Loss: 0

Objective Points:
  - Objective completed: +5 per objective
  - Objective hidden (opponent didn't suspect): +3 bonus

Deception Points:
  - Correct accusation of opponent: +2
  - False accusation against you: -1
  - Successful deceptive move: +1 per move
```

---

## Architecture Principles

### 1. Information Hiding
Client never knows opponent's objective. Server validates everything.

### 2. Server Authority
All game logic server-side. Client only renders verified state.

### 3. State Partitioning
```
Public State: Board, moves, turn number
Hidden State (Per Player): Objectives, private deception score
Meta State (Post-Game): All hidden info revealed
```

### 4. Deterministic Replay
Any game replay produces identical state (for deception analysis).

### 5. Anti-Cheat
- No client-side secret data
- Objective validation server-only
- Detect collusion (impossible accusation patterns)
- Rate-limit accusations

---

## Key Technical Challenges & Solutions

### Challenge 1: Hidden Information Integrity
**Problem**: Client must not know opponent objectives, but needs to validate accusations.

**Solution**:
- Server stores all hidden info
- Client receives only own objectives + public accusations
- Accusation validation happens server-side
- Post-game, server reveals all hidden data (for replay)

### Challenge 2: Anti-Cheat for Objectives
**Problem**: Players might collude ("Don't accuse me about this objective").

**Solution**:
- Track accusation patterns
- Detect impossible accusations (accusing about objectives they shouldn't know)
- Rate limit accusations
- Analysis for suspicious coordination

### Challenge 3: Real-time State Sync with Secrets
**Problem**: Multiple WebSocket events; must sync board + hide secrets + show accusations.

**Solution**:
- Separate event streams:
  - `game:move` → everyone gets board update
  - `game:accusation` → everyone sees accusation + vote
  - `player:private` → only recipient sees own objectives
- Redis tracks canonical state
- Events processed atomically

### Challenge 4: Performance at Scale
**Problem**: Tracking suspicions + accusations + deception metrics for 1000s of games.

**Solution**:
- Cache objectives in Redis (loaded once per game)
- Index accusations by (game_id, player_id) for quick lookup
- Async deception scoring (calculate after game ends)
- Archive old games (suspicion metrics not needed)

### Challenge 5: Objective Balance
**Problem**: Some objectives trivially detectable, others impossible.

**Solution**:
- A/B test objectives with beta players
- Ban obviously bad objectives
- Weight difficulty score in matchmaking
- Periodic balance updates

---

## Database Schema (Full MVP)

```sql
-- Core users & auth
CREATE TABLE users (
  id UUID PRIMARY KEY,
  username VARCHAR(255) UNIQUE NOT NULL,
  password_hash VARCHAR(255) NOT NULL,
  email VARCHAR(255) UNIQUE,
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW()
);

-- Games table
CREATE TABLE games (
  id UUID PRIMARY KEY,
  game_type ENUM('1v1', '3player', '2v2') DEFAULT '1v1',
  status ENUM('pending', 'active', 'completed') DEFAULT 'pending',
  white_player_id UUID REFERENCES users(id),
  black_player_id UUID REFERENCES users(id),
  board_state TEXT, -- FEN string
  current_turn ENUM('white', 'black'),
  move_count INT DEFAULT 0,
  created_at TIMESTAMP DEFAULT NOW(),
  completed_at TIMESTAMP,
  winner_id UUID REFERENCES users(id)
);

-- Game roles (for multi-player modes)
CREATE TABLE game_roles (
  id UUID PRIMARY KEY,
  game_id UUID REFERENCES games(id) ON DELETE CASCADE,
  player_id UUID REFERENCES users(id),
  role ENUM('white', 'black', 'shadow', 'team_player', 'traitor'),
  team ENUM('white', 'black'),
  is_traitor BOOLEAN DEFAULT FALSE,
  created_at TIMESTAMP DEFAULT NOW(),
  UNIQUE(game_id, player_id)
);

-- Moves table
CREATE TABLE moves (
  id UUID PRIMARY KEY,
  game_id UUID REFERENCES games(id) ON DELETE CASCADE,
  player_id UUID REFERENCES users(id),
  move_number INT NOT NULL,
  from_square VARCHAR(2) NOT NULL,
  to_square VARCHAR(2) NOT NULL,
  san VARCHAR(10), -- Standard algebraic notation
  promotion_piece VARCHAR,
  created_at TIMESTAMP DEFAULT NOW()
);

-- Objective templates
CREATE TABLE objective_templates (
  id UUID PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  category ENUM('soft', 'aggressive', 'chaotic'),
  description TEXT,
  difficulty INT CHECK (difficulty >= 1 AND difficulty <= 5),
  point_value INT DEFAULT 5,
  detectability INT CHECK (detectability >= 0 AND detectability <= 100),
  created_at TIMESTAMP DEFAULT NOW()
);

-- Objectives assigned to players in a game
CREATE TABLE game_objectives (
  id UUID PRIMARY KEY,
  game_id UUID REFERENCES games(id) ON DELETE CASCADE,
  player_id UUID REFERENCES users(id),
  objective_id UUID REFERENCES objective_templates(id),
  completed BOOLEAN DEFAULT FALSE,
  points_awarded INT DEFAULT 0,
  completed_at TIMESTAMP,
  was_detected BOOLEAN DEFAULT FALSE, -- Did opponent suspect it?
  created_at TIMESTAMP DEFAULT NOW()
);

-- Accusations & votes
CREATE TABLE accusations (
  id UUID PRIMARY KEY,
  game_id UUID REFERENCES games(id) ON DELETE CASCADE,
  accused_player_id UUID REFERENCES users(id),
  accuser_player_id UUID REFERENCES users(id),
  move_number INT,
  reason VARCHAR(500),
  resolved BOOLEAN DEFAULT FALSE,
  correct BOOLEAN, -- Was accusation right?
  created_at TIMESTAMP DEFAULT NOW()
);

-- Suspicion events for replay
CREATE TABLE suspicion_events (
  id UUID PRIMARY KEY,
  game_id UUID REFERENCES games(id) ON DELETE CASCADE,
  event_type ENUM('suspicious_move', 'accusation', 'vote', 'objective_completed'),
  player_id UUID REFERENCES users(id),
  target_player_id UUID REFERENCES users(id),
  move_number INT,
  details TEXT,
  created_at TIMESTAMP DEFAULT NOW()
);

-- User ratings
CREATE TABLE user_ratings (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  chess_elo INT DEFAULT 1600,
  deception_rating INT DEFAULT 1600,
  reputation_score INT DEFAULT 50 CHECK (reputation_score >= 0 AND reputation_score <= 100),
  games_played INT DEFAULT 0,
  wins INT DEFAULT 0,
  losses INT DEFAULT 0,
  draws INT DEFAULT 0,
  deceptions_successful INT DEFAULT 0,
  accusations_correct INT DEFAULT 0,
  updated_at TIMESTAMP DEFAULT NOW()
);

-- Create indexes for performance
CREATE INDEX idx_games_status ON games(status);
CREATE INDEX idx_games_white_player ON games(white_player_id);
CREATE INDEX idx_games_black_player ON games(black_player_id);
CREATE INDEX idx_moves_game_id ON moves(game_id);
CREATE INDEX idx_accusations_game_id ON accusations(game_id);
CREATE INDEX idx_objectives_game_id ON game_objectives(game_id);
CREATE INDEX idx_suspicion_game_id ON suspicion_events(game_id);
```

---

## Why ShadowChess is the Strongest Concept

### Viral/Social Potential
- Normal chess: deterministic, logical
- ShadowChess: drama, betrayal, mindgames
- Streamer content gold (accusations, reveals, paranoia)
- "Among Us for chess players" resonates with communities

### Lower Technical Complexity Than ChronoChess
- No complex timeline DAG visualization
- No Stockfish integration needed
- Simpler state management (role-based vs timeline branching)
- Faster MVP iteration

### Higher User Engagement
- Psychology > logic
- Replayability (deception changes each game)
- Social dynamics create community
- Easy to learn, deep to master

### Market Position
- Nobody is building social deception chess
- Massive untapped design space
- Bridges chess + party game audiences
- Potential to launch "Party Chess" genre

---

## Success Metrics

### User Engagement
- [ ] 500+ games completed (MVP launch)
- [ ] Average 3+ accusations per game
- [ ] 40% of replays watched by non-players (social sharing)
- [ ] 50% return player rate within 7 days

### Technical Performance
- [ ] <100ms move latency (P99)
- [ ] <50ms accusation response
- [ ] 1000+ concurrent games supported
- [ ] 99.9% uptime

### Community & Content
- [ ] 50+ streamers featuring game
- [ ] 1M+ YouTube views (first month)
- [ ] Active Discord with 5K+ members
- [ ] Top Reddit posts in r/chess & r/gaming

---

## File Structure (Recommended)

```
ShadowChess/
├── frontend/
│   ├── src/
│   │   ├── components/
│   │   │   ├── Board/
│   │   │   │   ├── ChessBoard.tsx
│   │   │   │   ├── Square.tsx
│   │   │   │   └── PieceIcon.tsx
│   │   │   ├── Accusations/
│   │   │   │   ├── AccusationPanel.tsx
│   │   │   │   └── SuspicionMeter.tsx
│   │   │   ├── Objectives/
│   │   │   │   ├── ObjectiveDisplay.tsx
│   │   │   │   └── DeceptionScore.tsx
│   │   │   ├── Replay/
│   │   │   │   ├── ReplayViewer.tsx
│   │   │   │   └── TimelineViz.tsx
│   │   │   └── Game/
│   │   │       ├── GamePage.tsx
│   │   │       └── GameStatus.tsx
│   │   ├── store/
│   │   │   ├── gameStore.ts
│   │   │   ├── authStore.ts
│   │   │   └── deceptionStore.ts
│   │   ├── utils/
│   │   │   ├── api.ts
│   │   │   └── wsClient.ts
│   │   └── pages/
│   │       ├── LobbyPage.tsx
│   │       ├── GamePage.tsx
│   │       ├── ReplayPage.tsx
│   │       └── LeaderboardPage.tsx
│   └── package.json
│
├── backend/
│   ├── main.go
│   ├── server/
│   │   ├── server.go
│   │   ├── websocket.go
│   │   ├── auth.go
│   │   ├── games.go
│   │   ├── accusations.go
│   │   ├── objectives.go
│   │   └── replay.go
│   ├── models/
│   │   ├── user.go
│   │   ├── game.go
│   │   ├── objective.go
│   │   ├── accusation.go
│   │   └── role.go
│   ├── db/
│   │   ├── db.go
│   │   ├── migrations/
│   │   │   ├── 001_init.sql
│   │   │   ├── 002_objectives.sql
│   │   │   ├── 003_accusations.sql
│   │   │   └── 004_ratings.sql
│   │   ├── queries/
│   │   │   └── queries.go
│   │   └── migrations.go
│   ├── util/
│   │   ├── chess.go
│   │   └── deception.go
│   └── go.mod
│
├── docker-compose.yml
├── .env.example
├── README.md
└── PLAN.md
```

---

## Next Steps

### Immediate (Week 1)
1. [ ] Set up GitHub repo
2. [ ] Initialize frontend & backend projects
3. [ ] Configure PostgreSQL & Redis locally
4. [ ] Design database schema (Phase 1-2)
5. [ ] Create API specifications

### Week 2-3 (Phase 1)
1. [ ] Implement basic authentication
2. [ ] Build WebSocket server
3. [ ] Create game rooms logic
4. [ ] Implement chess.js integration
5. [ ] Build React board component
6. [ ] Deploy to staging

### Week 4-5 (Phase 2-3)
1. [ ] Objective template system
2. [ ] Accusation mechanics
3. [ ] Suspicion UI
4. [ ] Replay viewer (basic)
5. [ ] MVP testing with 20 beta players

---

## Frequently Asked Questions

### Why ShadowChess over traditional chess sites?
Traditional sites optimize for skill-based ranking. ShadowChess adds psychology, social dynamics, and deception. This creates narrative, drama, and community engagement far beyond pure chess rating systems.

### How do you prevent objective collusion?
Server validates all accusations and deception patterns. Impossible accusations (knowing secrets you shouldn't) trigger rate-limiting and detection flags. Post-game analysis identifies suspicious coordination.

### Will experienced chess players dominate?
No. Deception rating is separate from chess ELO. A weak chess player with excellent deception instincts can beat a strong chess player with poor psychology. This balances skill levels.

### What makes this viral?
The "moment of betrayal" when traitor is revealed. Streamers will clip these for social media. The "guess who the traitor is" hook has proven engagement power (Among Us, Secret Hitler, Mafia).

### How do you monetize without P2W?
- Cosmetics (piece skins, board themes)
- Battle pass (seasonal achievements)
- Tournaments (spectator access)
- Premium replays (AI analysis)
- All core gameplay is free forever

---

## Final Thoughts

ShadowChess transforms chess from a game of pure logic into a game of **human psychology**. 

The innovation isn't technical—it's conceptual. You're taking a 1500-year-old game and adding the element it never had: **deception**.

This is why it has exceptional viral potential. Every game generates stories. Every replay is drama. Every accusation is a memorable moment.

**The MVP proves the concept.** Get multiplayer chess + hidden objectives + accusations working, and the game sells itself.

Good luck. This is going to be remarkable. 🎭♟️

