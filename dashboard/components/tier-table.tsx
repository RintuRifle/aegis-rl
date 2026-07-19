"use client";

import { useEffect, useState } from "react";
import { ConfigResponse, fetchConfig } from "@/lib/api";

export function TierTable() {
  const [config, setConfig] = useState<ConfigResponse | null>(null);

  useEffect(() => {
    fetchConfig().then(setConfig).catch(() => {});
  }, []);

  const tiers = config?.tiers
    ? Object.values(config.tiers).sort((a, b) => a.capacity - b.capacity)
    : [
        { name: "free", capacity: 100, refill_rate: 10 },
        { name: "pro", capacity: 1000, refill_rate: 100 },
        { name: "enterprise", capacity: 10000, refill_rate: 1000 },
      ];

  const badgeStyle: Record<string, string> = {
    free: "bg-zinc-500/20 text-zinc-400 border-zinc-500/30",
    pro: "bg-indigo-500/20 text-indigo-400 border-indigo-500/30",
    enterprise: "bg-amber-500/20 text-amber-400 border-amber-500/30",
  };

  return (
    <div className="glass-card p-5">
      <h2 className="text-sm font-semibold mb-4">Rate Limit Tiers</h2>
      <div className="overflow-hidden rounded-xl border border-[var(--aegis-border)]">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-[var(--aegis-surface)]">
              <th className="text-left py-2.5 px-4 text-xs font-medium text-[var(--aegis-text-muted)] uppercase tracking-wider">Tier</th>
              <th className="text-right py-2.5 px-4 text-xs font-medium text-[var(--aegis-text-muted)] uppercase tracking-wider">Burst</th>
              <th className="text-right py-2.5 px-4 text-xs font-medium text-[var(--aegis-text-muted)] uppercase tracking-wider">Rate (tok/s)</th>
              <th className="text-right py-2.5 px-4 text-xs font-medium text-[var(--aegis-text-muted)] uppercase tracking-wider">Full Refill</th>
            </tr>
          </thead>
          <tbody>
            {tiers.map((tier) => {
              const refillTime = tier.capacity / tier.refill_rate;
              return (
                <tr key={tier.name} className="border-t border-[var(--aegis-border)] hover:bg-[var(--aegis-surface-light)] transition-colors">
                  <td className="py-3 px-4">
                    <span className={`inline-flex px-2.5 py-0.5 rounded-full text-xs font-medium border ${badgeStyle[tier.name] || badgeStyle.free}`}>
                      {tier.name.charAt(0).toUpperCase() + tier.name.slice(1)}
                    </span>
                  </td>
                  <td className="py-3 px-4 text-right font-mono">{tier.capacity.toLocaleString()}</td>
                  <td className="py-3 px-4 text-right font-mono">{tier.refill_rate.toLocaleString()}</td>
                  <td className="py-3 px-4 text-right text-[var(--aegis-text-muted)]">
                    {refillTime >= 60 ? `${(refillTime / 60).toFixed(1)}m` : `${refillTime.toFixed(1)}s`}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
