import { useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { toast } from "sonner";
import { Check, Copy, Eraser } from "lucide-react";
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { useCommands, useExecute, useHistory } from "@/hooks/use-console";
import { ApiError, type CommandInfo, type CommandTier, type HistoryEntry } from "@/lib/api";
import { cn } from "@/lib/utils";

/** Entries rendered in the scrollback. */
interface Entry {
  id: number;
  command: string;
  at: string;
  /** Absent while the command is still running. */
  output?: string;
  failed?: boolean;
  /** Milliseconds the round trip took, absent for replayed history. */
  ms?: number;
}

/**
 * Scrollback ids.
 *
 * Not crypto.randomUUID: that is restricted to secure contexts, and this panel
 * is meant to be opened at http://192.168.x.x, where it is undefined and throws
 * on every command. http://localhost is a secure context by exception, which is
 * exactly why the bug survived development.
 */
let nextEntryId = 0;

const TIER_LABEL: Record<CommandTier, string> = {
  normal: "",
  mutating: "changes the world",
  destructive: "destructive",
};

/** How many suggestions the strip offers before it stops being a shortlist. */
const MAX_SUGGESTIONS = 6;

export function ConsolePage() {
  const { data: catalogue } = useCommands();
  const { data: historyData } = useHistory();
  const execute = useExecute();

  const [input, setInput] = useState("");
  const [entries, setEntries] = useState<Entry[]>([]);
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [pending, setPending] = useState<string | null>(null);
  const [confirmText, setConfirmText] = useState("");
  const [copied, setCopied] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  // Position within the recalled history, -1 meaning "the live input".
  const [historyIndex, setHistoryIndex] = useState(-1);
  const history = useMemo(() => historyData?.history ?? [], [historyData]);
  const commands = useMemo(() => catalogue?.commands ?? [], [catalogue]);

  // Which suggestion Tab will accept.
  const [highlight, setHighlight] = useState(0);

  /**
   * Replay what the panel already remembers.
   *
   * The server keeps a history of every command run through it, and the page
   * was throwing that away on load: a reload left an empty console even though
   * the record was one fetch away. Seeded once, so running something new does
   * not duplicate the list.
   */
  const seeded = useRef(false);
  useEffect(() => {
    if (seeded.current || history.length === 0) return;
    seeded.current = true;
    setEntries(
      [...history]
        .sort((a, b) => Date.parse(a.ranAt) - Date.parse(b.ranAt))
        .map((h: HistoryEntry) => ({
          id: nextEntryId++,
          command: h.command,
          at: h.ranAt,
          output: h.succeeded ? h.result?.trimEnd() || "(no output)" : (h.error ?? "failed"),
          failed: !h.succeeded,
        })),
    );
  }, [history]);

  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight });
  }, [entries]);

  // Ctrl/Cmd-K is the near-universal palette shortcut.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "k" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        setPaletteOpen((open) => !open);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const typed = input.trimStart();
  const firstToken = typed.split(/\s+/)[0] ?? "";
  const hasArguments = /\s/.test(typed);

  /** The command the input names exactly, if any. */
  const named = useMemo(
    () => commands.find((c) => matchesName(c, firstToken)),
    [commands, firstToken],
  );

  /**
   * Commands the first token could still become.
   *
   * The whole argument for this panel over the in-game console is that nobody
   * should have to remember what a command is called. Making that true here
   * means answering while they type, not only when they think to open a modal.
   */
  const suggestions = useMemo(() => {
    if (hasArguments || firstToken === "") return [];
    const needle = firstToken.toLowerCase();
    const matched = commands.filter(
      (c) =>
        c.name.toLowerCase().startsWith(needle) ||
        c.aliases.some((a) => a.toLowerCase().startsWith(needle)),
    );
    // An exact name on its own is not a suggestion, it is an answer.
    if (matched.length === 1 && matchesName(matched[0], firstToken)) return [];
    return matched.slice(0, MAX_SUGGESTIONS);
  }, [commands, firstToken, hasArguments]);

  useEffect(() => setHighlight(0), [firstToken]);

  function tierOf(commandLine: string): CommandTier {
    const name = commandLine.trim().split(/\s+/)[0] ?? "";
    return commands.find((c) => matchesName(c, name))?.tier ?? "normal";
  }

  async function run(commandLine: string) {
    const id = nextEntryId++;
    const startedAt = Date.now();
    setEntries((current) => [
      ...current,
      { id, command: commandLine, at: new Date().toISOString() },
    ]);
    setInput("");
    setHistoryIndex(-1);

    try {
      const result = await execute.mutateAsync(commandLine);
      finish(id, result.result.trimEnd() || "(no output)", false, Date.now() - startedAt);
    } catch (cause) {
      // Always the server's own words, never a generic failure.
      const message = cause instanceof ApiError ? cause.message : "The command failed.";
      finish(id, message, true, Date.now() - startedAt);
      toast.error("Command failed", { description: message });
    }
  }

  function finish(id: number, output: string, failed: boolean, ms: number) {
    setEntries((current) => current.map((e) => (e.id === id ? { ...e, output, failed, ms } : e)));
  }

  function submit(e: FormEvent) {
    e.preventDefault();
    const commandLine = input.trim();
    if (!commandLine) return;

    // Anything that changes the world asks first. Destructive commands ask
    // harder, in the dialog itself.
    if (tierOf(commandLine) === "normal") {
      void run(commandLine);
      return;
    }
    setPending(commandLine);
    setConfirmText("");
  }

  function accept(command: CommandInfo) {
    setInput(command.name + " ");
    inputRef.current?.focus();
  }

  function onKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    // Tab completes; shift-Tab walks back through the shortlist.
    if (e.key === "Tab" && suggestions.length > 0) {
      e.preventDefault();
      if (e.shiftKey) {
        setHighlight((h) => (h - 1 + suggestions.length) % suggestions.length);
      } else if (highlight === 0) {
        accept(suggestions[0]);
      } else {
        accept(suggestions[highlight]);
      }
      return;
    }
    if (e.key === "ArrowUp") {
      e.preventDefault();
      const next = Math.min(historyIndex + 1, history.length - 1);
      if (next >= 0) {
        setHistoryIndex(next);
        setInput(history[next].command);
      }
    } else if (e.key === "ArrowDown") {
      e.preventDefault();
      const next = historyIndex - 1;
      setHistoryIndex(next);
      setInput(next < 0 ? "" : history[next].command);
    }
  }

  async function copyAll() {
    const text = entries.map((e) => `> ${e.command}\n${e.output ?? ""}`).join("\n\n");
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // Denied outside a secure context, which a LAN panel on plain HTTP is.
      toast.error("Could not copy", { description: "The browser refused clipboard access." });
    }
  }

  const pendingTier = pending ? tierOf(pending) : "normal";
  const needsTyped = pendingTier === "destructive";
  const pendingName = pending?.trim().split(/\s+/)[0] ?? "";
  const confirmReady = !needsTyped || confirmText.trim() === pendingName;

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="region-head shrink-0 gap-3">
        <Button
          variant="ghost"
          size="sm"
          className="h-6 px-2 text-xs"
          onClick={() => setPaletteOpen(true)}
        >
          Browse commands
          <kbd className="ml-1.5 text-2xs text-bone-faint">⌘K</kbd>
        </Button>
        {catalogue && <span className="stencil">{commands.length} on this server</span>}

        <div className="ml-auto flex items-center gap-1">
          <Button
            variant="ghost"
            size="sm"
            className="h-6 gap-1.5 px-2 text-xs text-bone-faint hover:text-foreground"
            disabled={entries.length === 0}
            onClick={() => void copyAll()}
          >
            {copied ? <Check className="size-3 text-status-online" /> : <Copy className="size-3" />}
            Copy
          </Button>
          <Button
            variant="ghost"
            size="sm"
            className="h-6 gap-1.5 px-2 text-xs text-bone-faint hover:text-foreground"
            disabled={entries.length === 0}
            onClick={() => setEntries([])}
          >
            <Eraser className="size-3" />
            Clear
          </Button>
        </div>
      </div>

      <div ref={scrollRef} className="region min-h-0 flex-1 overflow-y-auto">
        {entries.length === 0 ? (
          <EmptyState history={history} onPick={(c) => setInput(c)} />
        ) : (
          <ul>
            {entries.map((entry) => (
              <Block key={entry.id} entry={entry} />
            ))}
          </ul>
        )}
      </div>

      {/* What the command is and what it takes, from the server's own help. */}
      {named && hasArguments && (
        <div className="shrink-0 border-t border-border px-4 py-1.5 md:px-6">
          <p className="truncate text-xs text-bone-dim">
            <span className="readout text-bone">{named.name}</span>
            {named.description && <span className="ml-2">{named.description}</span>}
          </p>
          {named.help && (
            <p className="readout mt-0.5 truncate text-xs text-bone-faint">
              {named.help.split("\n").find((l) => l.trim() !== "")}
            </p>
          )}
        </div>
      )}

      {suggestions.length > 0 && (
        <ul className="flex shrink-0 items-stretch gap-px overflow-x-auto border-t border-border">
          {suggestions.map((command, i) => (
            <li key={command.name} className="min-w-0">
              <button
                type="button"
                onMouseEnter={() => setHighlight(i)}
                onClick={() => accept(command)}
                className={cn(
                  "flex h-full min-w-0 flex-col items-start gap-0.5 border-r border-border px-3 py-1.5 text-left",
                  i === highlight ? "bg-accent" : "hover:bg-accent/50",
                )}
                title={command.description}
              >
                <span className="readout truncate text-xs text-bone">{command.name}</span>
                <span className="max-w-48 truncate text-2xs text-bone-faint">
                  {command.description || " "}
                </span>
              </button>
            </li>
          ))}
          <li className="ml-auto flex shrink-0 items-center px-3">
            <span className="stencil">
              Tab to complete{suggestions.length > 1 && " · shift-Tab to cycle"}
            </span>
          </li>
        </ul>
      )}

      <form onSubmit={submit} className="flex shrink-0 gap-2 border-t border-border p-3 md:px-6">
        <span aria-hidden className="readout self-center text-bone-faint">
          &gt;
        </span>
        <Input
          ref={inputRef}
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={onKeyDown}
          placeholder="Type a command, or press ⌘K to browse"
          autoComplete="off"
          spellCheck={false}
          className="h-8 font-mono text-sm"
        />
        <Button
          type="submit"
          size="sm"
          className="h-8"
          disabled={execute.isPending || !input.trim()}
        >
          Run
        </Button>
      </form>

      <CommandPalette
        open={paletteOpen}
        onOpenChange={setPaletteOpen}
        commands={commands}
        onPick={(name) => {
          setPaletteOpen(false);
          setInput(name + " ");
          inputRef.current?.focus();
        }}
      />

      <Dialog open={pending !== null} onOpenChange={(open) => !open && setPending(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{needsTyped ? "This is destructive" : "Run this command?"}</DialogTitle>
            <DialogDescription asChild>
              <div className="space-y-3">
                <code className="block bg-muted px-2 py-1 font-mono text-sm text-foreground">
                  {pending}
                </code>
                {needsTyped && (
                  <p>
                    This can disconnect players or discard world data. Type{" "}
                    <code className="font-mono text-foreground">{pendingName}</code> to confirm.
                  </p>
                )}
              </div>
            </DialogDescription>
          </DialogHeader>

          {needsTyped && (
            <Input
              value={confirmText}
              onChange={(e) => setConfirmText(e.target.value)}
              placeholder={pendingName}
              autoComplete="off"
              className="font-mono"
            />
          )}

          <DialogFooter>
            <Button variant="outline" onClick={() => setPending(null)}>
              Cancel
            </Button>
            <Button
              variant={needsTyped ? "destructive" : "default"}
              disabled={!confirmReady}
              onClick={() => {
                const commandLine = pending!;
                setPending(null);
                void run(commandLine);
              }}
            >
              Run command
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

/** Does this token name that command, by its name or one of its aliases? */
function matchesName(command: CommandInfo, token: string): boolean {
  const t = token.toLowerCase();
  return command.name.toLowerCase() === t || command.aliases.some((a) => a.toLowerCase() === t);
}

/**
 * One command and its output, as a block rather than two loose lines.
 *
 * The old scrollback was a flat run of <pre> tags, so where a command ended and
 * its output began was left to the reader. The rule down the left binds them,
 * and turns crimson when the server refused.
 */
function Block({ entry }: { entry: Entry }) {
  const running = entry.output === undefined;
  const time = new Date(entry.at);
  const clock = Number.isNaN(time.getTime())
    ? "--:--:--"
    : time.toLocaleTimeString([], {
        hour: "2-digit",
        minute: "2-digit",
        second: "2-digit",
        hour12: false,
      });

  return (
    <li className="border-b border-border px-4 py-2 last:border-b-0 md:px-6">
      <div className="flex items-baseline gap-3">
        <span
          aria-hidden
          className={cn(
            "-ml-4 h-4 w-0.5 shrink-0 md:-ml-6",
            entry.failed ? "bg-crimson-lit" : running ? "animate-breathe bg-bone/40" : "bg-bone/25",
          )}
        />
        <span className="readout shrink-0 text-xs text-bone-faint">{clock}</span>
        <code className="readout min-w-0 flex-1 text-sm break-words text-bone">
          {entry.command}
        </code>
        {entry.ms !== undefined && (
          <span className="readout shrink-0 text-xs text-bone-faint">{entry.ms} ms</span>
        )}
      </div>

      {running ? (
        <p className="readout mt-1 pl-[1.6rem] text-xs text-bone-faint">running…</p>
      ) : (
        <pre
          className={cn(
            "mt-1 pl-[1.6rem] font-mono text-sm leading-snug whitespace-pre-wrap",
            entry.failed ? "text-crimson-lit" : "text-bone-dim",
          )}
        >
          {entry.output}
        </pre>
      )}
    </li>
  );
}

/**
 * The empty console.
 *
 * A blank box and one grey sentence was a waste of the largest area on the
 * page. What somebody wants first is usually something they have already run.
 */
function EmptyState({
  history,
  onPick,
}: {
  history: HistoryEntry[];
  onPick: (command: string) => void;
}) {
  const recent = useMemo(() => {
    const seen = new Set<string>();
    return history.filter((h) => (seen.has(h.command) ? false : seen.add(h.command))).slice(0, 8);
  }, [history]);

  return (
    <div className="p-6 md:p-8">
      <p className="text-sm text-bone-dim">
        Type a command and press Enter. <kbd className="readout text-bone">Tab</kbd> completes what
        you have started, <kbd className="readout text-bone">↑</kbd> walks back through what you
        have run, and <kbd className="readout text-bone">⌘K</kbd> browses all of them.
      </p>

      {recent.length > 0 && (
        <div className="mt-6">
          <h2 className="stencil">Run again</h2>
          <ul className="mt-2 flex flex-wrap gap-2">
            {recent.map((h) => (
              <li key={h.id}>
                <button
                  type="button"
                  onClick={() => onPick(h.command)}
                  className={cn(
                    "readout border border-border px-2.5 py-1 text-xs transition-colors hover:bg-accent",
                    h.succeeded ? "text-bone-dim" : "text-crimson-lit",
                  )}
                  title={h.succeeded ? "Ran cleanly" : (h.error ?? "Failed")}
                >
                  {h.command}
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

function CommandPalette({
  open,
  onOpenChange,
  commands,
  onPick,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  commands: CommandInfo[];
  onPick: (name: string) => void;
}) {
  const [selected, setSelected] = useState<CommandInfo | null>(null);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-3xl p-0">
        <DialogHeader className="sr-only">
          <DialogTitle>Browse commands</DialogTitle>
          <DialogDescription>Search the commands this server accepts.</DialogDescription>
        </DialogHeader>
        <div className="grid md:grid-cols-2">
          <Command className="border-r border-border">
            <CommandInput placeholder="Search commands…" />
            <CommandList className="max-h-96">
              <CommandEmpty>No command matches that.</CommandEmpty>
              <CommandGroup>
                {commands.map((command) => (
                  <CommandItem
                    key={command.name}
                    value={`${command.name} ${command.aliases.join(" ")} ${command.description}`}
                    onSelect={() => onPick(command.name)}
                    onMouseEnter={() => setSelected(command)}
                    onFocus={() => setSelected(command)}
                    className="flex items-center gap-2"
                  >
                    <span className="font-mono">{command.name}</span>
                    {command.tier !== "normal" && (
                      <Badge variant={command.tier === "destructive" ? "destructive" : "secondary"}>
                        {TIER_LABEL[command.tier]}
                      </Badge>
                    )}
                    {command.blocked && <Badge variant="outline">blocked</Badge>}
                  </CommandItem>
                ))}
              </CommandGroup>
            </CommandList>
          </Command>

          {/* The help text is the server's own, not something the panel wrote. */}
          <div className="max-h-96 overflow-y-auto p-4">
            {selected ? (
              <div className="space-y-2">
                <h3 className="font-mono font-semibold">{selected.name}</h3>
                <p className="text-sm text-muted-foreground">{selected.description}</p>
                {selected.aliases.length > 1 && (
                  <p className="text-sm text-muted-foreground">
                    Also: {selected.aliases.filter((a) => a !== selected.name).join(", ")}
                  </p>
                )}
                {selected.help && (
                  <pre className="bg-muted p-2 font-mono text-xs whitespace-pre-wrap">
                    {selected.help}
                  </pre>
                )}
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">
                Select a command to see the server's usage notes.
              </p>
            )}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
