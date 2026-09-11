import { useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { toast } from "sonner";
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
import { ApiError, type CommandInfo, type CommandTier } from "@/lib/api";

/** Entries rendered in the scrollback. */
interface Line {
  id: string;
  kind: "input" | "output" | "error";
  text: string;
}

const TIER_LABEL: Record<CommandTier, string> = {
  normal: "",
  mutating: "changes the world",
  destructive: "destructive",
};

export function ConsolePage() {
  const { data: catalogue } = useCommands();
  const { data: historyData } = useHistory();
  const execute = useExecute();

  const [input, setInput] = useState("");
  const [lines, setLines] = useState<Line[]>([]);
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [pending, setPending] = useState<string | null>(null);
  const [confirmText, setConfirmText] = useState("");
  const scrollRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  // Position within the recalled history, -1 meaning "the live input".
  const [historyIndex, setHistoryIndex] = useState(-1);
  const history = useMemo(() => (historyData?.history ?? []).map((h) => h.command), [historyData]);

  const commands = catalogue?.commands ?? [];

  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight });
  }, [lines]);

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

  function append(line: Omit<Line, "id">) {
    setLines((current) => [...current, { ...line, id: crypto.randomUUID() }]);
  }

  function tierOf(commandLine: string): CommandTier {
    const name = commandLine.trim().split(/\s+/)[0]?.toLowerCase() ?? "";
    const match = commands.find(
      (c) => c.name.toLowerCase() === name || c.aliases.some((a) => a.toLowerCase() === name),
    );
    return match?.tier ?? "normal";
  }

  async function run(commandLine: string) {
    append({ kind: "input", text: commandLine });
    setInput("");
    setHistoryIndex(-1);

    try {
      const result = await execute.mutateAsync(commandLine);
      append({ kind: "output", text: result.result.trimEnd() || "(no output)" });
    } catch (cause) {
      // Always the server's own words, never a generic failure.
      const message = cause instanceof ApiError ? cause.message : "The command failed.";
      append({ kind: "error", text: message });
      toast.error("Command failed", { description: message });
    }
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

  function onKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === "ArrowUp") {
      e.preventDefault();
      const next = Math.min(historyIndex + 1, history.length - 1);
      if (next >= 0) {
        setHistoryIndex(next);
        setInput(history[next]);
      }
    } else if (e.key === "ArrowDown") {
      e.preventDefault();
      const next = historyIndex - 1;
      setHistoryIndex(next);
      setInput(next < 0 ? "" : history[next]);
    }
  }

  const pendingTier = pending ? tierOf(pending) : "normal";
  const needsTyped = pendingTier === "destructive";
  const pendingName = pending?.trim().split(/\s+/)[0] ?? "";
  const confirmReady = !needsTyped || confirmText.trim() === pendingName;

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-3">
        <h1 className="text-xl font-semibold">Console</h1>
        <Button variant="outline" size="sm" onClick={() => setPaletteOpen(true)}>
          Browse commands
          <kbd className="ml-2 text-xs text-muted-foreground">⌘K</kbd>
        </Button>
        {catalogue && (
          <span className="text-sm text-muted-foreground">
            {commands.length} commands on this server
          </span>
        )}
      </div>

      <div
        ref={scrollRef}
        className="h-[28rem] overflow-y-auto rounded-md border border-border bg-card p-4 font-mono text-sm"
      >
        {lines.length === 0 && (
          <p className="text-muted-foreground">
            Run a command to see its output. Press ↑ for earlier commands.
          </p>
        )}
        {lines.map((line) => (
          <pre
            key={line.id}
            className={
              line.kind === "input"
                ? "whitespace-pre-wrap text-primary"
                : line.kind === "error"
                  ? "whitespace-pre-wrap text-destructive"
                  : "whitespace-pre-wrap text-foreground"
            }
          >
            {line.kind === "input" ? `> ${line.text}` : line.text}
          </pre>
        ))}
      </div>

      <form onSubmit={submit} className="flex gap-2">
        <Input
          ref={inputRef}
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={onKeyDown}
          placeholder="Type a command, or press ⌘K to browse"
          autoComplete="off"
          spellCheck={false}
          className="font-mono"
        />
        <Button type="submit" disabled={execute.isPending || !input.trim()}>
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
                <code className="block rounded bg-muted px-2 py-1 font-mono text-sm text-foreground">
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
                  <pre className="whitespace-pre-wrap rounded bg-muted p-2 font-mono text-xs">
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
