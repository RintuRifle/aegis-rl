"use client";

import { useState } from "react";
import { sendTestRequest } from "@/lib/api";

interface TestResult {
  status: number;
  headers: Record<string, string>;
  body: string;
  timestamp: string;
}

export function RequestTester() {
  const [apiKey, setApiKey] = useState("demo-key-001");
  const [results, setResults] = useState<TestResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [burstCount, setBurstCount] = useState(10);

  const sendSingle = async () => {
    setLoading(true);
    try {
      const res = await sendTestRequest(apiKey);
      setResults((prev) =>
        [
          {
            ...res,
            timestamp: new Date().toLocaleTimeString("en-US", {
              hour12: false,
            }),
          },
          ...prev,
        ].slice(0, 20)
      );
    } catch (err) {
      setResults((prev) =>
        [
          {
            status: 0,
            headers: {},
            body: err instanceof Error ? err.message : "Network error",
            timestamp: new Date().toLocaleTimeString("en-US", {
              hour12: false,
            }),
          },
          ...prev,
        ].slice(0, 20)
      );
    }
    setLoading(false);
  };

  const sendBurst = async () => {
    setLoading(true);
    const promises = Array.from({ length: burstCount }, () =>
      sendTestRequest(apiKey).catch((err) => ({
        status: 0,
        headers: {},
        body: err instanceof Error ? err.message : "Network error",
      }))
    );

    const responses = await Promise.all(promises);
    const now = new Date().toLocaleTimeString("en-US", { hour12: false });

    const newResults = responses.map((res, i) => ({
      ...res,
      timestamp: `${now} [${i + 1}/${burstCount}]`,
    }));

    setResults((prev) => [...newResults.reverse(), ...prev].slice(0, 50));
    setLoading(false);
  };

  return (
    <div className="glass-card p-5 flex flex-col h-full">
      <h2 className="text-sm font-semibold mb-4">Request Tester</h2>

      {/* API Key Input */}
      <div className="mb-3">
        <label className="text-xs text-[var(--aegis-text-muted)] block mb-1.5">
          API Key
        </label>
        <input
          type="text"
          value={apiKey}
          onChange={(e) => setApiKey(e.target.value)}
          className="w-full bg-[var(--aegis-surface)] border border-[var(--aegis-border)] rounded-lg px-3 py-2 text-sm text-[var(--aegis-text)] focus:outline-none focus:border-indigo-500/50 focus:ring-1 focus:ring-indigo-500/20 transition-all"
          placeholder="Enter API key..."
        />
      </div>

      {/* Burst Size */}
      <div className="mb-3">
        <label className="text-xs text-[var(--aegis-text-muted)] block mb-1.5">
          Burst Size
        </label>
        <input
          type="number"
          value={burstCount}
          onChange={(e) =>
            setBurstCount(Math.max(1, Math.min(100, parseInt(e.target.value) || 1)))
          }
          className="w-full bg-[var(--aegis-surface)] border border-[var(--aegis-border)] rounded-lg px-3 py-2 text-sm text-[var(--aegis-text)] focus:outline-none focus:border-indigo-500/50 focus:ring-1 focus:ring-indigo-500/20 transition-all"
          min={1}
          max={100}
        />
      </div>

      {/* Buttons */}
      <div className="flex gap-2 mb-4">
        <button
          onClick={sendSingle}
          disabled={loading}
          className="flex-1 px-3 py-2 bg-indigo-600 hover:bg-indigo-500 disabled:bg-indigo-600/50 rounded-lg text-xs font-medium transition-all active:scale-95"
        >
          {loading ? "Sending..." : "Send Request"}
        </button>
        <button
          onClick={sendBurst}
          disabled={loading}
          className="flex-1 px-3 py-2 bg-amber-600 hover:bg-amber-500 disabled:bg-amber-600/50 rounded-lg text-xs font-medium transition-all active:scale-95"
        >
          {loading ? "Bursting..." : `Burst ×${burstCount}`}
        </button>
      </div>

      {/* Results */}
      <div className="flex-1 overflow-y-auto space-y-1.5 max-h-[240px]">
        {results.length === 0 ? (
          <div className="text-center text-[var(--aegis-text-muted)] text-xs py-8">
            <div className="text-2xl mb-2">🧪</div>
            <p>Send a request to test the rate limiter</p>
          </div>
        ) : (
          results.map((r, i) => (
            <div
              key={i}
              className={`flex items-center justify-between px-3 py-2 rounded-lg text-xs ${
                r.status === 200
                  ? "bg-emerald-500/10 border border-emerald-500/20"
                  : r.status === 429
                    ? "bg-red-500/10 border border-red-500/20"
                    : "bg-gray-500/10 border border-gray-500/20"
              }`}
            >
              <div className="flex items-center gap-2">
                <span
                  className={`font-mono font-bold ${
                    r.status === 200
                      ? "text-emerald-400"
                      : r.status === 429
                        ? "text-red-400"
                        : "text-gray-400"
                  }`}
                >
                  {r.status || "ERR"}
                </span>
                <span className="text-[var(--aegis-text-muted)]">
                  {r.timestamp}
                </span>
              </div>
              {r.headers["x-ratelimit-remaining"] && (
                <span className="text-[var(--aegis-text-muted)]">
                  {r.headers["x-ratelimit-remaining"]} left
                </span>
              )}
            </div>
          ))
        )}
      </div>
    </div>
  );
}
