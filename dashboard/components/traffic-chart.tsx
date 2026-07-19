"use client";

import { TimeSeriesPoint } from "@/lib/api";
import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend,
} from "recharts";

interface TrafficChartProps {
  data: TimeSeriesPoint[];
}

export function TrafficChart({ data }: TrafficChartProps) {
  return (
    <div className="glass-card p-5">
      <div className="flex items-center justify-between mb-4">
        <div>
          <h2 className="text-sm font-semibold text-[var(--aegis-text)]">
            Live Traffic
          </h2>
          <p className="text-xs text-[var(--aegis-text-muted)]">
            Requests per poll interval (1.5s)
          </p>
        </div>
        <div className="flex items-center gap-3 text-xs">
          <div className="flex items-center gap-1.5">
            <div className="w-2.5 h-2.5 rounded-full bg-emerald-400" />
            <span className="text-[var(--aegis-text-muted)]">Allowed</span>
          </div>
          <div className="flex items-center gap-1.5">
            <div className="w-2.5 h-2.5 rounded-full bg-red-400" />
            <span className="text-[var(--aegis-text-muted)]">Denied</span>
          </div>
        </div>
      </div>

      <div className="h-[280px]">
        {data.length > 1 ? (
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart
              data={data}
              margin={{ top: 5, right: 10, left: -20, bottom: 0 }}
            >
              <defs>
                <linearGradient id="allowedGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="#10b981" stopOpacity={0.3} />
                  <stop offset="100%" stopColor="#10b981" stopOpacity={0} />
                </linearGradient>
                <linearGradient id="deniedGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="#ef4444" stopOpacity={0.3} />
                  <stop offset="100%" stopColor="#ef4444" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid
                strokeDasharray="3 3"
                stroke="rgba(42,42,58,0.5)"
                vertical={false}
              />
              <XAxis
                dataKey="time"
                tick={{ fontSize: 10, fill: "#71717a" }}
                tickLine={false}
                axisLine={{ stroke: "#2a2a3a" }}
                interval="preserveStartEnd"
              />
              <YAxis
                tick={{ fontSize: 10, fill: "#71717a" }}
                tickLine={false}
                axisLine={false}
                allowDecimals={false}
              />
              <Tooltip
                contentStyle={{
                  backgroundColor: "rgba(17,17,25,0.95)",
                  border: "1px solid rgba(99,102,241,0.3)",
                  borderRadius: "12px",
                  backdropFilter: "blur(10px)",
                  fontSize: "12px",
                  color: "#e4e4e7",
                  boxShadow: "0 4px 20px rgba(0,0,0,0.3)",
                }}
                labelStyle={{ color: "#71717a", marginBottom: "4px" }}
              />
              <Area
                type="monotone"
                dataKey="allowed"
                stroke="#10b981"
                strokeWidth={2}
                fill="url(#allowedGrad)"
                dot={false}
                animationDuration={300}
              />
              <Area
                type="monotone"
                dataKey="denied"
                stroke="#ef4444"
                strokeWidth={2}
                fill="url(#deniedGrad)"
                dot={false}
                animationDuration={300}
              />
            </AreaChart>
          </ResponsiveContainer>
        ) : (
          <div className="h-full flex items-center justify-center text-[var(--aegis-text-muted)] text-sm">
            <div className="text-center">
              <div className="text-3xl mb-2">📡</div>
              <p>Waiting for traffic data...</p>
              <p className="text-xs mt-1">
                Send requests to see the chart populate
              </p>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
