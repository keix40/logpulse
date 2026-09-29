import { levelColor } from "@/lib/logLevels";
import type { LogEntry } from "@/lib/types";

export function LogLine({ entry }: { entry: LogEntry }) {
  const ts = new Date(entry.timestamp).toLocaleTimeString();
  return (
    <div className="font-mono text-sm border-b border-slate-800/60 py-2 px-3 hover:bg-slate-900/40">
      <span className="text-muted mr-2">{ts}</span>
      <span className={`uppercase text-xs font-semibold mr-2 ${levelColor(entry.level)}`}>
        {entry.level}
      </span>
      <span className="text-accent mr-2">[{entry.service}]</span>
      <span>{entry.message}</span>
    </div>
  );
}
