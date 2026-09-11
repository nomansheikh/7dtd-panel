import { useEffect, useRef, useState } from "react";
import type { PanelEvent } from "@/lib/api";

/** How many events the UI keeps in memory before dropping the oldest. */
const MAX_EVENTS = 2000;

export type FeedStatus = "connecting" | "open" | "reconnecting";

/**
 * Subscribes to the panel's event feed.
 *
 * EventSource reconnects on its own, and the panel replays a backlog on every
 * connection, so a reconnect refills the gap rather than leaving a hole. Events
 * are de-duplicated on the panel-assigned sequence number, since that backlog
 * necessarily overlaps what has already been seen.
 */
export function useEvents() {
  const [events, setEvents] = useState<PanelEvent[]>([]);
  const [status, setStatus] = useState<FeedStatus>("connecting");
  const seen = useRef<Set<number>>(new Set());

  useEffect(() => {
    const source = new EventSource("/api/events");

    source.addEventListener("open", () => setStatus("open"));

    source.addEventListener("panelEvent", (raw) => {
      let event: PanelEvent;
      try {
        event = JSON.parse((raw as MessageEvent<string>).data);
      } catch {
        // One malformed frame should not tear down the feed.
        return;
      }
      if (seen.current.has(event.seq)) return;
      seen.current.add(event.seq);

      setEvents((current) => {
        const next = [...current, event];
        if (next.length <= MAX_EVENTS) return next;
        const trimmed = next.slice(next.length - MAX_EVENTS);
        // Keep the de-duplication set from growing without bound alongside it.
        seen.current = new Set(trimmed.map((e) => e.seq));
        return trimmed;
      });
    });

    source.addEventListener("error", () => {
      // EventSource retries by itself; CLOSED means it gave up.
      setStatus(source.readyState === EventSource.CLOSED ? "reconnecting" : "reconnecting");
    });

    return () => source.close();
  }, []);

  return { events, status };
}
