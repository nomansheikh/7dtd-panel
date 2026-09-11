import { useEffect, useMemo, useRef, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useEvents } from "@/hooks/use-events";
import type { EventKind, PanelEvent } from "@/lib/api";

type Filter = "all" | "chat" | "players" | "problems";

const FILTERS: { value: Filter; label: string }[] = [
  { value: "all", label: "Everything" },
  { value: "chat", label: "Chat" },
  { value: "players", label: "Joins, leaves and deaths" },
  { value: "problems", label: "Warnings and errors" },
];

function matches(event: PanelEvent, filter: Filter): boolean {
  switch (filter) {
    case "chat":
      return event.kind === "chat";
    case "players":
      return event.kind === "join" || event.kind === "leave" || event.kind === "death";
    case "problems":
      return (
        event.severity === "Error" ||
        event.severity === "Exception" ||
        event.severity === "Warning" ||
        event.kind === "status"
      );
    default:
      return true;
  }
}

const KIND_BADGE: Partial<Record<EventKind, string>> = {
  chat: "chat",
  join: "joined",
  leave: "left",
  death: "died",
  status: "panel",
};

export function EventsPage() {
  const { events, status } = useEvents();
  const [filter, setFilter] = useState<Filter>("all");
  const [search, setSearch] = useState("");

  const visible = useMemo(() => {
    const needle = search.trim().toLowerCase();
    const filtered = events.filter(
      (e) =>
        matches(e, filter) &&
        (needle === "" ||
          e.message.toLowerCase().includes(needle) ||
          (e.player ?? "").toLowerCase().includes(needle)),
    );
    // Sort by when things happened, not when they reached the panel. The
    // backlog replays lines written long before the panel connected, so
    // arrival order would show hours-old log lines beneath a status event
    // raised seconds ago. Sequence breaks ties, since the server's timestamps
    // have only second resolution.
    return filtered.sort((a, b) => {
      const at = Date.parse(a.at) - Date.parse(b.at);
      return at !== 0 ? at : a.seq - b.seq;
    });
  }, [events, filter, search]);

  // A log reads oldest to newest, so keep the newest in view unless the
  // operator has scrolled up to read something.
  const scrollRef = useRef<HTMLDivElement>(null);
  const pinnedToBottom = useRef(true);

  useEffect(() => {
    const el = scrollRef.current;
    if (!el || !pinnedToBottom.current) return;
    el.scrollTop = el.scrollHeight;
  }, [visible]);

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-3">
        <h1 className="text-xl font-semibold">Events</h1>
        <span className="text-sm text-muted-foreground">
          {status === "open" ? "live" : "reconnecting"}
          {" · "}
          {events.length} received
        </span>
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <Tabs value={filter} onValueChange={(v) => setFilter(v as Filter)}>
          <TabsList>
            {FILTERS.map((f) => (
              <TabsTrigger key={f.value} value={f.value}>
                {f.label}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
        <Input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Filter by text or player"
          className="max-w-xs"
        />
        {search && (
          <Button variant="ghost" size="sm" onClick={() => setSearch("")}>
            Clear
          </Button>
        )}
      </div>

      <div
        ref={scrollRef}
        onScroll={(e) => {
          const el = e.currentTarget;
          // Within a few pixels of the bottom counts as pinned, so a smooth
          // scroll landing just short does not unpin it.
          pinnedToBottom.current = el.scrollHeight - el.scrollTop - el.clientHeight < 24;
        }}
        className="h-[32rem] overflow-y-auto rounded-md border border-border bg-card"
      >
        {visible.length === 0 ? (
          <p className="p-4 text-sm text-muted-foreground">
            {events.length === 0
              ? "Waiting for the first event from the server."
              : "Nothing matches that filter."}
          </p>
        ) : (
          <ul className="divide-y divide-border">
            {visible.map((event) => (
              <li key={event.seq} className="flex gap-3 px-4 py-2 font-mono text-sm">
                <time
                  className="shrink-0 text-muted-foreground"
                  dateTime={event.at}
                  title={new Date(event.at).toLocaleString()}
                >
                  {new Date(event.at).toLocaleTimeString()}
                </time>
                {KIND_BADGE[event.kind] && (
                  <Badge variant="secondary" className="shrink-0">
                    {KIND_BADGE[event.kind]}
                  </Badge>
                )}
                {event.severity && event.severity !== "Log" && (
                  <Badge
                    variant={event.severity === "Warning" ? "secondary" : "destructive"}
                    className="shrink-0"
                  >
                    {event.severity}
                  </Badge>
                )}
                <span className="break-all whitespace-pre-wrap">
                  {event.player && <span className="mr-1 text-primary">{event.player}:</span>}
                  {event.message}
                </span>
              </li>
            ))}
          </ul>
        )}
      </div>

      <p className="text-xs text-muted-foreground">
        Chat and join messages are identified from the server's log text, which has no dedicated
        event type. Anything unrecognised is shown as a plain log line.
      </p>
    </div>
  );
}
