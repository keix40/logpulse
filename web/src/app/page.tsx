"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { LogLine } from "@/components/LogLine";
import { LEVELS } from "@/lib/logLevels";
import type { LogEntry } from "@/lib/types";

const liveUrl = "/api/worker/live";

export default function LivePage() {
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [level, setLevel] = useState<string>("");
  const [service, setService] = useState<string>("");
  const [paused, setPaused] = useState(false);
  const [connected, setConnected] = useState(false);
  const pausedRef = useRef(paused);
  pausedRef.current = paused;

  useEffect(() => {
    const es = new EventSource(liveUrl);
    es.addEventListener("connected", () => setConnected(true));
    es.addEventListener("log", (ev) => {
      if (pausedRef.current) return;
      const entry = JSON.parse(ev.data) as LogEntry;
      setLogs((prev) => [entry, ...prev].slice(0, 500));
    });
    es.onerror = () => setConnected(false);
    return () => es.close();
  }, []);

  const filtered = useMemo(() => {
    return logs.filter((l) => {
      if (level && l.level !== level) return false;
      if (service && l.service !== service) return false;
      return true;
    });
  }, [logs, level, service]);

  const clear = useCallback(() => setLogs([]), []);

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-end gap-4">
        <div>
          <h1 className="text-2xl font-semibold">Live tail</h1>
          <p className="text-sm text-muted">
            {connected ? "Connected" : "Reconnecting…"} · {filtered.length}{" "}
            visible
          </p>
        </div>
        <div className="flex flex-wrap gap-3 ml-auto">
          <label className="text-sm">
            Level
            <select
              className="ml-2 rounded bg-panel border border-slate-700 px-2 py-1"
              value={level}
              onChange={(e) => setLevel(e.target.value)}
            >
              <option value="">All</option>
              {LEVELS.map((l) => (
                <option key={l} value={l}>
                  {l}
                </option>
              ))}
            </select>
          </label>
          <label className="text-sm">
            Service
            <input
              className="ml-2 rounded bg-panel border border-slate-700 px-2 py-1"
              placeholder="any"
              value={service}
              onChange={(e) => setService(e.target.value)}
            />
          </label>
          <button
            type="button"
            className="rounded bg-slate-700 px-3 py-1 text-sm hover:bg-slate-600"
            onClick={() => setPaused((p) => !p)}
          >
            {paused ? "Resume" : "Pause"}
          </button>
          <button
            type="button"
            className="rounded border border-slate-600 px-3 py-1 text-sm hover:bg-panel"
            onClick={clear}
          >
            Clear
          </button>
        </div>
      </div>
      <div className="rounded-lg border border-slate-800 bg-panel overflow-hidden max-h-[70vh] overflow-y-auto">
        {filtered.length === 0 ? (
          <p className="p-6 text-muted text-sm">
            Waiting for logs. Run{" "}
            <code className="text-accent">scripts/log-generator.sh</code> to
            emit sample events.
          </p>
        ) : (
          filtered.map((entry, i) => (
            <LogLine key={`${entry.timestamp}-${i}`} entry={entry} />
          ))
        )}
      </div>
    </div>
  );
}
