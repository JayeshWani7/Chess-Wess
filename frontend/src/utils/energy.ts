import { api } from "./api";

export const ENERGY_COSTS = {
  REWIND_PER_TURN: 1,
  JUMP_TIMELINE: 1,
  LOCK_TIMELINE: 3,
  PARADOX_PENALTY: 2,
  INITIAL_POOL: 15,
  COLLAPSE_THRESHOLD: 30,
} as const;

export function calculateRewindCost(turnsBack: number): number {
  return turnsBack * ENERGY_COSTS.REWIND_PER_TURN;
}

export async function fetchPlayerEnergy(token: string, gameId: string): Promise<any> {
  return api.getPlayerEnergy(token, gameId);
}

export async function spendEnergy(
  token: string,
  gameId: string,
  amount: number,
  action: "rewind" | "jump_timeline" | "lock" | "paradox_penalty",
  details: string
): Promise<any> {
  return api.spendEnergy(token, gameId, amount, action, details);
}

export async function refundEnergy(
  token: string,
  gameId: string,
  amount: number,
  reason: string
): Promise<any> {
  return api.refundEnergy(token, gameId, amount, reason);
}

export async function lockTimeline(
  token: string,
  gameId: string,
  timelineId: string
): Promise<any> {
  return api.lockTimeline(token, gameId, timelineId);
}

export async function getTimelineStatus(
  token: string,
  gameId: string,
  timelineId: string
): Promise<any> {
  return api.getTimelineStatus(token, gameId, timelineId);
}

export async function getEnergyStatus(token: string, gameId: string): Promise<any> {
  return api.getEnergyStatus(token, gameId);
}

export function hasEnoughEnergy(currentEnergy: number, costNeeded: number): boolean {
  return currentEnergy >= costNeeded;
}

export function getEnergyColor(percentage: number): string {
  if (percentage > 50) return "text-green-400";
  if (percentage > 25) return "text-yellow-400";
  return "text-red-400";
}

export function shouldCheckCollapse(timelineCount: number): boolean {
  return timelineCount >= ENERGY_COSTS.COLLAPSE_THRESHOLD;
}

export function isTimelineWeak(stabilityScore: number, paradoxCount: number): boolean {
  return stabilityScore < 40 || paradoxCount > 5;
}

export function calculateTimelineStrength(
  stabilityScore: number,
  paradoxCount: number,
  isLocked: boolean
): number {
  if (isLocked) return 1000;

  return stabilityScore - paradoxCount * 10;
}
