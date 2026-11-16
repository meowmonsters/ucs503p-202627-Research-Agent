import type { AgentMessage } from "../types";

// Base URL of the Go backend. Defaults to the backend's local dev port;
// override with VITE_API_URL (see .env.example). It is baked in at build
// time, so changing it on a deployed site means rebuilding the site.
export const API_BASE_URL: string =
  (import.meta.env.VITE_API_URL as string | undefined)?.trim().replace(/\/+$/, "") ||
  "http://localhost:8080";

// A run is capped server-side at about 90s of research plus the final write-up.
// Give up a little after the server's own hard limit instead of waiting forever.
const REQUEST_TIMEOUT_MS = 5 * 60 * 1000;

/**
 * Pings the backend so a sleeping host (e.g. Render's free plan) starts
 * waking up while the user is still typing. Fire-and-forget: it never
 * throws and nothing depends on the answer.
 */
export function warmUpBackend(): void {
  const controller = new AbortController();
  setTimeout(() => controller.abort(), 90_000);
  fetch(`${API_BASE_URL}/healthz`, { signal: controller.signal }).catch(() => {});
}

/** Turns an HTTP failure into something a user can act on. */
function messageForStatus(status: number, serverMessage?: string): string {
  if (serverMessage) return serverMessage;
  if (status === 429) return "Too many requests. Please wait a moment and try again.";
  if (status === 502 || status === 503 || status === 504) {
    // Hosting platforms answer with their own HTML page while the service
    // is starting, so there is no JSON message to show.
    return "The backend isn't responding yet — it may be waking up. Wait about a minute and try again.";
  }
  return `The backend returned an unexpected error (status ${status}).`;
}

/**
 * Runs the research agent for a company and resolves with every message it
 * produced (steps, then the report).
 *
 * The backend answers only once the whole run has finished, which can take
 * a while — callers should show a progress state while waiting.
 */
export async function runAgent(company: string): Promise<AgentMessage[]> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), REQUEST_TIMEOUT_MS);

  try {
    let response: Response;
    try {
      response = await fetch(`${API_BASE_URL}/api/agent`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ company }),
        signal: controller.signal,
      });
    } catch {
      if (controller.signal.aborted) {
        throw new Error("The research is taking too long. Please try again.");
      }
      // fetch only throws on network-level failures (server down, CORS, DNS).
      throw new Error(
        `Could not reach the backend at ${API_BASE_URL}. If this is a deployed site, ` +
          `the backend may be asleep or its ALLOWED_ORIGIN may not match this site.`,
      );
    }

    if (!response.ok) {
      let serverMessage: string | undefined;
      try {
        const body = (await response.json()) as { error?: unknown };
        if (typeof body.error === "string" && body.error) serverMessage = body.error;
      } catch {
        // Body wasn't JSON — fall back to a message based on the status.
      }
      throw new Error(messageForStatus(response.status, serverMessage));
    }

    let body: { messages?: AgentMessage[] };
    try {
      body = (await response.json()) as { messages?: AgentMessage[] };
    } catch {
      throw new Error("The backend sent a response that couldn't be read. Please try again.");
    }
    if (!Array.isArray(body.messages) || body.messages.length === 0) {
      throw new Error("The backend returned an empty response.");
    }
    return body.messages;
  } finally {
    clearTimeout(timer);
  }
}
