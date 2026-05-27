# ChessWess Development Plan

## Project Vision
ChessWess is a multiverse timeline-based chess game where every move creates history. Players can rewind, branch alternate realities, and win across multiple timelines simultaneously.

**Core Concept**: "What if chess had Git branches?"

---

## Tech Stack Overview

### Frontend
- **React + TypeScript** — Complex UI state management
- **TailwindCSS** — Fast styling
- **React Flow** — Timeline DAG visualization
- **Zustand** — Game state management
- **Framer Motion** — Timeline animation effects

### Backend
- **Go (recommended)**
- **WebSockets** (Socket.IO or native ws)
- **PostgreSQL** — Persistent history graph
- **Redis** — Realtime state & timeline caching

### Chess Engine
- **chess.js** — Legal moves, validation, checkmate
- **Stockfish WASM** — Evaluations & analysis

### Infrastructure
- **Docker** — Containerization
- **Vercel** — Frontend deployment
- **Railway/Fly.io/Render** — Backend deployment

---

## Phase 1: Basic Multiplayer Chess (Weeks 1-3)

### Goal
Establish a solid foundation with standard online chess before introducing timeline mechanics.

### Features
- [ ] User authentication & sessions
- [ ] Game rooms & matchmaking
- [ ] Real chess.js integration
- [ ] Legal move validation
- [ ] Game timers (5min, 10min, unlimited)
- [ ] Basic WebSocket communication
- [ ] Board rendering (React + simple styling)

### Architecture Decisions
- **State Management**: Zustand for game state
- **Database**: PostgreSQL with basic schema
- **Realtime**: WebSocket connection per room
- **Move Validation**: chess.js on both client & server

### Database Schema (Minimal)
```sql
CREATE TABLE users (
  id UUID PRIMARY KEY,
  username VARCHAR UNIQUE,
  created_at TIMESTAMP
);

CREATE TABLE games (
  id UUID PRIMARY KEY,
  white_player_id UUID,
  black_player_id UUID,
  status ENUM('pending', 'active', 'completed'),
  created_at TIMESTAMP
);
```

### Deliverables
- [ ] Playable online chess game
- [ ] 2-player real-time sync
- [ ] Move history (linear list)
- [ ] Basic UI mockups implemented

### Estimated Effort
- **Backend**: 8-10 hours (rooms, validation, WebSocket)
- **Frontend**: 10-12 hours (board, moves, timers)
- **Database**: 2-3 hours

### Success Criteria
- Two players can play a full game
- Moves sync instantly
- No race conditions
- Checkmate/stalemate detected correctly

---

## Phase 2: Immutable Move History (Weeks 4-6)

### Goal
Build the foundation for timelines by storing every move as an immutable node in a graph structure.

### Features
- [ ] Game state as DAG (Directed Acyclic Graph)
- [ ] Immutable GameNode structure
- [ ] Move tree visualization (simple)
- [ ] Replay any point in history
- [ ] Branch detection UI

### Key Data Structure
```typescript
type GameNode = {
  id: string;                    // Unique node ID
  parentId: string | null;       // Linear history parent
  childrenIds: string[];         // Branches from this node
  boardState: Board;             // FEN representation
  move: Move;                    // Chess move (e.g., e2e4)
  timelineId: string;           // Which timeline owns this
  turnNumber: number;           // Turn within timeline
  createdBy: Player;            // Who made the move
  metadata: {
    check: boolean;
    checkmate: boolean;
    evaluation: number;         // Stockfish score
  };
  createdAt: Date;
};
```

### Database Schema Expansion
```sql
CREATE TABLE game_nodes (
  id UUID PRIMARY KEY,
  game_id UUID REFERENCES games(id),
  timeline_id UUID,
  parent_node_id UUID REFERENCES game_nodes(id),
  board_state TEXT (FEN),        -- Fenstring
  move_san VARCHAR,              -- Move notation (e2e4)
  turn_number INT,
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMP
);

CREATE TABLE node_children (
  parent_id UUID REFERENCES game_nodes(id),
  child_id UUID REFERENCES game_nodes(id),
  PRIMARY KEY (parent_id, child_id)
);
```

### Deliverables
- [ ] Every move stored as immutable node
- [ ] Move tree reconstruction working
- [ ] Replay system (click any node, board updates)
- [ ] FEN serialization for all nodes

### Estimated Effort
- **Backend**: 6-8 hours (node storage, tree queries)
- **Frontend**: 4-6 hours (replay UI, node inspection)
- **Database Migration**: 3-4 hours

### Success Criteria
- Can replay any move sequence
- Graph structure is queryable
- Zero data loss on rewind
- Performance acceptable for 100+ nodes

---

## Phase 3: Timeline Branching (Weeks 7-9) ⭐ MVP Magic

### Goal
Introduce the core innovation: rewind and create alternate realities.

### Features
- [ ] **Rewind Move** — Go back X turns, create branch
- [ ] **Timeline Creation** — Each rewind creates new timeline
- [ ] **Timeline DAG Visualization** — React Flow timeline graph
- [ ] **Branch Inspection** — Click any branch, see board state
- [ ] **Active Timeline Switching** — Select which timeline is "active"

### Rewind Mechanics
```
Timeline A: Move 1 → Move 2 → Move 3 → Move 4 → Move 5
                                    ↓
                             [REWIND TO MOVE 3]
                                    ↓
Timeline B: Move 1 → Move 2 → Move 3 → Move 3B → Move 4B
```

### UI Components Needed
- **Timeline Graph** (React Flow)
  - Nodes represent board states
  - Edges represent moves
  - Color-coded by status (winning/losing/neutral)
- **Board State Inspector**
  - Select any node, see current board
- **Timeline Heatmap**
  - Highlight strong vs weak branches
- **Branch Control Panel**
  - Rewind button + turn selector
  - Timeline switcher

### Database Updates
```sql
CREATE TABLE timelines (
  id UUID PRIMARY KEY,
  game_id UUID REFERENCES games(id),
  root_node_id UUID REFERENCES game_nodes(id),
  created_at TIMESTAMP
);

-- Link nodes to timelines
ALTER TABLE game_nodes ADD timeline_id UUID REFERENCES timelines(id);
```

### Deliverables
- [ ] Rewind creates new timeline
- [ ] Timeline graph renders correctly
- [ ] Both players see same branching
- [ ] Board stays consistent across switches

### Estimated Effort
- **Backend**: 8-10 hours (timeline logic, branching queries)
- **Frontend**: 12-15 hours (React Flow setup, animations)
- **Visualization**: 6-8 hours (layout algorithms, heatmap)

### Success Criteria
- Players can rewind and branch
- Up to 50 timelines render smoothly
- No timeline data corruption
- Winning conditions evaluate per-timeline

---

## Phase 4: Timeline Navigation & Controls (Weeks 10-11)

### Goal
Make switching between timelines fluid and intuitive.

### Features
- [ ] **Jump Between Timelines** — Instant switch, continuous gameplay
- [ ] **Timeline Labels** — Name branches (e.g., "Sacrificial Queen", "Defensive Hold")
- [ ] **Breadcrumb Navigation** — Show active path from root
- [ ] **Divergence Highlights** — Visualize where timelines split
- [ ] **Performance Optimization** — Lazy load large graphs

### UI Enhancements
- Minimap of timeline graph
- Zoom/pan controls
- Hotkeys for timeline jumping
- Timeline sidebar showing stats (material count, evaluation)

### Deliverables
- [ ] Seamless branch switching
- [ ] Timeline labels persisted
- [ ] Graph renders efficiently (1000+ nodes)
- [ ] UX polished for competitive play

### Estimated Effort
- **Backend**: 4-5 hours (optimization queries)
- **Frontend**: 6-8 hours (UX polish, hotkeys, minimap)

### Success Criteria
- Can switch timelines in <100ms
- No lag with complex graphs
- Intuitive navigation for new players

---

## Phase 5: Time Mechanics & Energy System (Weeks 12-14)

### Goal
Introduce strategic depth through resource management.

### Features
- [ ] **Time Energy Pool** — Players get limited rewinds per game
- [ ] **Energy Costs**
  - Rewind to N turns ago = costs N energy
  - Jump timeline = costs 1 energy
- [ ] **Branch Locking** — Spend energy to freeze a timeline (opponent can't rewind into it)
- [ ] **Time Collapse** — If 30+ timelines, weakest collapses (auto-deletion)
- [ ] **Paradox Penalties** — Contradictions weaken timelines gradually

### Game Rules
```
Time Energy Pool: 15 (per player, per game)

Actions:
- Rewind 1 turn: 1 energy
- Rewind 5 turns: 5 energy
- Jump timeline: 1 energy
- Lock timeline: 3 energy
- Paradox created: -2 energy (penalty)

Win Condition Options (configurable):
1. Checkmate in ANY timeline
2. Control majority timelines (>50%)
3. Score-based: Checkmate + Timeline control = points
```

### Database Updates
```sql
CREATE TABLE timeline_metadata (
  timeline_id UUID PRIMARY KEY REFERENCES timelines(id),
  locked_by_player_id UUID REFERENCES users(id),
  is_locked BOOLEAN DEFAULT false,
  energy_cost_to_create INT,
  stability_score INT (0-100)
);

CREATE TABLE player_energy (
  game_id UUID,
  player_id UUID,
  energy_remaining INT,
  PRIMARY KEY (game_id, player_id)
);
```

### Deliverables
- [ ] Energy system fully functional
- [ ] Timeline locking prevents edits
- [ ] Collapse system removes old timelines
- [ ] UI shows energy & costs clearly

### Estimated Effort
- **Backend**: 6-7 hours (energy logic, collapse system)
- **Frontend**: 4-5 hours (energy UI, cost indicators)

### Success Criteria
- Energy prevents infinite branching
- Gameplay becomes strategic
- Locking mechanic creates tension
- Collapse prevents performance degradation

---

## Phase 6: Spectator & Replay System (Weeks 15-17)

### Goal
Create replay cinematic experience for sharing & learning.

### Features
- [ ] **Spectator Mode** — Watch live multiverse battles
- [ ] **Replay Engine** — Rewatch any stored game
- [ ] **Branching Playback** — Animate moves along timeline graph
- [ ] **Divergence Highlights** — Show "what-if" moments
- [ ] **AI Commentary** (placeholder for Phase 7)
- [ ] **Export Replay** — Share as GIF or link

### Replay UI
- Timeline graph with animated piece movement
- Show board state at each node
- Highlight critical divergence moments
- Speed controls (1x, 2x, 4x)
- Jump to interesting moments

### Deliverables
- [ ] Can replay any completed game
- [ ] Smooth animations across timelines
- [ ] Export to shareable link
- [ ] Spectator mode functional

### Estimated Effort
- **Backend**: 4-5 hours (replay API, export)
- **Frontend**: 8-10 hours (animations, playback UI)

### Success Criteria
- Replays are smooth & engaging
- Shareable links work correctly
- Competitive scene can broadcast

---

## Phase 7: AI Features & Stockfish Integration (Weeks 18-20)

### Goal
Add intelligent analysis and guidance.

### Features
- [ ] **Stockfish WASM Integration** — Evaluations per timeline
- [ ] **Best Move Suggestions** — Per timeline, per turn
- [ ] **Branch Scoring** — Which timelines are winning?
- [ ] **Evaluation Graph** — Show material advantage over time
- [ ] **AI Narrator** (optional)
  - Example: *"White sacrifices reality to save the queen."*

### Implementation
```typescript
// Evaluate all timelines
async function evaluateTimelineGraph(gameId: string) {
  const nodes = await getGameNodes(gameId);
  const evaluations = {};
  
  for (const node of nodes) {
    const score = await stockfish.evaluate(node.boardState);
    evaluations[node.id] = score;
  }
  
  return evaluations;
}
```

### Deliverables
- [ ] Stockfish evaluations cached in Redis
- [ ] Per-timeline strength displayed
- [ ] Suggestion engine working
- [ ] No lag in UI from analysis

### Estimated Effort
- **Backend**: 6-8 hours (Stockfish integration, caching)
- **Frontend**: 4-5 hours (evaluation UI)

### Success Criteria
- Evaluations <500ms per timeline
- Suggestions contextually relevant
- Cache prevents redundant computation

---

## Phase 8: Ranked Competitive & Polish (Weeks 21-24)

### Goal
Production-ready competitive system.

### Features
- [ ] **ELO Rating System** — Traditional chess rating
- [ ] **Ladder & Leaderboards** — Player rankings
- [ ] **Tournaments** — Seasonal competitive events
- [ ] **Replay Archive** — Searchable game database
- [ ] **Performance Monitoring** — APM metrics
- [ ] **Moderation Tools** — Report/ban system

### Ranked Rules
- Time controls: Bullet (1+0), Blitz (3+0), Rapid (10+0)
- ELO adjustments based on opponent rating
- K-factor: 32 for most players, 8 for 2000+ rated
- Anti-cheat: Detected engine use → ban

### Database
```sql
CREATE TABLE user_ratings (
  user_id UUID PRIMARY KEY,
  elo INT DEFAULT 1600,
  games_played INT,
  wins INT,
  losses INT,
  updated_at TIMESTAMP
);

CREATE TABLE tournaments (
  id UUID PRIMARY KEY,
  name VARCHAR,
  status ENUM('signup', 'active', 'completed'),
  created_at TIMESTAMP
);
```

### Deliverables
- [ ] ELO calculation working
- [ ] Leaderboard updated in real-time
- [ ] Tournament bracket system
- [ ] Player profiles complete

### Estimated Effort
- **Backend**: 8-10 hours (ELO, tournaments, moderation)
- **Frontend**: 6-8 hours (profile pages, leaderboards)
- **DevOps**: 4-6 hours (monitoring, scaling)

### Success Criteria
- 1000+ concurrent players
- <50ms latency
- Fair ELO distribution
- Community engagement

---

## Development Timeline Summary

| Phase | Duration | Focus | Key Deliverable |
|-------|----------|-------|-----------------|
| 1 | 3 weeks | Foundation | Playable chess |
| 2 | 3 weeks | Architecture | Immutable history |
| 3 | 3 weeks | **Core Magic** | Timeline branching ⭐ |
| 4 | 2 weeks | UX Polish | Smooth navigation |
| 5 | 3 weeks | Gameplay Depth | Energy system |
| 6 | 3 weeks | Virality | Replay system |
| 7 | 3 weeks | Intelligence | AI analysis |
| 8 | 4 weeks | Competition | Ranked ladder |
| **Total** | **24 weeks** | **~6 months** | **Full product** |

---

## Critical Dependencies

### Phase Ordering (CANNOT Skip)
```
Phase 1 (Chess) 
  ↓
Phase 2 (History Graph)
  ↓
Phase 3 (Branching) ← MVP LAUNCHES HERE
  ↓
Phase 4 (Navigation)
  ↓
Phase 5+ (Feature layers)
```

**Milestone**: Phase 3 completion = MVP ready for early users.

---

## Current Optimization Challenges

These are challenges visible in the current implementation that should be optimized alongside the roadmap. They are not all new features; many are reliability, scalability, and production-readiness improvements that will make the timeline mechanics safer to build on.

### Challenge 1: Plan Drift vs Implementation Drift
**Problem**: The current codebase already includes pieces from later phases, such as bot games, timeline metadata, energy tables, snapshots, and branching logic, while the plan still lists some of these as future work.

**Optimization**:
- Track each feature as `not started`, `partial`, `implemented`, or `needs optimization`.
- Add a short status note per phase before starting new work.
- Keep the MVP scope focused on the features that are actually playable end-to-end.

### Challenge 2: Timeline Branching Consistency
**Problem**: Rewind and branching exist, but the plan needs stricter gameplay rules for when and how branches can be created.

**Optimization**:
- Define who can rewind and when.
- Decide whether branching should automatically switch the active timeline.
- Block branch creation from locked, collapsed, completed, or invalid timelines.
- Specify behavior after checkmate, stalemate, resignation, and timeout.

**MVP Rule Set**:
- Only a seated player can rewind, and only when it is their turn in the target position.
- Rewind targets must be on the active timeline and within the same game.
- Rewinding always creates a new timeline branch; no edits to existing nodes.
- Branch creation auto-switches the active timeline for both players.
- Branching is blocked from locked or collapsed timelines.
- After checkmate, stalemate, resignation, or timeout: no new branches (replay-only).

### Challenge 3: Atomic Move Handling
**Problem**: A move can involve validation, node creation, energy changes, active timeline updates, and game-over checks. If these are not handled atomically, partial state can leak into the game.

**Optimization**:
- Wrap move creation, energy spending, timeline updates, and game completion in database transactions.
- Make repeated move submissions idempotent where possible.
- Return consistent errors when a move loses a race against another client.

### Challenge 4: WebSocket Reliability
**Problem**: Realtime sync currently depends on broadcast messages. If a client disconnects or misses a message, its local timeline graph can drift from the server.

**Optimization**:
- Add reconnect recovery and state resync.
- Add server sequence numbers to game events.
- Make move, rewind, and timeline-switch messages idempotent.
- Define a slow-client strategy: drop, resync, or disconnect with a recoverable error.

### Challenge 5: Security and Access Control
**Problem**: A valid token plus a game ID should not automatically allow a user to send gameplay events for that game.

**Optimization**:
- Verify that the user is a player, bot participant owner, or authorized spectator before joining a WebSocket room.
- Validate permissions per event type.
- Prevent non-players from moving, rewinding, locking timelines, or switching active competitive state.

### Challenge 6: CORS and Origin Policy
**Problem**: WebSocket origin checks are currently permissive, which is useful for local development but unsafe for production.

**Optimization**:
- Configure allowed origins by environment.
- Keep local development permissive only when explicitly enabled.
- Add deployment checks so production cannot start with unsafe origin settings.

### Challenge 7: Server-Authoritative Timers
**Problem**: Frontend clocks are useful for display, but competitive timers must be controlled by the server.

**Optimization**:
- Store clock state on the server.
- Update time during accepted moves, resignations, disconnects, and timeout checks.
- Broadcast authoritative clock snapshots.
- Add timeout handling as a first-class game result.

### Challenge 8: Bot Lifecycle Management
**Problem**: Bot games start background engine work, but the plan should cover how that work stops, retries, and avoids duplicate runners.

**Optimization**:
- Cancel bot workers when games end or are abandoned.
- Prevent multiple bot engines from controlling the same bot in one game.
- Add bot move delay, rate limits, and error recovery.
- Log bot failures with enough context to debug invalid positions or missed turns.

### Challenge 9: Database Migration Maturity
**Problem**: Embedded `CREATE IF NOT EXISTS` migrations are convenient early, but production needs versioned, repeatable schema changes.

**Optimization**:
- Move to versioned migrations before beta.
- Add migration rollback or forward-fix guidance.
- Test migrations against a copy of realistic data.
- Document required extensions such as `gen_random_uuid()`.

### Challenge 10: Board-State Snapshot Strategy
**Problem**: Snapshotting is already present, but the rules for when to store full board state versus reconstruct from history should be explicit.

**Optimization**:
- Define snapshot frequency based on turn count, timeline size, and replay cost.
- Cap maximum reconstruction depth.
- Benchmark path reconstruction for large games.
- Compress or archive completed games without losing deterministic replay.

### Challenge 11: Graph Query Scalability
**Problem**: Timeline APIs can become expensive as the number of timelines and nodes grows.

**Optimization**:
- Batch node counts and node-window queries.
- Avoid per-timeline N+1 query patterns.
- Add pagination or viewport-based graph loading.
- Cache graph summaries for active games.

### Challenge 12: Frontend Memory Growth
**Problem**: The frontend stores loaded timelines and nodes in memory. Large multiverse games can make the client slow even if the server is fast.

**Optimization**:
- Load only visible or recently used graph regions.
- Evict cold timelines from client memory.
- Keep selected node, active path, and graph summary separate from full node history.
- Add render budgets for large React Flow graphs.

### Challenge 13: Linear Moves vs Timeline Nodes
**Problem**: The app has both `game_moves` and `game_nodes`. The plan should identify which source is canonical as timeline chess becomes the primary mode.

**Optimization**:
- Decide whether `game_nodes` becomes the source of truth for all moves.
- Keep `game_moves` only for compatibility, summaries, or standard chess mode if needed.
- Avoid writing divergent move histories.
- Document migration strategy for existing games.

### Challenge 14: Game Rule Clarity
**Problem**: The final win condition affects database design, UI, energy costs, matchmaking, and competitive integrity.

**Optimization**:
- Choose an MVP win condition early.
- Compare options: checkmate in any timeline, active timeline only, majority timeline control, or score-based result.
- Document draw, resignation, timeout, and collapsed-timeline outcomes.
- Ensure the UI always explains the current win state clearly.

### Challenge 15: Observability
**Problem**: Timeline bugs are difficult to debug without structured metrics and event traces.

**Optimization**:
- Track move latency, rewind latency, DB query time, WebSocket disconnects, and failed move validations.
- Log game ID, timeline ID, node ID, and event sequence for important operations.
- Add lightweight health checks for database, Redis, and WebSocket readiness.
- Create dashboards before closed beta.

### Challenge 16: Testing Gap
**Problem**: Timeline mechanics create many edge cases that manual testing will miss.

**Optimization**:
- Add tests for legal move validation, branching from historical nodes, and concurrent move submissions.
- Test energy spending, refunds, locked timelines, and collapsed timelines.
- Add replay determinism tests.
- Add frontend tests for timeline switching, selected node replay, and reconnect resync.

### Challenge 17: Production Readiness
**Problem**: The app needs hardening before public or competitive play.

**Optimization**:
- Validate required environment variables at startup.
- Enforce secure JWT secret handling.
- Add request size limits and rate limiting.
- Tune database connection pools.
- Add graceful shutdown for HTTP, WebSocket, and bot workers.

---

## MVP Scope (Minimum Viable Product)

### What to Ship
- ✅ Multiplayer chess (Phase 1)
- ✅ Timeline branching (Phase 3)
- ✅ Basic graph visualization (Phase 3)
- ✅ Replay system (Phase 6, simplified)

### What to SKIP
- ❌ AI features
- ❌ Quantum mechanics (Phase 2+ feature)
- ❌ Ranked ladder (Phase 8)
- ❌ Cosmetics & skins
- ❌ Tournament system

**MVP Pitch**: "Chess with branching timelines. Rewind your blunders. Win across multiple realities."

---

## Architecture Principles

### 1. Immutability
Every game node is **frozen forever**. Never mutate old board states.

### 2. Isolation
Each timeline's move validation is **independent**. One timeline's moves don't affect others.

### 3. Caching
- Timeline state cached in Redis
- Evaluation scores cached per node
- Graph structure cached (refreshed on new branch)

### 4. Incremental Rendering
- Lazy load timeline graph (don't render 1000 nodes upfront)
- Virtual scrolling for node lists
- Minimap for quick navigation

### 5. Deterministic
Replay any timeline sequence and get **identical** board state.

---

## Key Technical Challenges & Solutions

### Challenge 1: Timeline Synchronization
**Problem**: Both players must see identical graphs, no race conditions.

**Solution**:
- WebSocket emits events in order
- Server processes moves atomically
- Redis maintains canonical timeline state
- Client replicates server state

### Challenge 2: Efficient Storage
**Problem**: Storing full board per node = massive database.

**Solution**:
- Store only **FEN strings** (compact)
- Store only the **move** (not full board diff)
- Derive board state from path: Root → Node
- Compress old timelines after completion

### Challenge 3: Move Validation Across Timelines
**Problem**: Same position reached via different paths = valid in both?

**Solution**:
- Validate moves in **current timeline context only**
- Each timeline has independent move history
- Piece positions derived from move sequence, not global state

### Challenge 4: Performance at Scale
**Problem**: 10,000 node games become slow.

**Solution**:
- Implement **timeline pruning** (collapse unused branches)
- Use **B-tree index** on (game_id, timeline_id, turn_number)
- Cache heatmap calculations in Redis
- Lazy-load graph UI (viewport-based rendering)

### Challenge 5: Timeline Rendering Complexity
**Problem**: Large DAGs become spaghetti.

**Solution**:
- Use **Sugiyama layout algorithm** (hierarchical)
- Implement **minimap** for quick orientation
- Use **zoom/pan** with performance budget
- Color-code nodes by strength (green = winning, red = losing)

---

## Tech Decisions & Rationale

### Why React Flow for DAG?
- Excellent out-of-box for graph visualization
- Handles 1000+ nodes efficiently
- Built-in zoom/pan/select
- Active community

### Why Zustand over Redux?
- Redux has too much boilerplate for game state
- Zustand: simple, fast, perfect for multiplayer
- Easy to sync with WebSocket updates

### Why Go over Node.js?
- Better concurrency (goroutines)
- Faster WebSocket handling
- Lower memory footprint
- Perfect for state-heavy apps

### Why PostgreSQL over MongoDB?
- Relational graph queries are complex in MongoDB
- Need ACID for move integrity
- FEN strings are simple text (no NoSQL advantage)

---

## Resource Allocation

### Team Composition (Recommended)
- **1 Backend Lead** (Go/WebSocket/DB architecture)
- **1 Frontend Lead** (React/visualization/UX)
- **1 DevOps/Infrastructure** (Docker, deployment)

**Solo Development**: Expect 12-18 months instead of 6. Prioritize MVP first.

---

## Success Metrics

### User Engagement
- [ ] 100+ games completed (Phase 3 launch)
- [ ] Average 5 rewinds per game (branching adoption)
- [ ] 50% of replays viewed by non-players (virality)

### Technical Performance
- [ ] <100ms move latency (P99)
- [ ] <500ms timeline graph render
- [ ] <50ms rewind operation
- [ ] 1000+ concurrent users (EC2 scale test)

### Business Metrics
- [ ] 10,000 MAU by end of Year 1
- [ ] 95% positive reviews/ratings
- [ ] Successful tournament with 100+ players

---

## Launch Checklist

### Before MVP Release
- [ ] All Phase 1-3 features working
- [ ] Database migrations tested in production
- [ ] Load testing (simulate 100 concurrent games)
- [ ] Security audit (no SQL injection, CORS correct)
- [ ] UX testing with 10+ beta players
- [ ] Mobile responsive design (basic)

### Beta Launch
- [ ] Closed beta with 100 players
- [ ] Collect feedback
- [ ] Fix critical bugs
- [ ] Optimize performance

### Public Launch
- [ ] Marketing campaign
- [ ] Press release
- [ ] Community Discord/Reddit
- [ ] Content creator outreach

---

## File Structure (Recommended)

```
ChessWess/
├── frontend/
│   ├── src/
│   │   ├── components/
│   │   │   ├── Board/
│   │   │   ├── Timeline/
│   │   │   ├── ReplayPlayer/
│   │   ├── store/
│   │   │   └── gameStore.ts (Zustand)
│   │   ├── utils/
│   │   │   └── chess.ts
│   ├── package.json
│   └── README.md
├── backend/
│   ├── main.go
│   ├── server/
│   ├── models/
│   │   ├── game.go
│   │   ├── timeline.go
│   │   └── node.go
│   ├── db/
│   │   └── migrations/
│   └── go.mod
├── docker-compose.yml
├── .env.example
└── PLAN.md (this file)
```

---

## Next Steps

### Immediate (Week 1)
1. [ ] Set up project repo (GitHub)
2. [ ] Create frontend & backend project structure
3. [ ] Set up PostgreSQL locally
4. [ ] Initialize Redis
5. [ ] Create initial database schema

### Week 2-3 (Phase 1)
1. [ ] Implement chess.js integration
2. [ ] Build WebSocket server
3. [ ] Create game rooms logic
4. [ ] Build basic React board component
5. [ ] Deploy to staging

### Week 4+ (Phase 2)
Begin immutable graph implementation...

---

## Questions to Answer Before Starting

1. **Team**: Solo or team? (Affects timeline: 6 months vs 12-18)
2. **Infrastructure**: Self-hosted or cloud? (AWS, Vercel, Railway?)
3. **Monetization**: Free forever or premium? (Affects Phase 8 design)
4. **Scope**: Full Phase 8 or stop at MVP (Phase 3)?
5. **Time Commitment**: Full-time or side project? (Affects feasibility)

---

## Resources

### Learning
- React Flow docs: https://reactflow.dev/
- Zustand docs: https://github.com/pmndrs/zustand
- chess.js docs: https://github.com/jhlywa/chess.js
- PostgreSQL graph queries: Window functions, CTEs

### Tools
- **Local**: Docker Compose (Postgres + Redis)
- **Staging**: Vercel (frontend) + Railway (backend)
- **Monitoring**: LogRocket (frontend), DataDog (backend)

---

## Final Notes

**This is not a side project.** ChessWess is a 6-month full-time commitment or 12-18 months part-time.

The magic moment happens in **Phase 3** when branching works. That's when people will understand the vision.

Every other phase builds on that foundation.

Start small, launch MVP, iterate with users.

**The innovation is the timeline graph. Everything else is execution.**

Good luck. This is going to be incredible. 🚀
