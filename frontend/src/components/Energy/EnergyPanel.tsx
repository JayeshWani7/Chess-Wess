import React from "react";
import { useGameStore } from "@/store/gameStore";

interface EnergyPanelProps {
  className?: string;
}

/**
 * EnergyPanel displays the current player's energy pool
 * Phase 5: Time Mechanics - shows energy remaining and spent
 */
export const EnergyPanel: React.FC<EnergyPanelProps> = ({ className = "" }) => {
  const playerEnergy = useGameStore((state) => state.playerEnergy);

  if (!playerEnergy) {
    return null;
  }

  const energyPercentage = (playerEnergy.energy_remaining / 15) * 100; // Max 15 energy

  return (
    <div className={`bg-gray-800 rounded-lg p-4 border border-purple-500 ${className}`}>
      <div className="flex items-center justify-between mb-2">
        <h3 className="text-sm font-bold text-white">TIME ENERGY</h3>
        <span className="text-xs text-purple-400">Phase 5</span>
      </div>

      {/* Energy Bar */}
      <div className="w-full bg-gray-700 rounded-full h-6 overflow-hidden border border-purple-400 mb-2">
        <div
          className={`h-full transition-all duration-300 ${
            energyPercentage > 50
              ? "bg-gradient-to-r from-blue-500 to-cyan-400"
              : energyPercentage > 25
              ? "bg-gradient-to-r from-yellow-500 to-orange-400"
              : "bg-gradient-to-r from-red-600 to-red-500"
          }`}
          style={{ width: `${energyPercentage}%` }}
        />
      </div>

      {/* Energy Stats */}
      <div className="grid grid-cols-2 gap-2 text-xs mb-3">
        <div className="bg-gray-700 p-2 rounded border border-gray-600">
          <p className="text-gray-400">Remaining</p>
          <p className="text-xl font-bold text-cyan-400">{playerEnergy.energy_remaining}</p>
        </div>
        <div className="bg-gray-700 p-2 rounded border border-gray-600">
          <p className="text-gray-400">Spent</p>
          <p className="text-xl font-bold text-orange-400">{playerEnergy.energy_spent}</p>
        </div>
      </div>

      {/* Energy Cost Info */}
      <div className="bg-gray-900 rounded p-2 text-xs text-gray-300 space-y-1 border border-gray-700">
        <p>💰 <span className="text-cyan-400 font-semibold">Rewind</span>: 1 energy/turn</p>
        <p>🌊 <span className="text-blue-400 font-semibold">Jump Timeline</span>: 1 energy</p>
        <p>🔒 <span className="text-purple-400 font-semibold">Lock Timeline</span>: 3 energy</p>
      </div>
    </div>
  );
};

/**
 * TimelineStatusCard shows metadata for a single timeline
 * Displays: lock status, stability score, paradoxes, collapse status
 */
interface TimelineStatusCardProps {
  timelineId: string;
  timelineName?: string;
  className?: string;
}

export const TimelineStatusCard: React.FC<TimelineStatusCardProps> = ({
  timelineId,
  timelineName = "Timeline",
  className = "",
}) => {
  const timelineMetadata = useGameStore((state) => state.timelineMetadata[timelineId]);
  const playerEnergy = useGameStore((state) => state.playerEnergy);

  if (!timelineMetadata) {
    return null;
  }

  const stabilityColor =
    timelineMetadata.stability_score > 70
      ? "text-green-400"
      : timelineMetadata.stability_score > 40
      ? "text-yellow-400"
      : "text-red-400";

  return (
    <div
      className={`bg-gray-800 rounded border ${
        timelineMetadata.is_locked ? "border-red-500" : "border-gray-600"
      } p-3 ${className}`}
    >
      <div className="flex items-start justify-between mb-2">
        <div>
          <h4 className="text-sm font-bold text-white">{timelineName}</h4>
          <p className="text-xs text-gray-400">ID: {timelineId.slice(0, 8)}...</p>
        </div>
        {timelineMetadata.is_locked && (
          <span className="bg-red-600 text-white text-xs px-2 py-1 rounded font-bold">
            🔒 LOCKED
          </span>
        )}
      </div>

      <div className="space-y-2 text-xs">
        {/* Stability Score */}
        <div>
          <div className="flex justify-between mb-1">
            <span className="text-gray-300">Stability</span>
            <span className={`font-bold ${stabilityColor}`}>
              {timelineMetadata.stability_score}%
            </span>
          </div>
          <div className="w-full bg-gray-700 rounded-full h-3 overflow-hidden">
            <div
              className={`h-full transition-all ${
                timelineMetadata.stability_score > 70
                  ? "bg-green-600"
                  : timelineMetadata.stability_score > 40
                  ? "bg-yellow-600"
                  : "bg-red-600"
              }`}
              style={{ width: `${timelineMetadata.stability_score}%` }}
            />
          </div>
        </div>

        {/* Paradox Count & Collapse Status */}
        <div className="grid grid-cols-2 gap-2">
          <div className="bg-gray-700 p-2 rounded">
            <p className="text-gray-400">Paradoxes</p>
            <p className="text-lg font-bold text-red-400">{timelineMetadata.paradox_count}</p>
          </div>
          <div className="bg-gray-700 p-2 rounded">
            <p className="text-gray-400">Status</p>
            <p className="text-sm font-bold">
              {timelineMetadata.is_collapsed ? (
                <span className="text-gray-500">COLLAPSED</span>
              ) : (
                <span className="text-green-400">ACTIVE</span>
              )}
            </p>
          </div>
        </div>

        {/* Energy Cost Info */}
        <div className="bg-gray-900 p-2 rounded border border-gray-700">
          <p className="text-gray-400">Energy to Create</p>
          <p className="text-cyan-400 font-bold">{timelineMetadata.energy_cost_to_create}</p>
        </div>
      </div>
    </div>
  );
};

/**
 * TimelineControlPanel allows locking timelines and viewing timeline health
 * Phase 5: Time Mechanics - Branch Locking
 */
interface TimelineControlPanelProps {
  onLockTimeline?: (timelineId: string) => void;
  className?: string;
}

export const TimelineControlPanel: React.FC<TimelineControlPanelProps> = ({
  onLockTimeline,
  className = "",
}) => {
  const timelines = useGameStore((state) => state.timelines);
  const timelineMetadata = useGameStore((state) => state.timelineMetadata);
  const playerEnergy = useGameStore((state) => state.playerEnergy);
  const activeTimelineId = useGameStore((state) => state.activeTimelineId);

  if (timelines.length === 0) {
    return null;
  }

  const totalTimelines = timelines.length;
  const timelinesToCollapse = Math.max(0, totalTimelines - 30);

  return (
    <div className={`space-y-3 ${className}`}>
      {/* Collapse Warning */}
      {timelinesToCollapse > 0 && (
        <div className="bg-red-900 border border-red-600 rounded p-3">
          <p className="text-sm text-red-200">
            ⚠️ <span className="font-bold">{timelinesToCollapse} timeline(s)</span> will collapse!
          </p>
          <p className="text-xs text-red-300 mt-1">
            {totalTimelines} / 30 timelines • Weakest first
          </p>
        </div>
      )}

      {/* Timeline List */}
      <div className="space-y-2">
        {timelines.map((timeline) => {
          const meta = timelineMetadata[timeline.timeline_id];
          const isActive = timeline.timeline_id === activeTimelineId;
          const canLock = playerEnergy && playerEnergy.energy_remaining >= 3;

          return (
            <div
              key={timeline.timeline_id}
              className={`bg-gray-800 rounded p-3 border-2 transition-all ${
                isActive ? "border-cyan-500" : "border-gray-600"
              } ${meta?.is_collapsed ? "opacity-50" : ""}`}
            >
              <div className="flex items-center justify-between mb-2">
                <div className="flex-1">
                  <h4 className="text-sm font-bold text-white">
                    {timeline.timeline_name || "Timeline"}
                  </h4>
                  <p className="text-xs text-gray-400">Moves: {timeline.node_count || 0}</p>
                </div>
                {isActive && (
                  <span className="bg-cyan-600 text-white text-xs px-2 py-1 rounded font-bold">
                    ACTIVE
                  </span>
                )}
              </div>

              {/* Lock Button */}
              {!meta?.is_locked && !meta?.is_collapsed && onLockTimeline && (
                <button
                  onClick={() => onLockTimeline(timeline.timeline_id)}
                  disabled={!canLock}
                  className={`w-full text-xs font-bold py-2 rounded transition-all ${
                    canLock
                      ? "bg-purple-600 hover:bg-purple-700 text-white cursor-pointer"
                      : "bg-gray-700 text-gray-500 cursor-not-allowed"
                  }`}
                >
                  🔒 Lock Timeline (3 energy)
                </button>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
};

/**
 * OpponentEnergyPanel displays the opponent's energy pool
 * Phase 5: Time Mechanics - shows opponent's energy remaining for bots and players
 */
interface OpponentEnergyPanelProps {
  opponentName?: string;
  isBot?: boolean;
  botRating?: number;
  className?: string;
}

export const OpponentEnergyPanel: React.FC<OpponentEnergyPanelProps> = ({
  opponentName = "Opponent",
  isBot = false,
  botRating = 0,
  className = "",
}) => {
  const opponentEnergy = useGameStore((state) => state.opponentEnergy);

  if (!opponentEnergy) {
    return null;
  }

  const energyPercentage = (opponentEnergy.energy_remaining / 15) * 100; // Max 15 energy

  const botBadge = isBot ? (
    <span className="text-xs bg-amber-900 text-amber-300 px-2 py-1 rounded font-bold border border-amber-600">
      Bot {botRating}
    </span>
  ) : null;

  return (
    <div className={`bg-gray-800 rounded-lg p-4 border border-red-500 ${className}`}>
      <div className="flex items-center justify-between mb-2">
        <h3 className="text-sm font-bold text-white">{opponentName.toUpperCase()}</h3>
        <div className="flex gap-2 items-center">{botBadge}</div>
      </div>

      {/* Opponent Energy Bar */}
      <div className="w-full bg-gray-700 rounded-full h-6 overflow-hidden border border-red-400 mb-2">
        <div
          className={`h-full transition-all duration-300 ${
            energyPercentage > 50
              ? "bg-gradient-to-r from-blue-500 to-cyan-400"
              : energyPercentage > 25
              ? "bg-gradient-to-r from-yellow-500 to-orange-400"
              : "bg-gradient-to-r from-red-600 to-red-500"
          }`}
          style={{ width: `${energyPercentage}%` }}
        />
      </div>

      {/* Opponent Energy Stats */}
      <div className="grid grid-cols-2 gap-2 text-xs">
        <div className="bg-gray-700 p-2 rounded border border-gray-600">
          <p className="text-gray-400">Remaining</p>
          <p className="text-xl font-bold text-cyan-400">{opponentEnergy.energy_remaining}</p>
        </div>
        <div className="bg-gray-700 p-2 rounded border border-gray-600">
          <p className="text-gray-400">Spent</p>
          <p className="text-xl font-bold text-orange-400">{opponentEnergy.energy_spent}</p>
        </div>
      </div>
    </div>
  );
};

/**
 * EnergyNotification shows warnings when energy is low or actions are blocked
 */
interface EnergyNotificationProps {
  message: string;
  type: "warning" | "error" | "info";
  onDismiss?: () => void;
}

export const EnergyNotification: React.FC<EnergyNotificationProps> = ({
  message,
  type,
  onDismiss,
}) => {
  const bgColor = {
    warning: "bg-yellow-900 border-yellow-600",
    error: "bg-red-900 border-red-600",
    info: "bg-blue-900 border-blue-600",
  }[type];

  const textColor = {
    warning: "text-yellow-200",
    error: "text-red-200",
    info: "text-blue-200",
  }[type];

  return (
    <div className={`${bgColor} border rounded p-3 flex justify-between items-center`}>
      <p className={`text-sm ${textColor}`}>{message}</p>
      {onDismiss && (
        <button
          onClick={onDismiss}
          className="text-gray-300 hover:text-white text-lg"
        >
          ✕
        </button>
      )}
    </div>
  );
};
