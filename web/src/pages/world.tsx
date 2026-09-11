import { useState, type ReactNode } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useDashboard } from "@/hooks/use-dashboard";
import { api, ApiError, type ActionResult, type WeatherSetting } from "@/lib/api";
import { BLOOD_MOON_CYCLE } from "@/lib/format";

/** A labelled group of controls. */
function Panel({
  title,
  description,
  children,
}: {
  title: string;
  description: string;
  children: ReactNode;
}) {
  return (
    <section className="rounded-md border border-border bg-card p-5">
      <h2 className="font-semibold">{title}</h2>
      <p className="mt-1 mb-4 text-sm text-muted-foreground">{description}</p>
      {children}
    </section>
  );
}

/** Weather knobs, with the ranges the game's own help text documents. */
const WEATHER: { value: WeatherSetting; label: string; min: number; max: number; step: number }[] =
  [
    { value: "Rain", label: "Rain", min: 0, max: 1, step: 0.1 },
    { value: "Clouds", label: "Clouds", min: 0, max: 1, step: 0.1 },
    { value: "SnowFall", label: "Snowfall", min: 0, max: 1, step: 0.1 },
    { value: "Fog", label: "Fog", min: 0, max: 1, step: 0.1 },
    { value: "Wind", label: "Wind", min: 0, max: 200, step: 5 },
    { value: "Temp", label: "Temperature", min: -99, max: 101, step: 1 },
  ];

export function WorldPage() {
  const queryClient = useQueryClient();
  const { data: dashboard } = useDashboard();

  // Every action funnels through here so success and failure are reported the
  // same way, always with the server's own words.
  const [confirm, setConfirm] = useState<{
    title: string;
    body: string;
    run: () => Promise<ActionResult>;
  } | null>(null);

  const action = useMutation({
    mutationFn: (run: () => Promise<ActionResult>) => run(),
    onSuccess: (result) => {
      toast.success(result.result.trim() || "Done", {
        description: result.command,
      });
      void queryClient.invalidateQueries({ queryKey: ["dashboard"] });
      void queryClient.invalidateQueries({ queryKey: ["console", "history"] });
    },
    onError: (error) => {
      toast.error("That did not work", {
        description: error instanceof ApiError ? error.message : "The action failed.",
      });
    },
  });

  function ask(title: string, body: string, run: () => Promise<ActionResult>) {
    setConfirm({ title, body, run });
  }

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-semibold">World</h1>

      <div className="grid gap-6 lg:grid-cols-2">
        <TimeControls dashboard={dashboard} onAsk={ask} />
        <WeatherControls onAsk={ask} onReset={() => action.mutate(api.resetWeather)} />
        <SpawnControls onAsk={ask} />
        <BroadcastControls onAsk={ask} />
      </div>

      <Dialog open={confirm !== null} onOpenChange={(open) => !open && setConfirm(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{confirm?.title}</DialogTitle>
            <DialogDescription>{confirm?.body}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirm(null)}>
              Cancel
            </Button>
            <Button
              disabled={action.isPending}
              onClick={() => {
                const run = confirm!.run;
                setConfirm(null);
                action.mutate(run);
              }}
            >
              Do it
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

type Ask = (title: string, body: string, run: () => Promise<ActionResult>) => void;

function TimeControls({
  dashboard,
  onAsk,
}: {
  dashboard: ReturnType<typeof useDashboard>["data"];
  onAsk: Ask;
}) {
  const [day, setDay] = useState("");
  const [hour, setHour] = useState("");
  const [minute, setMinute] = useState("0");

  const currentDay = dashboard?.world.day ?? 1;
  const nextBloodMoonDay = dashboard?.bloodMoon?.nextDay;

  return (
    <Panel
      title="Time"
      description="Move the in-game clock. The blood moon is tied to the day number, so this is also how you bring one forward."
    >
      <div className="flex flex-wrap items-end gap-3">
        <div className="w-20 space-y-1">
          <Label htmlFor="day">Day</Label>
          <Input
            id="day"
            inputMode="numeric"
            value={day}
            onChange={(e) => setDay(e.target.value)}
            placeholder={String(currentDay)}
          />
        </div>
        <div className="w-20 space-y-1">
          <Label htmlFor="hour">Hour</Label>
          <Input
            id="hour"
            inputMode="numeric"
            value={hour}
            onChange={(e) => setHour(e.target.value)}
            placeholder="0–23"
          />
        </div>
        <div className="w-20 space-y-1">
          <Label htmlFor="minute">Minute</Label>
          <Input
            id="minute"
            inputMode="numeric"
            value={minute}
            onChange={(e) => setMinute(e.target.value)}
          />
        </div>
        <Button
          onClick={() => {
            const d = Number(day || currentDay);
            const h = Number(hour);
            const m = Number(minute || 0);
            onAsk(
              "Change the time?",
              `The clock will jump to day ${d}, ${String(h).padStart(2, "0")}:${String(m).padStart(2, "0")}.`,
              () => api.setTime(d, h, m),
            );
          }}
          disabled={hour === ""}
        >
          Set time
        </Button>
      </div>

      {nextBloodMoonDay !== undefined && (
        <div className="mt-4 border-t border-border pt-4">
          <Button
            variant="outline"
            onClick={() =>
              onAsk(
                "Bring on the blood moon?",
                `The clock will jump to day ${nextBloodMoonDay} at 21:00, just before the horde arrives.`,
                () => api.setTime(nextBloodMoonDay, 21, 0),
              )
            }
          >
            Jump to blood moon (day {nextBloodMoonDay})
          </Button>
          <p className="mt-2 text-xs text-muted-foreground">
            There is no command to trigger a blood moon directly, and the API's blood moon endpoint
            is read-only. Moving the clock to its day is the only way to cause one. The cycle is{" "}
            {BLOOD_MOON_CYCLE} days.
          </p>
        </div>
      )}
    </Panel>
  );
}

function WeatherControls({ onAsk, onReset }: { onAsk: Ask; onReset: () => void }) {
  const [setting, setSetting] = useState<WeatherSetting>("Rain");
  const [value, setValue] = useState("0.5");
  const knob = WEATHER.find((w) => w.value === setting)!;

  return (
    <Panel
      title="Weather"
      description="Override one weather parameter. The game keeps simulating until you reset it."
    >
      <div className="flex flex-wrap items-end gap-3">
        <div className="space-y-1">
          <Label>Setting</Label>
          <Select value={setting} onValueChange={(v) => setSetting(v as WeatherSetting)}>
            <SelectTrigger className="w-40">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {WEATHER.map((w) => (
                <SelectItem key={w.value} value={w.value}>
                  {w.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="w-28 space-y-1">
          <Label htmlFor="weather-value">
            Value ({knob.min}–{knob.max})
          </Label>
          <Input
            id="weather-value"
            inputMode="decimal"
            value={value}
            onChange={(e) => setValue(e.target.value)}
          />
        </div>
        <Button
          onClick={() =>
            onAsk(
              "Change the weather?",
              `${knob.label} will be forced to ${value} until you reset it.`,
              () => api.setWeather(setting, Number(value)),
            )
          }
        >
          Apply
        </Button>
        <Button variant="outline" onClick={onReset}>
          Reset to simulated
        </Button>
      </div>
    </Panel>
  );
}

function SpawnControls({ onAsk }: { onAsk: Ask }) {
  const [query, setQuery] = useState("zombie");
  const [entityClass, setEntityClass] = useState("");
  const [coords, setCoords] = useState({ x: "0", y: "-1", z: "0" });
  const [count, setCount] = useState("1");

  const { data } = useQuery({
    queryKey: ["entities", query],
    queryFn: () => api.searchEntities(query),
    enabled: query.trim().length > 0,
  });

  return (
    <Panel
      title="Spawn"
      description="Place entities in the world. Only classes the game allows to be spawned manually are listed."
    >
      <div className="space-y-3">
        <div className="space-y-1">
          <Label htmlFor="entity-search">Search</Label>
          <Input
            id="entity-search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="zombie, animal, vehicle…"
          />
        </div>

        <div className="space-y-1">
          <Label>Entity</Label>
          <Select value={entityClass} onValueChange={setEntityClass}>
            <SelectTrigger>
              <SelectValue placeholder={data ? `${data.total} matches` : "Search first"} />
            </SelectTrigger>
            <SelectContent>
              {(data?.entities ?? []).map((e) => (
                <SelectItem key={e.name} value={e.name}>
                  {e.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="flex flex-wrap items-end gap-2">
          {(["x", "y", "z"] as const).map((axis) => (
            <div key={axis} className="w-20 space-y-1">
              <Label htmlFor={`coord-${axis}`}>{axis.toUpperCase()}</Label>
              <Input
                id={`coord-${axis}`}
                inputMode="numeric"
                value={coords[axis]}
                onChange={(e) => setCoords({ ...coords, [axis]: e.target.value })}
              />
            </div>
          ))}
          <div className="w-20 space-y-1">
            <Label htmlFor="spawn-count">Count</Label>
            <Input
              id="spawn-count"
              inputMode="numeric"
              value={count}
              onChange={(e) => setCount(e.target.value)}
            />
          </div>
          <Button
            disabled={!entityClass}
            onClick={() =>
              onAsk(
                "Spawn entities?",
                `${count} × ${entityClass} at ${coords.x}, ${coords.y}, ${coords.z}.`,
                () =>
                  api.spawn(
                    entityClass,
                    Number(coords.x),
                    Number(coords.y),
                    Number(coords.z),
                    Number(count),
                  ),
              )
            }
          >
            Spawn
          </Button>
        </div>

        <p className="text-xs text-muted-foreground">
          Y of −1 drops them onto the ground. Entities only persist in chunks a player has loaded,
          so spawning into an empty world does nothing visible.
        </p>

        <div className="border-t border-border pt-3">
          <Button
            variant="outline"
            onClick={() =>
              onAsk(
                "Send a wandering horde?",
                "A wandering horde will make its way across the map. This is not a blood moon.",
                api.wanderingHorde,
              )
            }
          >
            Send a wandering horde
          </Button>
        </div>
      </div>
    </Panel>
  );
}

function BroadcastControls({ onAsk }: { onAsk: Ask }) {
  const [message, setMessage] = useState("");

  return (
    <Panel
      title="Broadcast"
      description="Send a message to everyone on the server, shown as coming from the server."
    >
      <div className="flex flex-wrap items-end gap-3">
        <div className="min-w-64 flex-1 space-y-1">
          <Label htmlFor="broadcast">Message</Label>
          <Input
            id="broadcast"
            value={message}
            onChange={(e) => setMessage(e.target.value)}
            placeholder="Restarting in 5 minutes"
          />
        </div>
        <Button
          disabled={!message.trim()}
          onClick={() =>
            onAsk("Broadcast this?", `Everyone online will see: ${message}`, () => {
              const text = message;
              setMessage("");
              return api.say(text);
            })
          }
        >
          Send
        </Button>
      </div>
      <p className="mt-2 text-xs text-muted-foreground">
        Quotes and line breaks are rejected, because the game console has no documented way to
        escape them.
      </p>
    </Panel>
  );
}
