"use client";

import { StatsResponse, formatUptime } from "@/lib/api";

interface SystemStatusProps {
  stats: StatsResponse | null;
  connected: boolean;
}

export function SystemStatus({ stats, connected }: SystemStatusProps) {
  const items = [
    {
      label: "Engine Status",
      value: connected ? "Online" : "Offline",
      color: connected ? "text-emerald-400" : "text-red-400",
      dot: connected ? "bg-emerald-400" : "bg-red-400",
    },
    {
      label: "Redis Connection",
      value: stats && stats.degraded === 0 ? "Healthy" : "Degraded",
      color: stats && stats.degraded === 0 ? "text-emerald-400" : "text-amber-400",
      dot: stats && stats.degraded === 0 ? "bg-emerald-400" : "bg-amber-400",
    },
    {
      label: "Uptime",
      value: stats ? formatUptime(stats.uptime_seconds) : "—",
      color: "text-[var(--aegis-text)]",
      dot: "bg-indigo-400",
    },
    {
      label: "Algorithm",
      value: "Token Bucket",
      color: "text-[var(--aegis-text)]",
      dot: "bg-cyan-400",
    },
    {
      label: "Resilience",
      value: "Circuit Breaker + Fallback",
      color: "text-[var(--aegis-text)]",
      dot: "bg-indigo-400",
    },
    {
      label: "Atomicity",
      value: "Redis Lua (EVALSHA)",
      color: "text-[var(--aegis-text)]",
      dot: "bg-cyan-400",
    },
  ];

  return (
    <div className="glass-card p-5">
      <h2 className="text-sm font-semibold mb-4">System Status</h2>
      <div className="space-y-3">
        {items.map((item) => (
          <div key={item.label} className="flex items-center justify-between py-1.5">
            <div className="flex items-center gap-2">
              <div className={`w-2 h-2 rounded-full ${item.dot}`} />
              <span className="text-xs text-[var(--aegis-text-muted)]">{item.label}</span>
            </div>
            <span className={`text-xs font-medium ${item.color}`}>{item.value}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
