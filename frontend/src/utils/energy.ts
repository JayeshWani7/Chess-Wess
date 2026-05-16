import { api } from "@/utils/api";

/**
 * Energy cost constants for Phase 5
 */
export const ENERGY_COSTS = {
  REWIND_PER_TURN: 1,
  JUMP_TIMELINE: 1,
  LOCK_TIMELINE: 3,
  PARADOX_PENALTY: 2,
  INITIAL_POOL: 15,
  COLLAPSE_THRESHOLD: 30,
} as const;

/**
 * Calculates energy cost for rewinding N turns
 */
export function calculateRewindCost(turnsBack: number): number {
  return turnsBack * ENERGY_COSTS.REWIND_PER_TURN;
}

/**
 * Fetches current player energy from server
 */
export async function fetchPlayerEnergy(gameId: string): Promise<any> {
  const response = await api(`/games/${gameId}/energy`);
  return response;
}

/**
 * Spends energy for an action
 * Returns updated player energy
 */
export async function spendEnergy(
  gameId: string,
  amount: number,
  action: "rewind" | "jump_timeline" | "lock" | "paradox_penalty",
  details: string
): Promise<any> {
  const response = await api(`/games/${gameId}/energy/spend`, {
    method: "POST",
    body: JSON.stringify({
      amount,
      action,
      details,
    }),
  });
  return response;
}

/**
 * Refunds energy (e.g., invalid action)
 */
export async function refundEnergy(
  gameId: string,
  amount: number,
  reason: string
): Promise<any> {
  const response = await api(`/games/${gameId}/energy/refund`, {
    method: "POST",
    body: JSON.stringify({
      amount,
      reason,
    }),
  });
  return response;
}

/**
 * Locks a timeline (prevents opponent rewinding into it)
 */
export async function lockTimeline(gameId: string, timelineId: string): Promise<any> {
  const response = await api(`/games/${gameId}/energy/lock-timeline`, {
    method: "POST",
    body: JSON.stringify({
      timeline_id: timelineId,
    }),
  });
  return response;
}

/**
 * Gets timeline status (lock, stability, paradoxes)
 */
export async function getTimelineStatus(gameId: string, timelineId: string): Promise<any> {
  const response = await api(`/games/${gameId}/energy/timeline-status?timeline_id=${timelineId}`);
  return response;
}

/**
 * Gets full energy status (player energy + all timelines)
 */
export async function getEnergyStatus(gameId: string): Promise<any> {
  const response = await api(`/games/${gameId}/energy/status`);
  return response;
}

/**
 * Checks if player has enough energy for action
 */
export function hasEnoughEnergy(currentEnergy: number, costNeeded: number): boolean {
  return currentEnergy >= costNeeded;
}

/**
 * Formats energy display with colors
 */
export function getEnergyColor(percentage: number): string {
  if (percentage > 50) return "text-green-400";
  if (percentage > 25) return "text-yellow-400";
  return "text-red-400";
}

/**
 * Checks if timelines should collapse (30+ exist)
 */
export function shouldCheckCollapse(timelineCount: number): boolean {
  return timelineCount >= ENERGY_COSTS.COLLAPSE_THRESHOLD;
}

/**
 * Checks if timeline is weak (low stability, many paradoxes)
 */
export function isTimelineWeak(stabilityScore: number, paradoxCount: number): boolean {
  return stabilityScore < 40 || paradoxCount > 5;
}

/**
 * Calculates timeline strength for sorting (weakest first)
 */
export function calculateTimelineStrength(
  stabilityScore: number,
  paradoxCount: number,
  isLocked: boolean
): number {
  // Locked timelines always survive
  if (isLocked) return 1000;
  
  // Score = stability - paradox penalty
  return stabilityScore - paradoxCount * 10;
}
