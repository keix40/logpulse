"use client";

import { FormEvent, useState } from "react";
import { LogLine } from "@/components/LogLine";
import { LEVELS } from "@/lib/logLevels";
import type { LogEntry } from "@/lib/types";

const searchPath = "/api/worker/search";

export default function SearchPage() {
  const [q, setQ] = useState("");
  const [level, setLevel] = useState("");
  const [service, setService] = useState("");
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    const params = new URLSearchParams();
    if (q) params.set("q", q);
    if (level) params.set("level", level);
    if (service) params.set("service", service);
    params.set("limit", "200");
    try {
      const res = await fetch(`${searchPath}?${params}`);
      if (!res.ok) throw new Error(await res.text());
      const data = (await res.json()) as { logs: LogEntry[] };
      setLogs(
        data.logs.map((l) => ({
          ...l,
          timestamp:
            typeof l.timestamp === "string"
              ? l.timestamp
              : new Date(l.timestamp).toISOString(),
        })),
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : "Search failed");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-semibold">Search stored logs</h1>
      <form
        onSubmit={onSubmit}
        className="flex flex-wrap gap-3 items-end rounded-lg border border-slate-800 bg-panel p-4"
      >
        <label className="text-sm flex flex-col gap-1">
          Query
          <input
            className="rounded bg-surface border border-slate-700 px-3 py-2 min-w-[200px]"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="message substring"
          />
        </label>
        <label className="text-sm flex flex-col gap-1">
          Level
          <select
            className="rounded bg-surface border border-slate-700 px-3 py-2"
            value={level}
            onChange={(e) => setLevel(e.target.value)}
          >
            <option value="">Any</option>
            {LEVELS.map((l) => (
              <option key={l} value={l}>
                {l}
              </option>
            ))}
          </select>
        </label>
        <label className="text-sm flex flex-col gap-1">
          Service
          <input
            className="rounded bg-surface border border-slate-700 px-3 py-2"
            value={service}
            onChange={(e) => setService(e.target.value)}
          />
        </label>
        <button
          type="submit"
          disabled={loading}
          className="rounded bg-accent px-4 py-2 text-sm font-medium text-white hover:bg-blue-500 disabled:opacity-50"
        >
          {loading ? "Searching…" : "Search"}
        </button>
      </form>
      {error && <p className="text-red-400 text-sm">{error}</p>}
      <div className="rounded-lg border border-slate-800 bg-panel overflow-hidden">
        {logs.map((entry, i) => (
          <LogLine key={`${entry.timestamp}-${i}`} entry={entry} />
        ))}
        {!loading && logs.length === 0 && (
          <p className="p-4 text-muted text-sm">No results yet.</p>
        )}
      </div>
    </div>
  );
}
