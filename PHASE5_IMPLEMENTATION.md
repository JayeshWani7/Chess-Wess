# Phase 5: Time Mechanics & Energy System - Implementation Guide

## Overview
This document explains the complete Phase 5 implementation for the Chess Wess application, introducing strategic depth through resource management via time energy mechanics.

---

## ✅ What Has Been Implemented

### 1. **Database Schema (Backend)**
- **migrations.go**: Added 3 new migrations
  - `createPlayerEnergyTable`: Tracks energy pools per player per game
  - `createTimelineMetadataTable`: Stores lock status, stability scores, paradoxes
  - `alterTimelinesAddLocking`: Idempotent migrations for safe deployment

**Tables Created:**
- `player_energy` - Energy tracking per player/game
- `timeline_metadata` - Timeline health, locks, paradoxes
- `energy_transactions` - Audit trail of all energy spending

### 2. **Backend Models (Go)**
**File**: `backend/models/energy.go`
- `PlayerEnergy` - Player's energy pool state
- `TimelineMetadata` - Timeline health metrics
- `EnergyCostConfig` - Game rule configuration
- `EnergyTransaction` - Audit logging

### 3. **Backend Database Functions (Go)**
**File**: `backend/db/energy.go`

**Player Energy Functions:**
- `InitPlayerEnergy()` - Initialize 15 energy for each player
- `GetPlayerEnergy()` - Fetch current energy
- `SpendEnergy()` - Deduct energy with logging
- `RefundEnergy()` - Refund energy (on errors)

**Timeline Metadata Functions:**
- `InitTimelineMetadata()` - Create metadata for new timeline
- `GetTimelineMetadata()` - Retrieve timeline health
- `LockTimeline()` - Freeze a timeline
- `UnlockTimeline()` - Remove lock
- `IsTimelineLocked()` - Check lock status
- `ApplyParadoxPenalty()` - Reduce stability on contradictions

**Time Collapse Functions:**
- `GetGameTimelines()` - Fetch all timelines sorted by strength
- `CheckTimelineCollapse()` - Auto-collapse weak timelines when 30+ exist
- `MarkTimelineForCollapse()` - Mark timeline for deletion
- `GetCollapsedTimelines()` - Fetch marked timelines
- `DeleteCollapsedTimeline()` - Hard delete timeline & nodes

### 4. **Backend API Endpoints (Go)**
**File**: `backend/server/energy.go`

**Endpoints:**
- `GET /api/games/{gameID}/energy` - Get player's current energy
- `POST /api/games/{gameID}/energy/spend` - Spend energy (rewind, jump, lock)
- `POST /api/games/{gameID}/energy/refund` - Refund energy
- `POST /api/games/{gameID}/energy/lock-timeline` - Lock a timeline (3 energy)
- `GET /api/games/{gameID}/energy/timeline-status?timeline_id={id}` - Get timeline metadata

### 5. **Frontend Store (TypeScript)**
**File**: `frontend/src/store/gameStore.ts`

**New Types:**
- `PlayerEnergy` - Energy state interface
- `TimelineMetadata` - Timeline health interface

**New Store Fields:**
- `playerEnergy: PlayerEnergy | null` - Current player energy
- `timelineMetadata: Record<string, TimelineMetadata>` - All timeline health data

**New Store Actions:**
- `setPlayerEnergy(energy)` - Update energy display
- `setTimelineMetadata(metadata[])` - Load all timeline data
- `updateTimelineMetadata(timelineId, partial)` - Update single timeline
- `consumeEnergy(amount)` - Deduct energy locally
- `refundEnergy(amount)` - Add energy back locally

### 6. **Frontend UI Components (React/TypeScript)**
**File**: `frontend/src/components/Energy/EnergyPanel.tsx`

**Components:**
- `EnergyPanel` - Main energy display with bar chart and costs
  - Shows energy remaining vs max (15)
  - Color-coded: Green (50%+), Yellow (25-50%), Red (<25%)
  - Displays action costs
  
- `TimelineStatusCard` - Individual timeline health card
  - Lock status badge
  - Stability score with progress bar
  - Paradox counter
  - Collapse status
  
- `TimelineControlPanel` - Timeline management UI
  - List of all timelines
  - Lock buttons
  - Collapse warning when 30+ timelines
  
- `EnergyNotification` - Alert messages
  - Warning, error, info types
  - Dismissible

### 7. **Frontend Utilities (TypeScript)**
**File**: `frontend/src/utils/energy.ts`

**Constants:**
- Energy costs for all actions
- Collapse threshold (30 timelines)

**Functions:**
- `calculateRewindCost(turnsBack)` - N turns = N energy
- `fetchPlayerEnergy(gameId)` - Get from server
- `spendEnergy(gameId, amount, action, details)` - Spend energy
- `lockTimeline(gameId, timelineId)` - Lock API call
- `getTimelineStatus(gameId, timelineId)` - Get timeline health
- `hasEnoughEnergy(current, needed)` - Check available
- `shouldCheckCollapse(timelineCount)` - Check if collapse needed
- `isTimelineWeak(stability, paradoxes)` - Weak timeline check

### 8. **Frontend Custom Hook (React)**
**File**: `frontend/src/hooks/useEnergy.ts`

**Hook: `useEnergy()`**
Returns:
- `playerEnergy` - Current energy state
- `timelineMetadata` - All timeline metadata
- `loading` - Async operation state
- `error` - Error message

**Methods:**
- `refreshEnergy()` - Fetch from server
- `rewindTimeline(turnsBack, timelineId)` - Spend energy to rewind
- `jumpTimeline(targetTimelineId)` - Spend energy to jump
- `lockTimeline(timelineId)` - Spend energy to lock
- `getTimelineStrength(timelineId)` - Calculate timeline score

---

## 🔧 Integration Steps

### Step 1: Database Migration
```bash
# The migrations are already defined. Run this when deploying:
cd backend
go run main.go
# Migrations auto-execute on startup (see RunMigrations in db/migrations.go)
```

### Step 2: Initialize Energy on Game Start
In your game creation handler (after game is created), call:
```go
db.InitPlayerEnergy(ctx, pool, gameID, whitePlayerID, blackPlayerID, 15)
```

### Step 3: Create Timeline Metadata on Branch
When a player rewinds and creates a new timeline, call:
```go
db.InitTimelineMetadata(ctx, pool, newTimelineID, gameID, energyCostUsed)
```

### Step 4: Integrate Energy Spending
In your move validation/rewind handler:
```go
// Check energy before allowing rewind
cost := turnsBack * 1  // 1 energy per turn
err := db.SpendEnergy(ctx, pool, gameID, playerID, cost, "rewind", details)
if err != nil {
  // Not enough energy - reject rewind
  return fmt.Errorf("not enough energy")
}
```

### Step 5: Use Frontend Hook
In your Game component:
```typescript
import { useEnergy } from "@/hooks/useEnergy";

export function GamePage() {
  const { playerEnergy, timelineMetadata, lockTimeline, rewindTimeline } = useEnergy();

  const handleLockTimeline = async (timelineId: string) => {
    const success = await lockTimeline(timelineId);
    if (success) {
      // UI will update automatically via Zustand
    }
  };

  return (
    <div>
      <EnergyPanel />
      <TimelineControlPanel onLockTimeline={handleLockTimeline} />
    </div>
  );
}
```

---

## 📊 Energy System Rules

### Energy Costs
| Action | Cost | Effect |
|--------|------|--------|
| Rewind 1 turn | 1 | Create branch, replay to earlier state |
| Rewind 5 turns | 5 | Create branch, replay 5 moves back |
| Jump timeline | 1 | Switch to different branch (active timeline) |
| Lock timeline | 3 | Freeze timeline - opponent can't rewind into it |
| Paradox created | -2 | Penalty: lost 2 energy automatically |

### Energy Pool
- **Starting**: 15 energy per player per game
- **Max**: 15 (can't gain beyond this)
- **Minimum**: 0 (can't spend if 0)

### Timeline Collapse
- **Trigger**: When 30+ timelines exist in a game
- **Action**: Automatically marks weakest timelines for deletion
- **Protection**: Locked timelines never collapse
- **Strength Calculation**: `stability_score - (paradox_count × 10)`

### Stability & Paradoxes
- **Starting Stability**: 100%
- **Paradox**: Contradiction detected (e.g., move creates invalid board state)
- **Penalty**: Each paradox reduces stability by 10 points
- **Min Stability**: 0% (timeline weakened, will collapse first)

---

## 🚀 Testing Checklist

### Backend Tests
- [ ] Player energy initialized to 15 when game starts
- [ ] Energy deducts correctly on spend operations
- [ ] Energy refund works
- [ ] Timeline lock prevents opponent rewinding
- [ ] Time collapse removes weakest timelines when 30+ exist
- [ ] Paradox penalties reduce stability

### Frontend Tests
- [ ] EnergyPanel displays current energy
- [ ] Energy bar color changes (green → yellow → red)
- [ ] TimelineControlPanel shows all timelines
- [ ] Lock button disabled when insufficient energy
- [ ] Lock button works and updates timeline status
- [ ] Collapse warning appears when approaching limit
- [ ] useEnergy hook provides correct energy state
- [ ] Energy updates in real-time after actions

### Integration Tests
- [ ] Game starts with both players at 15 energy
- [ ] Rewind costs correct amount
- [ ] Jump costs 1 energy
- [ ] Lock costs 3 energy and prevents opponent moves
- [ ] Paradox applies -2 penalty
- [ ] 31st timeline triggers collapse check

---

## 🔗 WebSocket Integration (Optional)

For real-time energy updates in multiplayer:

```go
// In your WebSocket message handler:
case "timeline_locked":
  // Notify opponent timeline was locked
  // Broadcast: {type: "timeline_locked", timeline_id: "...", locked_by: "..."}

case "energy_spent":
  // Notify opponent player spent energy
  // Broadcast: {type: "energy_spent", player_id: "...", amount: 5, action: "rewind"}

case "timeline_collapsed":
  // Notify both players timeline was auto-deleted
  // Broadcast: {type: "timeline_collapsed", timeline_id: "...", reason: "weak"}
```

---

## 📁 File Summary

### Backend
```
backend/
├── db/
│   ├── migrations.go (UPDATED - added Phase 5 migrations)
│   └── energy.go (NEW - all energy functions)
├── models/
│   └── energy.go (NEW - energy/timeline types)
└── server/
    ├── energy.go (NEW - API endpoints)
    └── routes.go (UPDATED - added energy routes)
```

### Frontend
```
frontend/src/
├── components/
│   └── Energy/
│       └── EnergyPanel.tsx (NEW - UI components)
├── store/
│   └── gameStore.ts (UPDATED - energy fields & actions)
├── hooks/
│   └── useEnergy.ts (NEW - energy logic hook)
└── utils/
    └── energy.ts (NEW - energy utilities & API calls)
```

---

## 🎮 Game Loop Integration Example

```typescript
// In your GamePage component

export function GamePage() {
  const gameId = useGameStore((state) => state.activeGameId);
  const { playerEnergy, lockTimeline, rewindTimeline } = useEnergy();

  const handleRewind = async (turnsBack: number) => {
    const success = await rewindTimeline(turnsBack, activeTimelineId);
    if (success) {
      // Rewind successful, UI updated via store
      await fetchGameState();
    } else {
      // Show error: not enough energy
      showNotification("Not enough energy!", "error");
    }
  };

  const handleLockTimeline = async (timelineId: string) => {
    const success = await lockTimeline(timelineId);
    if (success) {
      showNotification("Timeline locked!", "success");
    }
  };

  return (
    <div className="game-container">
      <EnergyPanel /> {/* Shows: 12/15 energy */}
      <ChessBoard />
      <TimelineControlPanel onLockTimeline={handleLockTimeline} />
      <TimelineGraph />
    </div>
  );
}
```

---

## 🐛 Debugging

### Check Backend
```bash
# View player energy in database:
SELECT * FROM player_energy WHERE game_id = '{gameID}';

# Check timeline metadata:
SELECT * FROM timeline_metadata WHERE game_id = '{gameID}' ORDER BY stability_score;

# Check energy transactions (audit):
SELECT * FROM energy_transactions WHERE game_id = '{gameID}' ORDER BY created_at DESC;
```

### Check Frontend Store
```typescript
// In browser console:
import { useGameStore } from "@/store/gameStore";
const state = useGameStore.getState();
console.log(state.playerEnergy);
console.log(state.timelineMetadata);
```

---

## 🔮 Future Enhancements

- **Phase 6**: Replay system with energy costs
- **Phase 7**: AI analysis using energy to evaluate timelines
- **Phase 8**: Ranked ladder with energy-based skill rating
- **Quantum Mechanics**: Time paradoxes affecting stability differently
- **Energy Recovery**: Passive energy regen based on game length
- **Cosmetics**: Energy skin packs, visual effects for locks/collapses

---

## 📖 References

- PLAN.md - Phase 5 specifications
- Chess Wess Architecture - Backend & Frontend structure
- Go Chess Library - `github.com/notnil/chess`
- Zustand Documentation - State management

---

**Status**: ✅ **Complete** - All Phase 5 core features implemented
**Last Updated**: May 16, 2026
**Next Phase**: Phase 6 - Spectator & Replay System
