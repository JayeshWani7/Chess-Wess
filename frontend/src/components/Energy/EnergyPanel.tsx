import React from "react";
import { useGameStore } from "../../store/gameStore";

interface EnergyPanelProps {
  className?: string;
}

export const EnergyPanel: React.FC<EnergyPanelProps> = ({ className = "" }) => {
  const playerEnergy = useGameStore((state) => state.playerEnergy);

  if (!playerEnergy) {
    return null;
  }

  const energyPercentage = (playerEnergy.energy_remaining / 15) * 100;

  return (
    <div className={`card ${className}`}>
      <div className="flex items-center justify-between mb-2">
        <h3 className="text-sm font-semibold text-ink">Time Energy</h3>
        <span className="text-xs text-gold">Energy</span>
      </div>
      <div className="w-full bg-mist rounded-full h-6 overflow-hidden border border-line mb-2">
        <div
          className={`h-full transition-all duration-300 ${
            energyPercentage > 50
              ? "bg-gradient-to-r from-leaf to-gold"
              : energyPercentage > 25
              ? "bg-gradient-to-r from-gold to-amber-300"
              : "bg-gradient-to-r from-rust to-amber-300"
          }`}
          style={{ width: `${energyPercentage}%` }}
        />
      </div>
      <div className="grid grid-cols-2 gap-2 text-xs mb-3">
        <div className="glass p-2 rounded">
          <p className="text-moss">Remaining</p>
          <p className="text-xl font-semibold text-leaf">{playerEnergy.energy_remaining}</p>
        </div>
        <div className="glass p-2 rounded">
          <p className="text-moss">Spent</p>
          <p className="text-xl font-semibold text-gold">{playerEnergy.energy_spent}</p>
        </div>
      </div>
      <div className="glass rounded p-2 text-xs text-ink space-y-1">
        <p>Rewind: <span className="text-pine font-semibold">1 energy/turn</span></p>
        <p>Jump Timeline: <span className="text-pine font-semibold">1 energy</span></p>
        <p>Lock Timeline: <span className="text-pine font-semibold">3 energy</span></p>
      </div>
    </div>
  );
};

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
      ? "text-leaf"
      : timelineMetadata.stability_score > 40
      ? "text-gold"
      : "text-rust";

  return (
    <div
      className={`card ${timelineMetadata.is_locked ? "border-rust/70" : ""} p-3 ${className}`}
    >
      <div className="flex items-start justify-between mb-2">
        <div>
          <h4 className="text-sm font-semibold text-ink">{timelineName}</h4>
          <p className="text-xs text-moss">ID: {timelineId.slice(0, 8)}...</p>
        </div>
        {timelineMetadata.is_locked && (
          <span className="bg-rust text-paper text-xs px-2 py-1 rounded font-bold">
            LOCKED
          </span>
        )}
      </div>

      <div className="space-y-2 text-xs">
        <div>
          <div className="flex justify-between mb-1">
            <span className="text-ink">Stability</span>
            <span className={`font-bold ${stabilityColor}`}>
              {timelineMetadata.stability_score}%
            </span>
          </div>
          <div className="w-full bg-mist rounded-full h-3 overflow-hidden">
            <div
              className={`h-full transition-all ${
                timelineMetadata.stability_score > 70
                  ? "bg-leaf"
                  : timelineMetadata.stability_score > 40
                  ? "bg-gold"
                  : "bg-rust"
              }`}
              style={{ width: `${timelineMetadata.stability_score}%` }}
            />
          </div>
        </div>
        <div className="grid grid-cols-2 gap-2">
          <div className="glass p-2 rounded">
            <p className="text-moss">Paradoxes</p>
            <p className="text-lg font-semibold text-rust">{timelineMetadata.paradox_count}</p>
          </div>
          <div className="glass p-2 rounded">
            <p className="text-moss">Status</p>
            <p className="text-sm font-semibold">
              {timelineMetadata.is_collapsed ? (
                <span className="text-moss">COLLAPSED</span>
              ) : (
                <span className="text-leaf">ACTIVE</span>
              )}
            </p>
          </div>
        </div>
        <div className="glass p-2 rounded">
          <p className="text-moss">Energy to Create</p>
          <p className="text-pine font-semibold">{timelineMetadata.energy_cost_to_create}</p>
        </div>
      </div>
    </div>
  );
};

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
      {timelinesToCollapse > 0 && (
        <div className="bg-rust/10 border border-rust/50 rounded p-3">
          <p className="text-sm text-rust">
            <span className="font-bold">{timelinesToCollapse} timeline(s)</span> will collapse.
          </p>
          <p className="text-xs text-moss mt-1">
            {totalTimelines} / 30 timelines • Weakest first
          </p>
        </div>
      )}
      <div className="space-y-2">
        {timelines.map((timeline) => {
          const meta = timelineMetadata[timeline.timeline_id];
          const isActive = timeline.timeline_id === activeTimelineId;
          const canLock = playerEnergy && playerEnergy.energy_remaining >= 3;

          return (
            <div
              key={timeline.timeline_id}
              className={`glass rounded-xl p-3 border-2 transition-all ${
                isActive ? "border-gold" : "border-line"
              } ${meta?.is_collapsed ? "opacity-50" : ""}`}
            >
              <div className="flex items-center justify-between mb-2">
                <div className="flex-1">
                  <h4 className="text-sm font-semibold text-ink">
                    {timeline.timeline_name || "Timeline"}
                  </h4>
                  <p className="text-xs text-moss">Moves: {timeline.node_count || 0}</p>
                </div>
                {isActive && (
                  <span className="bg-gold text-ink text-xs px-2 py-1 rounded font-bold">
                    ACTIVE
                  </span>
                )}
              </div>
              {!meta?.is_locked && !meta?.is_collapsed && onLockTimeline && (
                <button
                  onClick={() => onLockTimeline(timeline.timeline_id)}
                  disabled={!canLock}
                  className={`w-full text-xs font-bold py-2 rounded transition-all ${
                    canLock
                      ? "bg-gold/90 hover:bg-gold text-ink cursor-pointer"
                      : "bg-mist text-moss cursor-not-allowed"
                  }`}
                >
                  Lock Timeline (3 energy)
                </button>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
};

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

  const energyPercentage = (opponentEnergy.energy_remaining / 15) * 100;

  const botBadge = isBot ? (
    <span className="text-xs bg-gold/20 text-pine px-2 py-1 rounded font-bold border border-gold/60">
      Bot {botRating}
    </span>
  ) : null;

  return (
    <div className={`card ${className}`}>
      <div className="flex items-center justify-between mb-2">
        <h3 className="text-sm font-semibold text-ink">{opponentName.toUpperCase()}</h3>
        <div className="flex gap-2 items-center">{botBadge}</div>
      </div>
      <div className="w-full bg-mist rounded-full h-6 overflow-hidden border border-line mb-2">
        <div
          className={`h-full transition-all duration-300 ${
            energyPercentage > 50
              ? "bg-gradient-to-r from-leaf to-gold"
              : energyPercentage > 25
              ? "bg-gradient-to-r from-gold to-amber-300"
              : "bg-gradient-to-r from-rust to-amber-300"
          }`}
          style={{ width: `${energyPercentage}%` }}
        />
      </div>
      <div className="grid grid-cols-2 gap-2 text-xs">
        <div className="glass p-2 rounded">
          <p className="text-moss">Remaining</p>
          <p className="text-xl font-semibold text-leaf">{opponentEnergy.energy_remaining}</p>
        </div>
        <div className="glass p-2 rounded">
          <p className="text-moss">Spent</p>
          <p className="text-xl font-semibold text-gold">{opponentEnergy.energy_spent}</p>
        </div>
      </div>
    </div>
  );
};

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
    warning: "bg-gold/15 border-gold/50",
    error: "bg-rust/15 border-rust/50",
    info: "bg-leaf/10 border-leaf/40",
  }[type];

  const textColor = {
    warning: "text-pine",
    error: "text-rust",
    info: "text-leaf",
  }[type];

  return (
    <div className={`${bgColor} border rounded p-3 flex justify-between items-center`}>
      <p className={`text-sm ${textColor}`}>{message}</p>
      {onDismiss && (
        <button
          onClick={onDismiss}
          className="text-moss hover:text-ink text-lg"
        >
          ✕
        </button>
      )}
    </div>
  );
};
