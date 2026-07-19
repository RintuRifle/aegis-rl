const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081";

export interface StatsResponse {
  allowed: number;
  denied: number;
  degraded: number;
  total: number;
  uptime_seconds: number;
  timestamp: number;
}

export interface TierConfig {
  name: string;
  capacity: number;
  refill_rate: number;
}

export interface ConfigResponse {
  tiers: Record<string, TierConfig>;
}

export interface TimeSeriesPoint {
  time: string;
  allowed: number;
  denied: number;
  total: number;
}

export async function fetchStats(): Promise<StatsResponse> {
  const res = await fetch(`${API_BASE}/api/stats`, {
    cache: "no-store",
  });
  if (!res.ok) throw new Error(`Stats fetch failed: ${res.status}`);
  return res.json();
}

export async function fetchConfig(): Promise<ConfigResponse> {
  const res = await fetch(`${API_BASE}/api/config`, {
    cache: "no-store",
  });
  if (!res.ok) throw new Error(`Config fetch failed: ${res.status}`);
  return res.json();
}

export async function sendTestRequest(
  apiKey: string = "demo-key"
): Promise<{ status: number; headers: Record<string, string>; body: string }> {
  const res = await fetch(`${API_BASE}/api/test`, {
    headers: { "X-API-Key": apiKey },
    cache: "no-store",
  });

  const headers: Record<string, string> = {};
  res.headers.forEach((value, key) => {
    if (key.startsWith("x-ratelimit") || key === "retry-after") {
      headers[key] = value;
    }
  });

  const body = await res.text();
  return { status: res.status, headers, body };
}

export function formatUptime(seconds: number): string {
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = Math.floor(seconds % 60);

  if (d > 0) return `${d}d ${h}h ${m}m`;
  if (h > 0) return `${h}h ${m}m ${s}s`;
  if (m > 0) return `${m}m ${s}s`;
  return `${s}s`;
}

export function formatNumber(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return n.toString();
}
