import { workerAuthHeaders, workerBackendUrl } from "@/lib/workerBackend";

export async function GET(request: Request) {
  const url = new URL(request.url);
  const target = `${workerBackendUrl()}/v1/logs/search?${url.searchParams.toString()}`;
  const res = await fetch(target, {
    headers: {
      Accept: "application/json",
      ...workerAuthHeaders(),
    },
    cache: "no-store",
  });
  const body = await res.text();
  return new Response(body, {
    status: res.status,
    headers: { "Content-Type": res.headers.get("Content-Type") ?? "application/json" },
  });
}
