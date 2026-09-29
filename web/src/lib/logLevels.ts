export const LEVELS = ["debug", "info", "warn", "error", "fatal"] as const;

export function levelColor(level: string): string {
  switch (level.toLowerCase()) {
    case "debug":
      return "text-slate-400";
    case "info":
      return "text-sky-400";
    case "warn":
      return "text-amber-400";
    case "error":
      return "text-red-400";
    case "fatal":
      return "text-fuchsia-400";
    default:
      return "text-slate-300";
  }
}
