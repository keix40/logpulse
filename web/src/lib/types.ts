export type LogEntry = {
  timestamp: string;
  level: string;
  service: string;
  message: string;
  attributes?: Record<string, string>;
};

export type AlertRule = {
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
