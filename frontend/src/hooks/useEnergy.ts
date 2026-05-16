import { useCallback, useState } from "react";
import { useGameStore, PlayerEnergy, TimelineMetadata } from "@/store/gameStore";
import {
  spendEnergy,
  lockTimeline as lockTimelineAPI,
  fetchPlayerEnergy,
  ENERGY_COSTS,
  calculateRewindCost,
  hasEnoughEnergy,
} from "@/utils/energy";

interface UseEnergyReturn {
  playerEnergy: PlayerEnergy | null;
  timelineMetadata: Record<string, TimelineMetadata>;
  loading: boolean;
  error: string | null;
  refreshEnergy: () => Promise<void>;
  rewindTimeline: (turnsBack: number, timelineId: string) => Promise<boolean>;
  jumpTimeline: (targetTimelineId: string) => Promise<boolean>;
  lockTimeline: (timelineId: string) => Promise<boolean>;
  getTimelineStrength: (timelineId: string) => number;
}

/**
 * Custom hook for managing Phase 5 energy operations
 * Handles spending, refunding, locking, and UI state
 */
export function useEnergy(): UseEnergyReturn {
  const gameId = useGameStore((state) => state.activeGameId);
  const playerEnergy = useGameStore((state) => state.playerEnergy);
  const timelineMetadata = useGameStore((state) => state.timelineMetadata);
  const setPlayerEnergy = useGameStore((state) => state.setPlayerEnergy);
  const updateTimelineMetadata = useGameStore((state) => state.updateTimelineMetadata);
  const consumeEnergy = useGameStore((state) => state.consumeEnergy);

  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const refreshEnergy = useCallback(async () => {
    if (!gameId) return;
    try {
      setLoading(true);
      const energy = await fetchPlayerEnergy(gameId);
      setPlayerEnergy(energy);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to refresh energy");
    } finally {
      setLoading(false);
    }
  }, [gameId, setPlayerEnergy]);

  /**
   * Rewind to N turns back in current timeline
   * Costs N energy
   */
  const rewindTimeline = useCallback(
    async (turnsBack: number, timelineId: string): Promise<boolean> => {
      if (!gameId || !playerEnergy) return false;

      const cost = calculateRewindCost(turnsBack);

      if (!hasEnoughEnergy(playerEnergy.energy_remaining, cost)) {
        setError(
          `Insufficient energy: need ${cost}, have ${playerEnergy.energy_remaining}`
        );
        return false;
      }

      try {
        setLoading(true);
        const updated = await spendEnergy(
          gameId,
          cost,
          "rewind",
          `Rewound ${turnsBack} turns in timeline ${timelineId}`
        );
        setPlayerEnergy(updated);
        consumeEnergy(cost);
        setError(null);
        return true;
      } catch (err) {
        const msg = err instanceof Error ? err.message : "Failed to rewind";
        setError(msg);
        return false;
      } finally {
        setLoading(false);
      }
    },
    [gameId, playerEnergy, setPlayerEnergy, consumeEnergy]
  );

  /**
   * Jump to a different timeline
   * Costs 1 energy
   */
  const jumpTimeline = useCallback(
    async (targetTimelineId: string): Promise<boolean> => {
      if (!gameId || !playerEnergy) return false;

      const cost = ENERGY_COSTS.JUMP_TIMELINE;

      if (!hasEnoughEnergy(playerEnergy.energy_remaining, cost)) {
        setError(
          `Insufficient energy: need ${cost}, have ${playerEnergy.energy_remaining}`
        );
        return false;
      }

      try {
        setLoading(true);
        const updated = await spendEnergy(
          gameId,
          cost,
          "jump_timeline",
          `Jumped to timeline ${targetTimelineId}`
        );
        setPlayerEnergy(updated);
        consumeEnergy(cost);
        setError(null);
        return true;
      } catch (err) {
        const msg = err instanceof Error ? err.message : "Failed to jump timeline";
        setError(msg);
        return false;
      } finally {
        setLoading(false);
      }
    },
    [gameId, playerEnergy, setPlayerEnergy, consumeEnergy]
  );

  /**
   * Lock a timeline (prevents opponent rewinding into it)
   * Costs 3 energy
   */
  const lockTimeline = useCallback(
    async (timelineId: string): Promise<boolean> => {
      if (!gameId || !playerEnergy) return false;

      const cost = ENERGY_COSTS.LOCK_TIMELINE;

      if (!hasEnoughEnergy(playerEnergy.energy_remaining, cost)) {
        setError(
          `Insufficient energy: need ${cost}, have ${playerEnergy.energy_remaining}`
        );
        return false;
      }

      try {
        setLoading(true);
        const updated = await lockTimelineAPI(gameId, timelineId);
        setPlayerEnergy(updated.player_energy || playerEnergy);
        updateTimelineMetadata(timelineId, {
          is_locked: true,
          locked_by_player_id: playerEnergy.player_id,
        });
        consumeEnergy(cost);
        setError(null);
        return true;
      } catch (err) {
        const msg = err instanceof Error ? err.message : "Failed to lock timeline";
        setError(msg);
        return false;
      } finally {
        setLoading(false);
      }
    },
    [gameId, playerEnergy, setPlayerEnergy, updateTimelineMetadata, consumeEnergy]
  );

  /**
   * Calculate timeline strength (for sorting in collapse)
   * Lower score = weaker (collapsed first)
   */
  const getTimelineStrength = useCallback(
    (timelineId: string): number => {
      const meta = timelineMetadata[timelineId];
      if (!meta) return 0;

      // Locked timelines are always strong (survive collapse)
      if (meta.is_locked) return 1000;

      // Strength = stability - paradox penalty
      return meta.stability_score - meta.paradox_count * 10;
    },
    [timelineMetadata]
  );

  return {
    playerEnergy,
    timelineMetadata,
    loading,
    error,
    refreshEnergy,
    rewindTimeline,
    jumpTimeline,
    lockTimeline,
    getTimelineStrength,
  };
}
