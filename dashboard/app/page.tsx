"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import {
  StatsResponse,
  TimeSeriesPoint,
  fetchStats,
  formatUptime,
  formatNumber,
} from "@/lib/api";
import { StatsCards } from "@/components/stats-cards";
import { TrafficChart } from "@/components/traffic-chart";
import { RequestTester } from "@/components/request-tester";
import { TierTable } from "@/components/tier-table";
import { SystemStatus } from "@/components/system-status";

const POLL_INTERVAL = 1500; // ms

export default function DashboardPage() {
  const [stats, setStats] = useState<StatsResponse | null>(null);
  const [history, setHistory] = useState<TimeSeriesPoint[]>([]);
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const prevStats = useRef<StatsResponse | null>(null);

  const poll = useCallback(async () => {
    try {
      const data = await fetchStats();
      setStats(data);
      setConnected(true);
      setError(null);

      // Calculate deltas for time series
      const now = new Date().toLocaleTimeString("en-US", {
        hour12: false,
        hour: "2-digit",
        minute: "2-digit",
        second: "2-digit",
      });

      const prev = prevStats.current;
      const allowedDelta = prev ? data.allowed - prev.allowed : 0;
      const deniedDelta = prev ? data.denied - prev.denied : 0;

      setHistory((h) => {
        const next = [
          ...h,
          {
            time: now,
            allowed: allowedDelta,
            denied: deniedDelta,
            total: allowedDelta + deniedDelta,
          },
        ];
        return next.slice(-40); // keep last 40 data points (60s of data)
      });

      prevStats.current = data;
    } catch (err) {
      setConnected(false);
      setError(
        err instanceof Error ? err.message : "Failed to connect to AegisRL"
      );
    }
  }, []);

  useEffect(() => {
    poll();
    const interval = setInterval(poll, POLL_INTERVAL);
    return () => clearInterval(interval);
  }, [poll]);

  return (
    <div className="gradient-bg min-h-screen">
      {/* Header */}
      <header className="border-b border-[var(--aegis-border)] px-6 py-4">
        <div className="max-w-7xl mx-auto flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="relative">
              <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-indigo-500 to-cyan-400 flex items-center justify-center font-bold text-lg">
                ⚡
              </div>
              <div
                className={`absolute -bottom-0.5 -right-0.5 w-3 h-3 rounded-full border-2 border-[var(--aegis-bg)] ${
                  connected ? "bg-emerald-400 status-pulse" : "bg-red-400"
                }`}
              />
            </div>
            <div>
              <h1 className="text-xl font-bold tracking-tight">
                Aegis
                <span className="bg-gradient-to-r from-indigo-400 to-cyan-400 bg-clip-text text-transparent">
                  RL
                </span>
              </h1>
              <p className="text-xs text-[var(--aegis-text-muted)]">
                Distributed Rate Limiter
              </p>
            </div>
          </div>

          <div className="flex items-center gap-4">
            {stats && (
              <span className="text-xs text-[var(--aegis-text-muted)]">
                Uptime: {formatUptime(stats.uptime_seconds)}
              </span>
            )}
            <div
              className={`flex items-center gap-2 px-3 py-1.5 rounded-full text-xs font-medium ${
                connected
                  ? "bg-emerald-500/10 text-emerald-400 border border-emerald-500/20"
                  : "bg-red-500/10 text-red-400 border border-red-500/20"
              }`}
            >
              <div
                className={`w-1.5 h-1.5 rounded-full ${
                  connected ? "bg-emerald-400" : "bg-red-400"
                }`}
              />
              {connected ? "Connected" : "Disconnected"}
            </div>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="max-w-7xl mx-auto px-6 py-6 space-y-6">
        {error && (
          <div className="glass-card p-4 border-red-500/30 bg-red-500/5">
            <div className="flex items-center gap-2 text-red-400 text-sm">
              <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                <path
                  fillRule="evenodd"
                  d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.28 7.22a.75.75 0 00-1.06 1.06L8.94 10l-1.72 1.72a.75.75 0 101.06 1.06L10 11.06l1.72 1.72a.75.75 0 101.06-1.06L11.06 10l1.72-1.72a.75.75 0 00-1.06-1.06L10 8.94 8.28 7.22z"
                  clipRule="evenodd"
                />
              </svg>
              <span>{error}</span>
              <span className="text-xs text-[var(--aegis-text-muted)] ml-2">
                Make sure the Go engine is running on{" "}
                {process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081"}
              </span>
            </div>
          </div>
        )}

        {/* Stats Cards */}
        <StatsCards stats={stats} />

        {/* Charts + Tester */}
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          <div className="lg:col-span-2">
            <TrafficChart data={history} />
          </div>
          <div>
            <RequestTester />
          </div>
        </div>

        {/* Bottom Row */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <TierTable />
          <SystemStatus stats={stats} connected={connected} />
        </div>
      </main>

      {/* Footer */}
      <footer className="border-t border-[var(--aegis-border)] px-6 py-4 mt-8">
        <div className="max-w-7xl mx-auto flex items-center justify-between text-xs text-[var(--aegis-text-muted)]">
          <span>
            AegisRL Edge Bouncer — Go + Redis + Lua Atomic Token Bucket
          </span>
          <span>
            Built by{" "}
            <a
              href="https://github.com/RintuRifle"
              target="_blank"
              rel="noopener noreferrer"
              className="text-indigo-400 hover:text-indigo-300 transition-colors"
            >
              Akshit Tiwari
            </a>
          </span>
        </div>
      </footer>
    </div>
  );
}
