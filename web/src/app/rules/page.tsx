import fs from "fs";
import path from "path";
import yaml from "yaml";

type RuleRow = {
  id: string;
  name: string;
  description?: string;
  enabled: boolean;
  service?: string;
  level?: string;
  window?: string;
  threshold?: number;
  pattern?: string;
  cooldown?: string;
  channels?: string[];
};

function loadRules(): RuleRow[] {
  const candidates = [
    process.env.ALERT_RULES_PATH,
    path.join(process.cwd(), "..", "deploy", "alerts.yaml"),
    path.join(process.cwd(), "public", "alerts.yaml"),
  ].filter(Boolean) as string[];

  for (const p of candidates) {
    try {
      if (fs.existsSync(p)) {
        const doc = yaml.parse(fs.readFileSync(p, "utf8")) as {
          rules?: RuleRow[];
        };
        return doc.rules ?? [];
      }
    } catch {
      /* try next */
    }
  }
  return [];
}

export default function RulesPage() {
  const rules = loadRules();

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-semibold">Alert rules</h1>
      <p className="text-sm text-muted">
        Rules are loaded from{" "}
        <code className="text-accent">deploy/alerts.yaml</code> (mounted into
        the alerter at runtime). Edit the file and restart the alerter to apply
        changes.
      </p>
      <div className="overflow-x-auto rounded-lg border border-slate-800">
        <table className="w-full text-sm">
          <thead className="bg-panel text-left text-muted">
            <tr>
              <th className="p-3">ID</th>
              <th className="p-3">Name</th>
              <th className="p-3">Match</th>
              <th className="p-3">Window</th>
              <th className="p-3">Cooldown</th>
              <th className="p-3">Channels</th>
            </tr>
          </thead>
          <tbody>
            {rules.map((r) => (
              <tr key={r.id} className="border-t border-slate-800">
                <td className="p-3 font-mono text-xs">{r.id}</td>
                <td className="p-3">
                  <div className="font-medium">{r.name}</div>
                  {r.description && (
                    <div className="text-muted text-xs">{r.description}</div>
                  )}
                </td>
                <td className="p-3 text-xs font-mono">
                  {r.pattern && <div>pattern: {r.pattern}</div>}
                  {r.level && <div>level: {r.level}</div>}
                  {r.service && <div>service: {r.service}</div>}
                  {r.threshold != null && <div>threshold: {r.threshold}</div>}
                </td>
                <td className="p-3">{r.window ?? "—"}</td>
                <td className="p-3">{r.cooldown ?? "—"}</td>
                <td className="p-3">{(r.channels ?? []).join(", ")}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {rules.length === 0 && (
          <p className="p-4 text-muted">No rules file found in this environment.</p>
        )}
      </div>
    </div>
  );
}
