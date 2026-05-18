import { api } from "./api";

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
export async function fetchPlayerEnergy(token: string, gameId: string): Promise<any> {
  return api.getPlayerEnergy(token, gameId);
}

/**
 * Spends energy for an action
 * Returns updated player energy
 */
export async function spendEnergy(
  token: string,
  gameId: string,
  amount: number,
  action: "rewind" | "jump_timeline" | "lock" | "paradox_penalty",
  details: string
): Promise<any> {
  return api.spendEnergy(token, gameId, amount, action, details);
}

/**
 * Refunds energy (e.g., invalid action)
 */
export async function refundEnergy(
  token: string,
  gameId: string,
  amount: number,
  reason: string
): Promise<any> {
  return api.refundEnergy(token, gameId, amount, reason);
}

/**
 * Locks a timeline (prevents opponent rewinding into it)
 */
export async function lockTimeline(
  token: string,
  gameId: string,
  timelineId: string
): Promise<any> {
  return api.lockTimeline(token, gameId, timelineId);
}

/**
 * Gets timeline status (lock, stability, paradoxes)
 */
export async function getTimelineStatus(
  token: string,
  gameId: string,
  timelineId: string
): Promise<any> {
  return api.getTimelineStatus(token, gameId, timelineId);
}

/**
 * Gets full energy status (player energy + all timelines)
 */
export async function getEnergyStatus(token: string, gameId: string): Promise<any> {
  return api.getEnergyStatus(token, gameId);
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
