import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Skeleton } from "@/components/ui/skeleton";
import { countMatches, matches, sectionsWithMatches } from "@/lib/settings-search";
import { useSettings, useUpdateSetting } from "@/hooks/use-settings";
import { useServerId } from "@/hooks/use-servers";
import { api, ApiError, type ActionResult, type Setting, type SettingsSection } from "@/lib/api";
import { cn } from "@/lib/utils";

export function SettingsPage() {
  const { data, isLoading, error } = useSettings();
  const [query, setQuery] = useState("");
  const [changedOnly, setChangedOnly] = useState(false);
  const needle = query.trim().toLowerCase();
  // Controlled, so that "found it in server settings" can actually take you
  // there rather than telling you to go yourself.
  const [tab, setTab] = useState<string | null>(null);

  if (isLoading) {
    return (
      <div className="space-y-px p-4">
        {Array.from({ length: 8 }, (_, i) => (
          <Skeleton key={i} className="h-14 w-full rounded-none" />
        ))}
      </div>
    );
  }

  if (error || !data) {
    return (
      <p className="p-8 text-center text-sm text-destructive">
        {error?.message ?? "The settings could not be loaded."}
      </p>
    );
  }

  return (
    <Tabs
      value={tab ?? data.sections[0]?.id ?? "world"}
      onValueChange={setTab}
      className="flex h-full min-h-0 flex-col gap-0"
    >
      {/*
        The filters stay put and the list moves under them. Nearly three
        hundred settings scrolled the search box off the top of the screen
        exactly when somebody needed it.
      */}
      <div className="flex shrink-0 flex-wrap items-center gap-3 border-b border-border px-4 py-2.5 md:px-6">
        <TabsList className="h-7 rounded-none bg-transparent p-0">
          {data.sections.map((section) => (
            <TabsTrigger
              key={section.id}
              value={section.id}
              className="h-7 gap-1.5 rounded-none border-0 px-3 data-[state=active]:bg-accent"
            >
              <span className="stencil">{section.title}</span>
              {/*
                While searching this counts matches rather than the section's
                size: the whole problem was not being able to tell which tab
                the thing you typed actually lives in.
              */}
              <span
                className={cn(
                  "readout text-xs",
                  needle && countMatches(section, needle, changedOnly) > 0
                    ? "text-ember"
                    : "text-bone-faint",
                )}
              >
                {needle ? countMatches(section, needle, changedOnly) : section.total}
              </span>
            </TabsTrigger>
          ))}
          {/* Not a preferences section: these are console commands rather than
              gameprefs, so they come from a different place and are kept
              visibly apart from the rows that read and write settings. */}
          <TabsTrigger
            value="access"
            className="h-7 gap-1.5 rounded-none border-0 px-3 data-[state=active]:bg-accent"
          >
            <span className="stencil">Access</span>
          </TabsTrigger>
        </TabsList>

        <Input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Search settings"
          className="ml-auto h-7 max-w-56 text-xs"
          aria-label="Search settings"
        />
        <label className="flex items-center gap-2">
          <Switch checked={changedOnly} onCheckedChange={setChangedOnly} />
          <span className="stencil">Changed only</span>
        </label>
      </div>

      {data.sections.map((section) => (
        <TabsContent key={section.id} value={section.id} className="min-h-0 flex-1 overflow-hidden">
          <Section
            section={section}
            query={query}
            changedOnly={changedOnly}
            elsewhere={sectionsWithMatches(data.sections, needle, changedOnly, section.id)}
            onGo={setTab}
          />
        </TabsContent>
      ))}

      <TabsContent value="access" className="min-h-0 flex-1 overflow-y-auto">
        <AccessTab />
      </TabsContent>

      {data.sandboxCode && (
        <p className="readout shrink-0 border-t border-border px-4 py-2 text-xs text-bone-faint md:px-6">
          sandbox code {data.sandboxCode}
        </p>
      )}
    </Tabs>
  );
}

/**
 * One section, with its groups down the left.
 *
 * Tabs alone were not enough: "World rules" is 165 settings across eight
 * groups, so picking a tab still left somebody scrolling past a hundred rows
 * about crafting to reach the one about traders. The groups are a second axis
 * and they get their own navigation, showing one group at a time.
 *
 * Searching overrides the group, because when you are looking for a word you
 * do not know which group it is in — that is why you are searching.
 */
function Section({
  section,
  query,
  changedOnly,
  elsewhere,
  onGo,
}: {
  section: SettingsSection;
  query: string;
  changedOnly: boolean;
  /** Sections that do have matches, so an empty one can point at them. */
  elsewhere: SettingsSection[];
  /** Switches tabs, so the pointer is a link rather than an instruction. */
  onGo: (sectionId: string) => void;
}) {
  const [active, setActive] = useState<string | null>(null);
  const searching = query.trim() !== "";

  const groups = useMemo(() => {
    const needle = query.trim().toLowerCase();
    return section.groups
      .map((group) => ({
        ...group,
        settings: group.settings.filter((setting) => matches(setting, needle, changedOnly)),
      }))
      .filter((group) => group.settings.length > 0);
  }, [section, query, changedOnly]);

  // A group that filters down to nothing should not stay selected and show an
  // empty page; fall back to the first group that still has rows.
  const current = groups.find((group) => group.name === active) ?? groups[0];
  const shown = searching ? groups : current ? [current] : [];
  const total = groups.reduce((n, group) => n + group.settings.length, 0);

  // Every read-only row in a section shares one reason. Repeating it on all
  // 139 of them buries the settings; it belongs at the top, once.
  const readOnlyNote = section.groups
    .flatMap((group) => group.settings)
    .find((setting) => setting.readOnlyReason)?.readOnlyReason;

  return (
    <div className="flex h-full min-h-0">
      <nav
        aria-label={`${section.title} groups`}
        className="hidden w-48 shrink-0 flex-col overflow-y-auto border-r border-border lg:flex"
      >
        {groups.map((group) => {
          const selected = !searching && group.name === current?.name;
          return (
            <button
              key={group.name}
              type="button"
              onClick={() => setActive(group.name)}
              className={cn(
                "relative flex items-baseline gap-2 border-b border-border px-4 py-2.5 text-left transition-colors",
                selected
                  ? "bg-accent before:absolute before:inset-y-0 before:left-0 before:w-0.5 before:bg-crimson-lit"
                  : "hover:bg-accent/50",
              )}
            >
              <span className={cn("stencil", selected && "text-bone")}>{group.name}</span>
              <span className="readout ml-auto text-xs text-bone-faint">
                {group.settings.length}
              </span>
            </button>
          );
        })}
      </nav>

      <div className="min-h-0 min-w-0 flex-1 overflow-y-auto">
        <div className="border-b border-border px-4 py-3 md:px-6">
          <p className="max-w-prose text-xs text-bone-dim">{section.description}</p>
          {readOnlyNote && (
            <p className="mt-2 max-w-prose text-xs text-bone-faint">{readOnlyNote}</p>
          )}
        </div>

        {searching && (
          <p className="border-b border-border px-4 py-2 md:px-6">
            <span className="stencil">
              {total} {total === 1 ? "match" : "matches"} across {groups.length}{" "}
              {groups.length === 1 ? "group" : "groups"}
            </span>
          </p>
        )}

        {shown.length === 0 && (
          <div className="p-8 text-center">
            <p className="text-sm text-bone-faint">Nothing in here matches.</p>
            {/*
              The whole bug, in one sentence. "Nothing here matches" was a lie
              when the setting sat one tab over, and it ended the search.
            */}
            {elsewhere.length > 0 && (
              <p className="mt-2 text-xs text-bone-dim">
                Found it in{" "}
                {elsewhere.map((other, i) => (
                  <span key={other.id}>
                    {i > 0 && (i === elsewhere.length - 1 ? " and " : ", ")}
                    <button
                      type="button"
                      className="text-ember underline-offset-2 hover:underline"
                      onClick={() => onGo(other.id)}
                    >
                      {other.title.toLowerCase()}
                    </button>
                  </span>
                ))}
                .
              </p>
            )}
          </div>
        )}

        {shown.map((group) => (
          <section key={group.name}>
            {/* Sticky, so a long group still says which one it is. Shown
                always while searching, since results span groups. */}
            <h2 className="region-head sticky top-0 z-10 bg-background/95 backdrop-blur">
              <span className="stencil">{group.name}</span>
              <span className="readout text-xs text-bone-faint">{group.settings.length}</span>
            </h2>
            <ul className="region divide-y divide-border border-b border-border">
              {group.settings.map((setting) => (
                <li key={setting.name}>
                  <SettingRow setting={setting} />
                </li>
              ))}
            </ul>
          </section>
        ))}
      </div>
    </div>
  );
}

/** The string form of a value, which is what the API takes and returns. */
function asText(value: Setting["value"]): string {
  if (value === null || value === undefined) return "";
  return String(value);
}

function SettingRow({ setting }: { setting: Setting }) {
  const update = useUpdateSetting();
  const current = asText(setting.value);
  const [draft, setDraft] = useState(current);

  // A refetch after somebody else changed the same setting should win over a
  // stale draft; a draft being edited right now should not be clobbered.
  useEffect(() => setDraft(current), [current]);

  const commit = (value: string) => {
    if (value === current) return;
    update.mutate(
      { name: setting.name, value },
      {
        onSuccess: (result) => {
          // The server answers with a raw value; say it back in the same words
          // the control uses.
          const applied =
            setting.choices?.find((c) => c.value === result.value)?.label ?? result.value;
          toast.success(`${setting.label} is now ${applied}`, {
            description: "This lasts until the server restarts.",
          });
        },
        onError: (err) => {
          toast.error(`${setting.label} was not changed`, { description: err.message });
          setDraft(current);
        },
      },
    );
  };

  const defaultText = setting.defaultLabel || asText(setting.default);

  return (
    <div className="flex flex-wrap items-start gap-4 px-4 py-2.5 md:px-6">
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
          <span className="text-sm font-medium">{setting.label}</span>
          <code className="readout text-xs text-bone-faint">{setting.name}</code>
          {setting.changed && (
            <Badge variant="secondary" className="text-xs">
              Changed
            </Badge>
          )}
          {!setting.editable && (
            <Badge variant="outline" className="text-xs" title={setting.readOnlyReason}>
              Read-only
            </Badge>
          )}
        </div>
        {setting.description && (
          <p className="mt-1 max-w-prose text-xs text-bone-dim">{setting.description}</p>
        )}
      </div>

      <div className="flex w-56 shrink-0 flex-col items-end gap-1">
        <Control
          setting={setting}
          draft={draft}
          setDraft={setDraft}
          commit={commit}
          busy={update.isPending}
        />
        {defaultText !== "" && (
          <span className="text-xs text-muted-foreground">Default {defaultText}</span>
        )}
      </div>
    </div>
  );
}

function Control({
  setting,
  draft,
  setDraft,
  commit,
  busy,
}: {
  setting: Setting;
  draft: string;
  setDraft: (value: string) => void;
  commit: (value: string) => void;
  busy: boolean;
}) {
  const disabled = !setting.editable || busy;

  // A read-only setting still has to show its value, and the game's own word
  // for it beats the raw number.
  if (!setting.editable) {
    return (
      <span className="text-sm font-medium">{setting.valueLabel || asText(setting.value)}</span>
    );
  }

  // Where the game enumerates the allowed values, use them. This is what makes
  // most of the page a list of choices rather than a list of numbers to guess.
  if (setting.choices && setting.choices.length > 0) {
    return (
      <Select
        value={draft}
        disabled={disabled}
        onValueChange={(value) => {
          setDraft(value);
          commit(value);
        }}
      >
        <SelectTrigger className="w-full" aria-label={setting.label}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {setting.choices.map((choice) => (
            <SelectItem key={choice.value} value={choice.value}>
              {choice.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    );
  }

  if (setting.type === "bool") {
    return (
      <Switch
        checked={draft.toLowerCase() === "true"}
        disabled={disabled}
        aria-label={setting.label}
        onCheckedChange={(checked) => {
          const value = checked ? "true" : "false";
          setDraft(value);
          commit(value);
        }}
      />
    );
  }

  const numeric = setting.type === "int" || setting.type === "float";
  return (
    <Input
      value={draft}
      disabled={disabled}
      inputMode={numeric ? "decimal" : undefined}
      aria-label={setting.label}
      className="w-full text-right"
      onChange={(e) => setDraft(e.target.value)}
      onBlur={() => commit(draft)}
      onKeyDown={(e) => {
        if (e.key === "Enter") e.currentTarget.blur();
      }}
    />
  );
}

/* ---------------------------------------------------------------- access -- */

/**
 * The parts of access control that are about the server rather than a person.
 *
 * Promoting somebody, whitelisting them or lifting their ban belongs on their
 * own page, next to everything else about them — sorting those into a separate
 * screen because the underlying command happens to take a platform id rather
 * than an entity id is the game's filing system, not an operator's. What is
 * left here is what has no player attached: the lists as the server reports
 * them, the cap, and the off switch.
 */
function AccessTab() {
  const serverId = useServerId();
  const queryClient = useQueryClient();
  const [maxPlayers, setMaxPlayers] = useState("");

  const { data, isLoading } = useQuery({
    queryKey: ["access", serverId],
    queryFn: () => api.access(serverId),
    enabled: serverId !== "",
  });

  const act = useMutation({
    mutationFn: ({ run }: { done: string; run: () => Promise<ActionResult> }) => run(),
    onSuccess: (result, { done }) => {
      toast.success(done, { description: result.command });
      void queryClient.invalidateQueries({ queryKey: ["access", serverId] });
    },
    onError: (error) =>
      toast.error("That did not work", {
        description: error instanceof ApiError ? error.message : "The action failed.",
      }),
  });

  return (
    <div className="max-w-4xl space-y-10 p-4 md:p-6">
      <div className="space-y-3">
        <div className="flex flex-wrap items-baseline gap-x-3">
          <span className="stencil">Who has what</span>
          <Link to="/players" className="text-xs text-bone-dim underline-offset-4 hover:underline">
            change it on a player&apos;s page
          </Link>
        </div>
        <div className="grid gap-3 sm:grid-cols-2">
          <div className="space-y-1.5">
            <span className="stencil text-bone-faint">Admins</span>
            <pre className="readout max-h-56 overflow-auto border border-border p-3 text-xs leading-relaxed whitespace-pre-wrap text-bone-dim">
              {isLoading ? "…" : data?.admins.trim() || "Nobody."}
            </pre>
          </div>
          <div className="space-y-1.5">
            <span className="stencil text-bone-faint">Whitelist</span>
            <pre className="readout max-h-56 overflow-auto border border-border p-3 text-xs leading-relaxed whitespace-pre-wrap text-bone-dim">
              {isLoading ? "…" : data?.whitelist.trim() || "Empty."}
            </pre>
          </div>
        </div>
        <p className="max-w-prose text-xs text-bone-faint">
          Shown as the server writes it. Group permissions and anybody promoted by an id that has
          never connected appear here and nowhere else, because the panel has no player to attach
          them to.
        </p>
      </div>

      <div className="space-y-3">
        <span className="stencil">Player cap</span>
        <p className="max-w-prose text-xs text-bone-faint">
          Applies to this run only. The game writes nothing back to serverconfig, so it reverts on
          restart.
        </p>
        <div className="flex flex-wrap items-center gap-2">
          <Input
            inputMode="numeric"
            value={maxPlayers}
            onChange={(e) => setMaxPlayers(e.target.value)}
            placeholder="8"
            className="h-8 w-20 text-xs"
            aria-label="Maximum players"
          />
          <Button
            variant="outline"
            size="sm"
            disabled={maxPlayers.trim() === ""}
            onClick={() =>
              act.mutate({
                done: `Player cap set to ${Number(maxPlayers)}`,
                run: () => api.setMaxPlayers(serverId, Number(maxPlayers)),
              })
            }
          >
            Set cap
          </Button>
        </div>
      </div>

      <div className="space-y-3 border-t border-crimson-deep pt-8">
        <span className="stencil text-crimson-lit">Shut down</span>
        <p className="max-w-prose text-xs text-bone-faint">
          Stops the game server. The panel cannot start it again — whatever runs it, systemd, Docker
          or a terminal, has to do that.
        </p>
        <Button
          variant="outline"
          size="sm"
          className="border-crimson text-crimson-lit hover:bg-crimson-deep/30"
          onClick={() =>
            act.mutate({ done: "Server shutting down", run: () => api.shutdown(serverId) })
          }
        >
          Shut down the server
        </Button>
      </div>
    </div>
  );
}
