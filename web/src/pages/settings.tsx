import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
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
import { useSettings, useUpdateSetting } from "@/hooks/use-settings";
import type { Setting, SettingsSection } from "@/lib/api";

export function SettingsPage() {
  const { data, isLoading, error } = useSettings();
  const [query, setQuery] = useState("");
  const [changedOnly, setChangedOnly] = useState(false);

  if (isLoading) {
    return (
      <div className="space-y-3">
        {Array.from({ length: 6 }, (_, i) => (
          <Skeleton key={i} className="h-14 w-full" />
        ))}
      </div>
    );
  }

  if (error || !data) {
    return (
      <p className="py-12 text-sm text-destructive">
        {error?.message ?? "The settings could not be loaded."}
      </p>
    );
  }

  return (
    <div className="space-y-6">
      <header>
        <p className="max-w-3xl text-sm text-muted-foreground">
          Everything the server will tell us about how this world is configured. Changes apply
          straight away, and are lost when the server restarts: the game holds them in memory and
          never writes them back to its config file.
        </p>
      </header>

      <div className="flex flex-wrap items-center gap-4">
        <Input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Search settings"
          className="max-w-xs"
          aria-label="Search settings"
        />
        <label className="flex items-center gap-2 text-sm">
          <Switch checked={changedOnly} onCheckedChange={setChangedOnly} />
          Only settings that differ from their default
        </label>
        {data.sandboxCode && (
          <span className="ml-auto text-xs text-muted-foreground">
            Sandbox code <code className="font-mono">{data.sandboxCode}</code>
          </span>
        )}
      </div>

      <Tabs defaultValue={data.sections[0]?.id ?? "world"}>
        <TabsList>
          {data.sections.map((section) => (
            <TabsTrigger key={section.id} value={section.id}>
              {section.title}
              <span className="ml-2 text-xs text-muted-foreground">{section.total}</span>
            </TabsTrigger>
          ))}
        </TabsList>

        {data.sections.map((section) => (
          <TabsContent key={section.id} value={section.id} className="mt-4">
            <Section section={section} query={query} changedOnly={changedOnly} />
          </TabsContent>
        ))}
      </Tabs>
    </div>
  );
}

function Section({
  section,
  query,
  changedOnly,
}: {
  section: SettingsSection;
  query: string;
  changedOnly: boolean;
}) {
  const groups = useMemo(() => {
    const needle = query.trim().toLowerCase();
    return section.groups
      .map((group) => ({
        ...group,
        settings: group.settings.filter((setting) => {
          if (changedOnly && !setting.changed) return false;
          if (!needle) return true;
          return (
            setting.label.toLowerCase().includes(needle) ||
            setting.name.toLowerCase().includes(needle)
          );
        }),
      }))
      .filter((group) => group.settings.length > 0);
  }, [section, query, changedOnly]);

  // Every read-only row in a section shares one reason. Repeating it on all
  // 139 of them buries the settings; it belongs at the top, once.
  const readOnlyNote = section.groups
    .flatMap((group) => group.settings)
    .find((setting) => setting.readOnlyReason)?.readOnlyReason;

  return (
    <div className="space-y-6">
      <p className="text-sm text-muted-foreground">{section.description}</p>

      {readOnlyNote && (
        <p className="rounded-md border border-border bg-muted/40 px-4 py-3 text-sm text-muted-foreground">
          {readOnlyNote}
        </p>
      )}

      {groups.length === 0 && (
        <p className="py-8 text-sm text-muted-foreground">Nothing here matches.</p>
      )}

      {groups.map((group) => (
        <section key={group.name} className="panel rounded-md">
          <h2 className="border-b border-border px-5 py-3 text-sm font-semibold">
            {group.name}
            <span className="ml-2 font-normal text-muted-foreground">{group.settings.length}</span>
          </h2>
          <ul className="divide-y divide-border">
            {group.settings.map((setting) => (
              <li key={setting.name}>
                <SettingRow setting={setting} />
              </li>
            ))}
          </ul>
        </section>
      ))}
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
    <div className="flex flex-wrap items-start gap-4 px-5 py-3">
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-sm font-medium">{setting.label}</span>
          {setting.changed && (
            <Badge variant="secondary" className="text-[11px]">
              Changed
            </Badge>
          )}
          {!setting.editable && (
            <Badge variant="outline" className="text-[11px]" title={setting.readOnlyReason}>
              Read-only
            </Badge>
          )}
        </div>
        <code className="text-xs text-muted-foreground">{setting.name}</code>
        {setting.description && (
          <p className="mt-1 text-sm text-muted-foreground">{setting.description}</p>
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
