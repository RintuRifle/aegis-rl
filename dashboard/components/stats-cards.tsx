"use client";

import { StatsResponse, formatNumber } from "@/lib/api";

interface StatsCardsProps {
  stats: StatsResponse | null;
}

interface CardData {
  label: string;
  value: string;
  icon: string;
  color: string;
  gradient: string;
  subtext?: string;
}

export function StatsCards({ stats }: StatsCardsProps) {
  const cards: CardData[] = [
    {
      label: "Total Requests",
      value: stats ? formatNumber(stats.total) : "—",
      icon: "📊",
      color: "text-indigo-400",
      gradient: "from-indigo-500/20 to-indigo-500/5",
      subtext: stats ? `${formatNumber(stats.total)} processed` : undefined,
    },
    {
      label: "Allowed",
      value: stats ? formatNumber(stats.allowed) : "—",
      icon: "✅",
      color: "text-emerald-400",
      gradient: "from-emerald-500/20 to-emerald-500/5",
      subtext: stats
        ? `${stats.total > 0 ? ((stats.allowed / stats.total) * 100).toFixed(1) : 0}% pass rate`
        : undefined,
    },
    {
      label: "Denied (429s)",
      value: stats ? formatNumber(stats.denied) : "—",
      icon: "🛡️",
      color: "text-red-400",
      gradient: "from-red-500/20 to-red-500/5",
      subtext: stats
        ? `${stats.total > 0 ? ((stats.denied / stats.total) * 100).toFixed(1) : 0}% throttled`
        : undefined,
    },
    {
      label: "Degraded Mode",
      value: stats ? formatNumber(stats.degraded) : "—",
      icon: "⚠️",
      color: "text-amber-400",
      gradient: "from-amber-500/20 to-amber-500/5",
      subtext:
        stats && stats.degraded > 0
          ? "Local fallback active"
          : "Redis healthy",
    },
  ];

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
      {cards.map((card) => (
        <div key={card.label} className="glass-card p-5 group">
          <div className="flex items-start justify-between mb-3">
            <span className="text-xs font-medium text-[var(--aegis-text-muted)] uppercase tracking-wider">
              {card.label}
            </span>
            <span className="text-lg">{card.icon}</span>
          </div>
          <div className={`text-3xl font-bold ${card.color} count-animate mb-1`}>
            {card.value}
          </div>
          {card.subtext && (
            <p className="text-xs text-[var(--aegis-text-muted)]">
              {card.subtext}
            </p>
          )}
          {/* Bottom gradient accent */}
          <div
            className={`absolute bottom-0 left-0 right-0 h-0.5 bg-gradient-to-r ${card.gradient} rounded-b-2xl opacity-0 group-hover:opacity-100 transition-opacity`}
          />
        </div>
      ))}
    </div>
  );
}
