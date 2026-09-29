const workerBase = () =>
  process.env.WORKER_URL ??
  process.env.NEXT_PUBLIC_WORKER_URL ??
  "http://localhost:8081";

export function workerBackendUrl(): string {
  return workerBase().replace(/\/$/, "");
}

export function workerAuthHeaders(): HeadersInit {
  const key = process.env.READ_API_KEY;
  if (!key) {
    return {};
  }
  return { Authorization: `Bearer ${key}` };
}
