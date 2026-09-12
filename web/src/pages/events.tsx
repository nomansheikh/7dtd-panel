import { useEffect, useMemo, useRef, useState, type FormEvent, type ReactNode } from "react";
import { useMutation } from "@tanstack/react-query";
import { ArrowDown, SendHorizonal } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { StatusDot } from "@/components/connection-status";
import { useEvents } from "@/hooks/use-events";
import { usePlayers } from "@/hooks/use-players";
import { useServerId } from "@/hooks/use-servers";
import { api, ApiError, type EventKind, type PanelEvent } from "@/lib/api";
import { userColor } from "@/lib/user-color";
import { cn } from "@/lib/utils";

type Filter = "all" | "chat" | "players" | "problems";

const FILTERS: { value: Filter; label: string }[] = [
  { value: "all", label: "Everything" },
  { value: "chat", label: "Chat" },
  { value: "players", label: "People" },
  { value: "problems", label: "Problems" },
];

function matches(event: PanelEvent, filter: Filter): boolean {
  switch (filter) {
    case "chat":
      return event.kind === "chat";
    case "players":
      return event.kind === "join" || event.kind === "leave" || event.kind === "death";
    case "problems":
      return isProblem(event);
    default:
      return true;
  }
}

function isProblem(event: PanelEvent): boolean {
  return (
    event.severity === "Error" ||
    event.severity === "Exception" ||
    event.severity === "Warning" ||
    event.kind === "status"
  );
}

/**
 * What each row is, in four characters.
 *
 * Short and fixed-width so the column stays a column: a log you scan by shape
 * stops working the moment the labels are ragged.
 */
const LABEL: Partial<Record<EventKind, string>> = {
  chat: "say",
  join: "in",
  leave: "out",
  death: "died",
  status: "panel",
};

/**
 * How a row is coloured.
 *
 * Every line used to be the same grey, which made a death, a chat message and
 * an engine spawn notice indistinguishable in a wall of monospace. Severity
 * wins over kind, because an error about chat is an error first.
 */
function toneOf(event: PanelEvent): { bar: string; text: string; label: string } {
  if (event.severity === "Error" || event.severity === "Exception") {
    return { bar: "bg-crimson-lit", text: "text-crimson-lit", label: "text-crimson-lit" };
  }
  if (event.severity === "Warning") {
    return { bar: "bg-ember", text: "text-ember", label: "text-ember" };
  }
  switch (event.kind) {
    case "chat":
      return { bar: "bg-bone/40", text: "text-foreground", label: "text-bone-dim" };
    case "join":
      return { bar: "bg-status-online", text: "text-status-online", label: "text-status-online" };
    case "leave":
      return { bar: "bg-bone/20", text: "text-bone-dim", label: "text-bone-faint" };
    case "death":
      return { bar: "bg-crimson", text: "text-crimson-lit", label: "text-crimson-lit" };
    case "status":
      return { bar: "bg-ember", text: "text-ember", label: "text-ember" };
    default:
      // Plain engine output: present, but never competing with the rest.
      return { bar: "bg-transparent", text: "text-bone-faint", label: "text-bone-faint" };
  }
}

/** 24-hour, to match every other clock in the panel. */
function clockOf(at: string): string {
  const when = new Date(at);
  if (Number.isNaN(when.getTime())) return "--:--:--";
  return when.toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  });
}

export function EventsPage() {
  const { events, status } = useEvents();
  const [filter, setFilter] = useState<Filter>("all");
  const [search, setSearch] = useState("");

  // Text search is applied first so the filter counts describe what picking
  // that filter would actually show.
  const searched = useMemo(() => {
    const needle = search.trim().toLowerCase();
    if (needle === "") return events;
    return events.filter(
      (e) =>
        e.message.toLowerCase().includes(needle) || (e.player ?? "").toLowerCase().includes(needle),
    );
  }, [events, search]);

  const counts = useMemo(
    () => ({
      all: searched.length,
      chat: searched.filter((e) => e.kind === "chat").length,
      players: searched.filter((e) => ["join", "leave", "death"].includes(e.kind)).length,
      problems: searched.filter(isProblem).length,
    }),
    [searched],
  );

  const visible = useMemo(() => {
    // Sort by when things happened, not when they reached the panel. The
    // backlog replays lines written long before the panel connected, so
    // arrival order would show hours-old log lines beneath a status event
    // raised seconds ago. Sequence breaks ties, since the server's timestamps
    // have only second resolution.
    return searched
      .filter((e) => matches(e, filter))
      .sort((a, b) => {
        const at = Date.parse(a.at) - Date.parse(b.at);
        return at !== 0 ? at : a.seq - b.seq;
      });
  }, [searched, filter]);

  const scrollRef = useRef<HTMLDivElement>(null);
  const [pinned, setPinned] = useState(true);
  const [unseen, setUnseen] = useState(0);
  const seenCount = useRef(0);

  // A log reads oldest to newest, so keep the newest in view — unless the
  // operator has scrolled up to read something, in which case say how much
  // arrived while they were reading rather than yanking the page.
  useEffect(() => {
    const grew = Math.max(0, visible.length - seenCount.current);
    seenCount.current = visible.length;

    if (pinned) {
      const el = scrollRef.current;
      if (el) {
        el.scrollTop = el.scrollHeight;
        // Rows with long messages wrap, and their final height is not known
        // until after layout. Without this second pass the scroll lands short
        // and the page decides it has been detached from the tail.
        requestAnimationFrame(() => {
          if (scrollRef.current) scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
        });
      }
      setUnseen(0);
    } else if (grew > 0) {
      setUnseen((n) => n + grew);
    }
  }, [visible, pinned]);

  function jumpToLive() {
    const el = scrollRef.current;
    if (el) el.scrollTo({ top: el.scrollHeight, behavior: "smooth" });
    setPinned(true);
    setUnseen(0);
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex shrink-0 flex-wrap items-center gap-x-1 gap-y-2 border-b border-border px-4 py-2 md:px-6">
        {FILTERS.map((f) => (
          <button
            key={f.value}
            type="button"
            onClick={() => setFilter(f.value)}
            className={cn(
              "flex items-baseline gap-1.5 px-2.5 py-1 transition-colors",
              filter === f.value ? "bg-accent" : "hover:bg-accent/50",
            )}
          >
            <span className={cn("stencil", filter === f.value && "text-bone")}>{f.label}</span>
            <span className="readout text-xs text-bone-faint">{counts[f.value]}</span>
          </button>
        ))}

        <Input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Filter by text or player"
          className="ml-auto h-7 max-w-56 text-xs"
        />
        {search && (
          <Button
            variant="ghost"
            size="sm"
            className="h-7 px-2 text-xs text-bone-faint"
            onClick={() => setSearch("")}
          >
            Clear
          </Button>
        )}

        <span className="flex items-center gap-2 pl-2">
          <StatusDot status={status === "open" ? "online" : "degraded"} />
          <span className="stencil">{status === "open" ? "live" : "reconnecting"}</span>
        </span>
      </div>

      <div className="relative min-h-0 flex-1">
        <div
          ref={scrollRef}
          onScroll={(e) => {
            const el = e.currentTarget;
            // Within a few pixels of the bottom counts as pinned, so a smooth
            // scroll landing just short does not unpin it.
            const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 24;
            setPinned(atBottom);
            if (atBottom) setUnseen(0);
          }}
          className="region h-full overflow-y-auto"
        >
          {visible.length === 0 ? (
            <p className="p-6 text-center text-sm text-bone-faint">
              {events.length === 0
                ? "Waiting for the first event from the server."
                : "Nothing matches that filter."}
            </p>
          ) : (
            <ul>
              {visible.map((event) => (
                <Row
                  key={event.seq}
                  event={event}
                  needle={search.trim()}
                  onPickPlayer={setSearch}
                />
              ))}
            </ul>
          )}
        </div>

        {/* Detached from the tail. Say how much has arrived and offer the way
            back, rather than silently scrolling out from under a reader. */}
        {!pinned && (
          <Button
            size="sm"
            onClick={jumpToLive}
            className="animate-rise absolute right-4 bottom-4 h-7 gap-1.5 px-2.5 shadow-lg md:right-6"
          >
            <ArrowDown className="size-3" />
            <span className="stencil text-primary-foreground">
              {unseen > 0 ? `${unseen} new` : "Live"}
            </span>
          </Button>
        )}
      </div>

      <p className="shrink-0 border-t border-border px-4 py-1.5 text-xs text-bone-faint md:px-6">
        Chat and join messages are identified from the server's log text, which has no dedicated
        event type. Anything unrecognised is shown as a plain log line. Messages you send here
        appear in game as coming from the server, not from you.
      </p>

      <Composer />
    </div>
  );
}

function Row({
  event,
  needle,
  onPickPlayer,
}: {
  event: PanelEvent;
  needle: string;
  onPickPlayer: (name: string) => void;
}) {
  const tone = toneOf(event);
  const label = LABEL[event.kind] ?? (event.severity === "Warning" ? "warn" : "");

  return (
    <li
      className="flex items-baseline gap-3 border-b border-border px-4 py-1.5 last:border-b-0 md:px-6"
      // The server's original wording, for when the parse looks wrong.
      title={event.raw || undefined}
    >
      {/* A colour down the left edge, so the shape of a busy night is legible
          before a single word is read. */}
      <span aria-hidden className={cn("-ml-4 h-4 w-0.5 shrink-0 md:-ml-6", tone.bar)} />

      <time
        className="readout shrink-0 text-xs text-bone-faint"
        dateTime={event.at}
        title={new Date(event.at).toLocaleString()}
      >
        {clockOf(event.at)}
      </time>

      <span className={cn("stencil w-10 shrink-0 text-2xs", tone.label)}>{label}</span>

      {/*
        Who said it is a column, not a prefix.
        
        Running the name into the message meant every line started at a
        different place and the eye had to find the colon before it could find
        the words. Right-aligned against a rule, the messages line up and the
        names read as a list of who is talking.
      */}
      <span className="flex w-28 shrink-0 justify-end sm:w-36">
        {event.player ? (
          <button
            type="button"
            onClick={() => onPickPlayer(event.player!)}
            style={{ color: userColor(event.player) }}
            className="max-w-full truncate text-sm font-semibold underline-offset-2 hover:underline"
            title={`Show only ${event.player}`}
          >
            {event.player}
          </button>
        ) : null}
      </span>

      <p
        className={cn(
          "min-w-0 flex-1 border-l border-border pl-3 font-mono text-sm leading-snug break-words",
          tone.text,
        )}
      >
        {event.channel && <span className="stencil mr-1.5 text-ember">[{event.channel}]</span>}
        <Highlighted text={event.message} needle={needle} />
      </p>
    </li>
  );
}

/**
 * Says something to everyone on the server.
 *
 * The World page has had this as "Broadcast" all along, which is the wrong
 * place for it: you decide to say something because of what you just read in
 * the feed, and making somebody navigate away and back to answer a question is
 * how a chat stops being one.
 *
 * It can also address one player, which is the same reasoning a step further:
 * answering a question somebody asked in chat should not mean announcing the
 * answer to the whole server.
 */
function Composer() {
  const serverId = useServerId();
  const [message, setMessage] = useState("");
  // "" is everybody; anything else is one player's entity id.
  const [to, setTo] = useState("");

  const { data: playerData } = usePlayers();
  const online = (playerData?.players ?? []).filter((p) => p.online);
  const recipient = online.find((p) => String(p.entityId) === to);

  // Somebody who logs off mid-conversation must not leave the box quietly
  // addressed to them, or the next message goes nowhere.
  useEffect(() => {
    if (to !== "" && !recipient) setTo("");
  }, [to, recipient]);

  const send = useMutation({
    mutationFn: (text: string) =>
      recipient ? api.privateMessage(serverId, recipient.entityId, text) : api.say(serverId, text),
    onSuccess: () => setMessage(""),
    onError: (error) =>
      toast.error("Not sent", {
        description: error instanceof ApiError ? error.message : String(error),
      }),
  });

  function submit(event: FormEvent) {
    event.preventDefault();
    const text = message.trim();
    if (text !== "") send.mutate(text);
  }

  return (
    <form
      onSubmit={submit}
      className="flex shrink-0 items-center gap-2 border-t border-border px-4 py-2 md:px-6"
    >
      {/*
        Who it goes to sits inside the composer rather than in a menu
        somewhere, because it changes what the Send button does and that is
        not something to hide.
      */}
      <select
        value={to}
        onChange={(e) => setTo(e.target.value)}
        aria-label="Who to send to"
        className="readout h-8 shrink-0 border border-border bg-transparent px-2 text-xs text-bone-dim focus-visible:ring-1 focus-visible:ring-ring focus-visible:outline-none"
      >
        <option value="">Everyone</option>
        {online.map((p) => (
          <option key={p.entityId} value={String(p.entityId)}>
            {p.name}
          </option>
        ))}
      </select>
      <Input
        value={message}
        onChange={(e) => setMessage(e.target.value)}
        placeholder={
          recipient ? `Message ${recipient.name} privately` : "Message everyone on the server"
        }
        className="h-8 flex-1 text-sm"
        maxLength={200}
        disabled={send.isPending}
      />
      <Button
        type="submit"
        size="sm"
        className="h-8 gap-1.5"
        disabled={send.isPending || message.trim() === ""}
      >
        <SendHorizonal className="size-3.5" />
        Send
      </Button>
    </form>
  );
}

/**
 * Marks where the search matched.
 *
 * Filtering a log to eleven lines and then leaving the reader to find the word
 * themselves is half a feature.
 */
function Highlighted({ text, needle }: { text: string; needle: string }): ReactNode {
  if (needle === "") return text;

  const lower = text.toLowerCase();
  const target = needle.toLowerCase();
  const parts: ReactNode[] = [];
  let at = 0;

  for (;;) {
    const found = lower.indexOf(target, at);
    if (found === -1) break;
    if (found > at) parts.push(text.slice(at, found));
    parts.push(
      <mark key={found} className="bg-ember/25 text-inherit">
        {text.slice(found, found + needle.length)}
      </mark>,
    );
    at = found + needle.length;
  }

  if (parts.length === 0) return text;
  if (at < text.length) parts.push(text.slice(at));
  return parts;
}
