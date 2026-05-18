import { useCallback, useState } from "react";
import { useGameStore, PlayerEnergy, TimelineMetadata } from "../store/gameStore";
import { useAuthStore } from "../store/authStore";
import {
  spendEnergy,
  lockTimeline as lockTimelineAPI,
  fetchPlayerEnergy,
  ENERGY_COSTS,
  calculateRewindCost,
  hasEnoughEnergy,
} from "../utils/energy";

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
  const token = useAuthStore((state) => state.token);

  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const refreshEnergy = useCallback(async () => {
    if (!gameId || !token) return;
    try {
      setLoading(true);
      const energy = await fetchPlayerEnergy(token, gameId);
      setPlayerEnergy(energy);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to refresh energy");
    } finally {
      setLoading(false);
    }
  }, [gameId, token, setPlayerEnergy]);

  /**
   * Rewind to N turns back in current timeline
   * Costs N energy
   */
  const rewindTimeline = useCallback(
    async (turnsBack: number, timelineId: string): Promise<boolean> => {
      if (!gameId || !token || !playerEnergy) return false;

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
          token,
          gameId,
          cost,
          "rewind",
          `Rewound ${turnsBack} turns in timeline ${timelineId}`
        );
        setPlayerEnergy(updated);
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
    [gameId, token, playerEnergy, setPlayerEnergy]
  );

  /**
   * Jump to a different timeline
   * Costs 1 energy
   */
  const jumpTimeline = useCallback(
    async (targetTimelineId: string): Promise<boolean> => {
      if (!gameId || !token || !playerEnergy) return false;

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
          token,
          gameId,
          cost,
          "jump_timeline",
          `Jumped to timeline ${targetTimelineId}`
        );
        setPlayerEnergy(updated);
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
    [gameId, token, playerEnergy, setPlayerEnergy]
  );

  /**
   * Lock a timeline (prevents opponent rewinding into it)
   * Costs 3 energy
   */
  const lockTimeline = useCallback(
    async (timelineId: string): Promise<boolean> => {
      if (!gameId || !token || !playerEnergy) return false;

      const cost = ENERGY_COSTS.LOCK_TIMELINE;

      if (!hasEnoughEnergy(playerEnergy.energy_remaining, cost)) {
        setError(
          `Insufficient energy: need ${cost}, have ${playerEnergy.energy_remaining}`
        );
        return false;
      }

      try {
        setLoading(true);
        const timelineMeta = await lockTimelineAPI(token, gameId, timelineId);
        const updatedEnergy = await fetchPlayerEnergy(token, gameId);
        setPlayerEnergy(updatedEnergy);
        updateTimelineMetadata(timelineId, timelineMeta);
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
    [gameId, token, playerEnergy, setPlayerEnergy, updateTimelineMetadata]
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
