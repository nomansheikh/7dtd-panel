import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import {
  CloudLightning,
  HardDriveDownload,
  Moon,
  PackageOpen,
  Siren,
  Skull,
  Sun,
  Trash2,
  Users,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Skeleton } from "@/components/ui/skeleton";
import { DayDial } from "@/components/day-dial";
import { Sky, type SkyConditions } from "@/components/sky";
import { WeatherMeter } from "@/components/weather-meter";
import { useDashboard } from "@/hooks/use-dashboard";
import { usePlayers } from "@/hooks/use-players";
import { useServerId } from "@/hooks/use-servers";
import {
  api,
  ApiError,
  type ActionResult,
  type BiomeWeather,
  type EntityClass,
  type Player,
  type Weather,
  type WeatherSetting,
} from "@/lib/api";
import { formatGameClock } from "@/lib/format";
import { cn } from "@/lib/utils";

/**
 * Running a command, and what to say once it works.
 *
 * The toast used to echo the game's own reply, which is written for a server
 * console and not for anybody else: setting the clock announced "Set time to
 * 258000". The caller already knows what it asked for, in the same words the
 * page uses, so it says so — and the raw command goes underneath, where it is
 * evidence rather than a headline.
 */
type Done = string;
type Ask = (title: string, body: string, done: Done, run: () => Promise<ActionResult>) => void;
type Run = (done: Done, fn: () => Promise<ActionResult>) => void;

const TABS = [
  { id: "time", label: "Time" },
  { id: "weather", label: "Weather" },
  { id: "spawn", label: "Spawn" },
  { id: "upkeep", label: "Upkeep" },
];

/**
 * The world page.
 *
 * A tab per subject rather than four groups crammed into two columns: each one
 * gets the whole width, which is what lets Weather show every parameter at
 * once instead of a dropdown that changes one at a time.
 */
export function WorldPage() {
  const serverId = useServerId();
  const queryClient = useQueryClient();

  const [confirm, setConfirm] = useState<{
    title: string;
    body: string;
    done: Done;
    run: () => Promise<ActionResult>;
  } | null>(null);

  const action = useMutation({
    mutationFn: ({ run }: { done: Done; run: () => Promise<ActionResult> }) => run(),
    onSuccess: (result, { done }) => {
      toast.success(done, { description: result.command });
      void queryClient.invalidateQueries({ queryKey: ["dashboard", serverId] });
      void queryClient.invalidateQueries({ queryKey: ["weather", serverId] });
      void queryClient.invalidateQueries({ queryKey: ["console", "history", serverId] });
    },
    onError: (error) =>
      toast.error("That did not work", {
        description: error instanceof ApiError ? error.message : String(error),
      }),
  });

  const run: Run = (done, fn) => action.mutate({ done, run: fn });
  const ask: Ask = (title, body, done, fn) => setConfirm({ title, body, done, run: fn });

  return (
    <Tabs defaultValue="time" className="flex h-full min-h-0 flex-col gap-0">
      <TabsList className="h-auto w-full shrink-0 justify-start rounded-none border-b border-border bg-transparent p-0">
        {TABS.map((tab) => (
          <TabsTrigger
            key={tab.id}
            value={tab.id}
            className="grow-0 rounded-none border-0 px-5 py-2.5 data-[state=active]:bg-accent"
          >
            <span className="stencil">{tab.label}</span>
          </TabsTrigger>
        ))}
      </TabsList>

      <TabsContent value="time" className="min-h-0 flex-1 overflow-y-auto">
        <TimeTab serverId={serverId} onRun={run} />
      </TabsContent>
      <TabsContent value="weather" className="min-h-0 flex-1 overflow-y-auto">
        <WeatherTab serverId={serverId} onRun={run} onAsk={ask} />
      </TabsContent>
      <TabsContent value="spawn" className="min-h-0 flex-1 overflow-y-auto">
        <SpawnTab serverId={serverId} onRun={run} onAsk={ask} />
      </TabsContent>
      <TabsContent value="upkeep" className="min-h-0 flex-1 overflow-y-auto">
        <UpkeepTab serverId={serverId} onRun={run} onAsk={ask} />
      </TabsContent>

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
                const { done, run: fn } = confirm!;
                setConfirm(null);
                action.mutate({ done, run: fn });
              }}
            >
              Do it
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Tabs>
  );
}

/* ------------------------------------------------------------------ time -- */

/**
 * Dawn is fixed; dusk is dawn plus however long this server makes the day.
 *
 * The server reports DayLightLength with every serverinfo poll, so the band is
 * drawn from the world's own setting rather than assuming the default of 18.
 * A server running six-hour days gets a six-hour band.
 */
const DAWN_HOUR = 4;
const DEFAULT_DAYLIGHT_HOURS = 18;

/**
 * How much real time a stretch of game time takes.
 *
 * The server reports DayNightLength, the real minutes in a whole game day, so
 * this is arithmetic rather than a guess. It is the one thing an operator
 * genuinely cannot work out in their head, and the reason the countdowns below
 * are worth the space.
 */
function realMinutes(gameHours: number, dayMinutes: number): number {
  return gameHours * (dayMinutes / 24);
}

/** "3h 43m", or "42m" when there is no hour to show. */
function humanMinutes(total: number): string {
  const m = Math.max(0, Math.round(total));
  if (m < 60) return `${m}m`;
  return `${Math.floor(m / 60)}h ${String(m % 60).padStart(2, "0")}m`;
}

/** "2 days 14h", for a stretch of in-game time. */
function humanGameTime(hours: number): string {
  const h = Math.max(0, Math.round(hours));
  const days = Math.floor(h / 24);
  const rest = h % 24;
  if (days === 0) return `${rest}h`;
  return `${days} ${days === 1 ? "day" : "days"} ${rest}h`;
}

function TimeTab({ serverId, onRun }: { serverId: string; onRun: Run }) {
  const { data: dashboard } = useDashboard();
  const world = dashboard?.world;

  const [pending, setPending] = useState<{ day: number; hour: number; minute: number } | null>(
    null,
  );

  // Matching on the time arriving rather than on any change to it, because a
  // poll can already be in flight when the command goes out and that one still
  // carries the old clock.
  const settled =
    pending !== null &&
    world !== undefined &&
    pending.day === world.day &&
    Math.abs(pending.hour * 60 + pending.minute - (world.hour * 60 + world.minute)) <= 30;
  useEffect(() => {
    if (settled) setPending(null);
  }, [settled]);

  // A command can fail or be refused. Without this the page would show a time
  // nothing agrees with for as long as it stayed open.
  useEffect(() => {
    if (pending === null) return;
    const giveUp = setTimeout(() => setPending(null), 6000);
    return () => clearTimeout(giveUp);
  }, [pending]);

  if (!dashboard || !world) {
    return (
      <div className="p-6">
        <Skeleton className="h-64 w-full rounded-none" />
      </div>
    );
  }

  const lit =
    world.daylightHours && world.daylightHours > 0 ? world.daylightHours : DEFAULT_DAYLIGHT_HOURS;
  const dayMinutes = world.dayMinutes && world.dayMinutes > 0 ? world.dayMinutes : 60;
  const duskHour = DAWN_HOUR + lit;

  // The whole tab reads from the clock we asked for until the server confirms
  // it, so the dial, the countdowns and the schedule all move together the
  // instant something is clicked. Holding it in one place rather than inside
  // the dial is what lets a row of the schedule feel as immediate as a drag.
  const day = pending?.day ?? world.day;
  const hour = pending?.hour ?? world.hour;
  const minute = pending?.minute ?? world.minute;
  const now = hour + minute / 60;
  const isNight = now < DAWN_HOUR || now >= duskHour;

  // Hours until the light changes, wrapping past midnight.
  const untilDawn = now < DAWN_HOUR ? DAWN_HOUR - now : 24 - now + DAWN_HOUR;
  const nextChange = isNight
    ? { label: "until dawn", hours: untilDawn }
    : { label: "until dusk", hours: duskHour - now };

  const moon = dashboard.bloodMoon;
  const moonHours = moon ? (moon.nextDay - day) * 24 + (moon.nextHour - now) : 0;

  /** Sets the clock, and shows it there straight away. */
  function setTime(d: number, h: number, m: number) {
    setPending({ day: d, hour: h, minute: m });
    onRun(`Day ${d}, ${formatGameClock(h, m)}`, () => api.setTime(serverId, d, h, m));
  }

  /** Moves the clock forward, rolling into the next day where it needs to. */
  function skip(hours: number) {
    // Counted in whole minutes throughout: working in fractional hours and
    // rounding at the end could land on minute 60, which is not a time.
    const total = Math.round((day * 24 + now + hours) * 60);
    const rest = total % (24 * 60);
    setTime(Math.floor(total / (24 * 60)), Math.floor(rest / 60), rest % 60);
  }

  /*
    The next time each thing happens, in order, with how far off it is.

    This is the part an operator actually asks the panel: not "what hour is it"
    but "how long have they got before dark", and — since a game day is an hour
    of someone's evening — how long that is in real minutes. Every row is also
    the way to get there, so reading the schedule and skipping to a point in it
    are the same control rather than two.
  */
  const moments = [
    { label: "Dawn", hour: DAWN_HOUR },
    { label: "Noon", hour: 12 },
    { label: "Dusk", hour: duskHour % 24 },
    { label: "Midnight", hour: 0 },
  ]
    .map((m) => ({ ...m, ahead: (m.hour - now + 24) % 24 || 24 }))
    .sort((a, b) => a.ahead - b.ahead);

  return (
    <div className="p-4 md:p-8">
      <div className="flex flex-wrap items-start gap-x-12 gap-y-8">
        <div className="shrink-0 space-y-3">
          <DayDial
            day={day}
            hour={hour}
            minute={minute}
            daylightHours={lit}
            dawnHour={DAWN_HOUR}
            hordeHour={moon?.nextHour}
            hordeTonight={moon?.active || moon?.nextDay === day}
            onSetTime={(h, m) => setTime(day, h, m)}
          />

          {/*
            Under the dial rather than in the prose on the right, because that
            is where someone's eye already is when they wonder whether the sun
            is a picture or a control.
          */}
          <p className="w-[360px] max-w-full text-center text-xs text-bone-dim">
            Drag the sun round the ring to change the time.
          </p>
        </div>

        <div className="min-w-0 flex-1 space-y-7">
          <p className="flex items-center gap-2">
            {isNight ? (
              <Moon className="size-4 text-crimson-lit" />
            ) : (
              <Sun className="size-4 text-ember" />
            )}
            <span className={cn("stencil", isNight ? "text-crimson-lit" : "text-ember")}>
              {isNight ? "Night — they run" : "Daylight"}
            </span>
            <span className="readout text-xs text-bone-faint">
              {humanGameTime(nextChange.hours)} {nextChange.label}
            </span>
          </p>

          {/*
            Game hours mean nothing on their own when a day takes an hour of
            real time, so both are given. It is the one sum an operator cannot
            do in their head.
          */}
          {/*
            Only the blood moon gets a headline. Dusk and dawn used to have one
            too, and it said exactly what the first row of the schedule below
            says — the same number twice, a hand's width apart.
          */}
          {moon && (
            <dl>
              <Countdown
                label={moon.active ? "blood moon" : "until blood moon"}
                gameHours={moon.active ? 0 : moonHours}
                dayMinutes={dayMinutes}
                tone="moon"
                note={
                  moon.active
                    ? "happening now"
                    : `day ${moon.nextDay}, ${String(moon.nextHour).padStart(2, "0")}:00`
                }
              />
            </dl>
          )}

          <div className="space-y-2">
            <span className="stencil">What happens next</span>
            <ul className="border border-border">
              {moments.map((m) => (
                <li key={m.label} className="border-b border-border last:border-b-0">
                  <button
                    type="button"
                    className="flex w-full items-baseline gap-4 px-3 py-2.5 text-left transition-colors hover:bg-accent"
                    onClick={() => skip(m.ahead)}
                  >
                    <span className="readout w-14 shrink-0 text-xs text-bone-dim">
                      {String(m.hour).padStart(2, "0")}:00
                    </span>
                    <span className="min-w-0 flex-1 truncate text-sm text-bone">{m.label}</span>
                    <span className="readout shrink-0 text-xs text-bone-dim">
                      in {humanGameTime(m.ahead)}
                    </span>
                    <span className="readout hidden w-20 shrink-0 text-right text-xs text-bone-faint sm:inline">
                      {humanMinutes(realMinutes(m.ahead, dayMinutes))} real
                    </span>
                  </button>
                </li>
              ))}
              {moon && !moon.active && (
                <li className="border-t border-border">
                  <button
                    type="button"
                    className="flex w-full items-baseline gap-4 px-3 py-2.5 text-left transition-colors hover:bg-accent"
                    onClick={() => skip(moonHours)}
                  >
                    <span className="readout w-14 shrink-0 text-xs text-crimson-lit">
                      {String(moon.nextHour).padStart(2, "0")}:00
                    </span>
                    <span className="min-w-0 flex-1 truncate text-sm text-crimson-lit">
                      Blood moon
                      <span className="ml-2 text-bone-faint">day {moon.nextDay}</span>
                    </span>
                    <span className="readout shrink-0 text-xs text-bone-dim">
                      in {humanGameTime(moonHours)}
                    </span>
                    <span className="readout hidden w-20 shrink-0 text-right text-xs text-bone-faint sm:inline">
                      {humanMinutes(realMinutes(moonHours, dayMinutes))} real
                    </span>
                  </button>
                </li>
              )}
            </ul>
          </div>

          <div className="space-y-2">
            <span className="stencil">Skip forward</span>
            <div className="flex flex-wrap gap-1">
              {[1, 6, 12, 24].map((h) => (
                <Button
                  key={h}
                  variant="ghost"
                  size="sm"
                  className="gap-1.5 px-2.5"
                  onClick={() => skip(h)}
                >
                  <span className="text-bone">+{h === 24 ? "1 day" : `${h}h`}</span>
                  <span className="readout text-xs text-bone-faint">
                    {humanMinutes(realMinutes(h, dayMinutes))} real
                  </span>
                </Button>
              ))}
            </div>
          </div>

          <p className="max-w-prose text-xs text-bone-faint">
            Noon at the top, midnight at the bottom: the ring is the sun's path. Light runs{" "}
            {String(DAWN_HOUR).padStart(2, "0")}:00 to {String(duskHour % 24).padStart(2, "0")}:00 —{" "}
            {lit} of 24 hours — and a whole day takes {dayMinutes} real minutes.
          </p>
        </div>
      </div>
    </div>
  );
}

/** One countdown, in game time and in real time. */
function Countdown({
  label,
  gameHours,
  dayMinutes,
  tone,
  note,
}: {
  label: string;
  gameHours: number;
  dayMinutes: number;
  tone?: "moon";
  note?: string;
}) {
  return (
    <div>
      <dt className={cn("stencil", tone === "moon" && "text-crimson-lit")}>{label}</dt>
      <dd className="mt-1.5">
        <div className={cn("figure text-3xl", tone === "moon" && "text-crimson-lit")}>
          {gameHours > 0 ? humanGameTime(gameHours) : "now"}
        </div>
        {/* On its own line: beside the game figure, "17h" and "43m real" read
            as a single "17h 43m". */}
        {gameHours > 0 && (
          <div className="readout mt-1 text-xs text-bone-dim">
            ≈ {humanMinutes(realMinutes(gameHours, dayMinutes))} of real time
          </div>
        )}
        {note && <div className="readout mt-0.5 text-xs text-bone-faint">{note}</div>}
      </dd>
    </div>
  );
}

/* --------------------------------------------------------------- weather -- */

const WEATHER_PRESETS: { label: string; knobs: [WeatherSetting, number][] }[] = [
  {
    label: "Clear",
    knobs: [
      ["Clouds", 0],
      ["Rain", 0],
      ["SnowFall", 0],
      ["Fog", 0],
    ],
  },
  {
    label: "Overcast",
    knobs: [
      ["Clouds", 0.85],
      ["Rain", 0],
      ["Fog", 0.15],
    ],
  },
  {
    label: "Rain",
    knobs: [
      ["Clouds", 0.9],
      ["Rain", 0.8],
      ["Fog", 0.3],
    ],
  },
  {
    label: "Snow",
    knobs: [
      ["Clouds", 0.9],
      ["SnowFall", 0.8],
      ["Temp", 10],
    ],
  },
  {
    label: "Fog",
    knobs: [
      ["Fog", 0.9],
      ["Clouds", 0.5],
    ],
  },
  {
    label: "Gale",
    knobs: [
      ["Wind", 90],
      ["Clouds", 0.7],
    ],
  },
];

/**
 * The weather parameters, in the units their commands take.
 *
 * `of` names the matching field in the override block the server prints at the
 * end of its weather report. That block is the only part of the report in these
 * units: the per-biome lines alongside it are the simulation's own internals
 * and are on entirely different scales — cloud comes back as a thickness up to
 * a hundred where the command takes a fraction, and fog was observed between
 * 0.9 and 3.2 across five biomes where the command takes nought to one.
 *
 * So these bars are what has been forced, and the scene beside them is what is
 * actually happening. They are two different facts and the page says so. Note
 * that the server reports a cleared override and one deliberately set to zero
 * identically, which is why the sky and not this is the source of truth.
 */
const WEATHER: {
  value: WeatherSetting;
  label: string;
  min: number;
  max: number;
  of: keyof Weather["overrides"];
  describe: (value: number) => string;
}[] = [
  {
    value: "Rain",
    label: "Rain",
    min: 0,
    max: 1,
    of: "rain",
    describe: (v) => portion(v, ["dry", "spitting", "steady", "heavy", "torrential"]),
  },
  {
    value: "SnowFall",
    label: "Snowfall",
    min: 0,
    max: 1,
    of: "snow",
    describe: (v) => portion(v, ["none", "flurries", "settling", "heavy", "whiteout"]),
  },
  {
    value: "Clouds",
    label: "Cloud",
    min: 0,
    max: 1,
    of: "clouds",
    describe: (v) => portion(v, ["clear", "broken", "clouded", "overcast", "black"]),
  },
  {
    value: "Fog",
    label: "Fog",
    min: 0,
    max: 1,
    of: "fog",
    describe: (v) => portion(v, ["none", "haze", "misty", "thick", "blind"]),
  },
  {
    value: "Wind",
    label: "Wind",
    min: 0,
    max: 200,
    of: "wind",
    describe: (v) =>
      `${Math.round(v)} · ${band(
        Math.round(v),
        [
          [15, "still"],
          [45, "breeze"],
          [90, "blowing"],
          [150, "gale"],
        ],
        "screaming",
      )}`,
  },
  {
    value: "Temp",
    label: "Temperature",
    min: -99,
    max: 101,
    of: "temperature",
    describe: (v) =>
      `${Math.round(v)}° · ${band(
        Math.round(v),
        [
          [0, "lethal"],
          [33, "freezing"],
          [55, "cold"],
          [80, "mild"],
          [95, "hot"],
        ],
        "blistering",
      )}`,
  },
];

/**
 * The word for a reading, by threshold rather than by proportion.
 *
 * Spreading five words evenly across a range only works when the range is the
 * range things actually occur in. Temperature takes -99 to 101 but a world
 * sits near 77, which an even split called blistering.
 */
function band(v: number, steps: [number, string][], last: string): string {
  for (const [below, name] of steps) if (v < below) return name;
  return last;
}

/** The word for a nought-to-one reading, and the reading itself. */
function portion(v: number, words: string[]): string {
  return `${Math.round(v * 100)}% · ${word(v, words)}`;
}

function word(v: number, words: string[]): string {
  const i = Math.min(words.length - 1, Math.max(0, Math.round(v * (words.length - 1))));
  return words[i];
}

const clamp01 = (n: number) => Math.min(1, Math.max(0, n));

/**
 * What the sky actually looks like, from the two things the server reports.
 *
 * A forced value wins wherever there is one, because a forced value is what
 * players are standing in — the per-biome line beneath it is the simulation the
 * override is sitting on top of, and it goes on reporting its own numbers
 * regardless. Checked against a live server: with cloud forced to 0.9 and rain
 * to 0.8, pine_forest still read cloud 0 and rain 0 twenty seconds later,
 * completely unmoved. Driving the picture from the biome line meant a preset
 * showed for one frame and then vanished, which is exactly what it looked like.
 *
 * Nought is treated as nothing being forced. That is not a choice so much as an
 * admission: the server prints a cleared override and one deliberately set to
 * zero identically, and falling through to the simulation is the better answer
 * in the case that is far more common.
 *
 * Each quantity also arrives on its own scale — cloud as a thickness to a
 * hundred where the command takes a fraction, fog somewhere around nought to
 * four — so this is the one place those are reconciled.
 */
function conditionsFor(
  biome: BiomeWeather,
  overrides: Weather["overrides"],
  pending: Partial<Record<WeatherSetting, number>>,
): SkyConditions {
  const forced = (asked: number | undefined, simulated: number) =>
    asked !== undefined && asked !== 0 ? asked : simulated;
  return {
    biome: biome.biome,
    state: biome.state,
    cloud: clamp01(forced(pending.Clouds ?? overrides.clouds, biome.clouds / 100)),
    rain: clamp01(forced(pending.Rain ?? overrides.rain, biome.rain)),
    snow: clamp01(forced(pending.SnowFall ?? overrides.snow, biome.snow)),
    fog: clamp01(forced(pending.Fog ?? overrides.fog, biome.fog / 4)),
    wind: clamp01(forced(pending.Wind ?? overrides.wind, biome.wind) / 200),
    temperature: forced(pending.Temp ?? overrides.temperature, biome.temperature),
  };
}

function WeatherTab({ serverId, onRun, onAsk }: { serverId: string; onRun: Run; onAsk: Ask }) {
  // The sky is lit by the world clock, so this tab needs the time too.
  const { data: dashboard } = useDashboard();
  const world = dashboard?.world;
  const moon = dashboard?.bloodMoon;

  const { data: weather, isLoading } = useQuery({
    queryKey: ["weather", serverId],
    queryFn: () => api.weather(serverId),
    enabled: serverId !== "",
    refetchInterval: 30_000,
    retry: 1,
  });

  const [biome, setBiome] = useState("");
  const [hours, setHours] = useState("2");

  /*
    What we have asked for but not yet seen come back.

    The clock taught this: without it, letting go of a bar sent the command and
    then the bar sprang back to the old reading until the next poll, so the one
    thing that proved the drag had worked arrived a second after the drag. Held
    here rather than inside the bar so that a preset — which is four commands
    at once — moves all four bars and the sky together.
  */
  const [pending, setPending] = useState<Partial<Record<WeatherSetting, number>>>({});

  useEffect(() => {
    if (biome === "" && weather && weather.biomes.length > 0) setBiome(weather.biomes[0].biome);
  }, [weather, biome]);

  // The sky can only show one biome at a time, and the storm command only
  // takes one, so the same choice serves both.
  const shownBiome = weather?.biomes.find((b) => b.biome === biome) ?? weather?.biomes[0];

  // Drop a held value once the world reports something close to it. Anything
  // still outstanding is cleared wholesale below, in case a command was
  // refused and the reading is never going to arrive.
  useEffect(() => {
    if (!weather || Object.keys(pending).length === 0) return;
    const settled = { ...pending };
    let changed = false;
    for (const knob of WEATHER) {
      const want = settled[knob.value];
      if (want === undefined) continue;
      const reported = weather?.overrides[knob.of];
      if (reported !== undefined && Math.abs(reported - want) <= (knob.max - knob.min) * 0.02) {
        delete settled[knob.value];
        changed = true;
      }
    }
    if (changed) setPending(settled);
  }, [weather, pending]);

  useEffect(() => {
    if (Object.keys(pending).length === 0) return;
    const giveUp = setTimeout(() => setPending({}), 6000);
    return () => clearTimeout(giveUp);
  }, [pending]);

  /** What a bar shows: what we asked for if it is still in flight. */
  function reading(knob: (typeof WEATHER)[number]): number {
    return pending[knob.value] ?? weather?.overrides[knob.of] ?? knob.min;
  }

  const sky = shownBiome && weather && conditionsFor(shownBiome, weather.overrides, pending);

  function force(setting: WeatherSetting, value: number, done: string) {
    setPending((p) => ({ ...p, [setting]: value }));
    onRun(done, () => api.setWeather(serverId, setting, value));
  }

  function applyPreset(label: string, knobs: [WeatherSetting, number][]) {
    setPending((p) => ({ ...p, ...Object.fromEntries(knobs) }));
    onRun(`Weather set to ${label.toLowerCase()}`, async () => {
      let last: ActionResult | undefined;
      const ran: string[] = [];
      for (const [name, v] of knobs) {
        last = await api.setWeather(serverId, name, v);
        ran.push(last.command);
      }
      // Reporting only the last one made the rain preset announce itself with
      // "weather Fog 0.3", which is the tail of the sequence and reads like the
      // wrong button was pressed. A preset is all of its commands.
      return { ...last!, command: ran.join(", ") };
    });
  }

  if (isLoading && !weather) {
    return (
      <div className="p-4 md:p-8">
        <Skeleton className="h-72 w-full rounded-none" />
      </div>
    );
  }

  if (!weather || weather.biomes.length === 0 || !sky) {
    return (
      <div className="p-4 md:p-8">
        <p className="max-w-prose text-xs text-bone-faint">
          The server did not report any conditions. There is no REST endpoint for weather; it comes
          back through the console, so this needs a working console connection.
        </p>
      </div>
    );
  }

  const biomeName = sky.biome.replace(/_/g, " ");

  return (
    <div className="p-4 md:p-8">
      <div className="flex flex-wrap items-start gap-x-10 gap-y-8">
        <div className="w-[480px] max-w-full shrink-0 space-y-3">
          <div className="relative border border-border">
            <Sky
              conditions={sky}
              hour={world ? world.hour + world.minute / 60 : 12}
              dawnHour={DAWN_HOUR}
              daylightHours={
                world?.daylightHours && world.daylightHours > 0
                  ? world.daylightHours
                  : DEFAULT_DAYLIGHT_HOURS
              }
              hordeTonight={moon?.active || moon?.nextDay === world?.day}
            />

            {/* Over the sky rather than under it: the name of what you are
                looking at belongs in the picture. */}
            <div className="pointer-events-none absolute inset-x-0 top-0 flex items-start justify-between p-3">
              <div>
                <p className="stencil text-bone-dim">{biomeName}</p>
                <p className="readout mt-1 text-xs text-bone-faint">
                  {sky.state === "default" ? "settled" : sky.state}
                </p>
              </div>
              <p className="figure text-3xl text-bone">{Math.round(sky.temperature)}°</p>
            </div>
          </div>

          {/* Every biome the world reports, with its own temperature, so the
              one that is storming is visible without opening a menu. */}
          <div className="flex flex-wrap gap-1">
            {weather.biomes.map((b) => (
              <button
                key={b.biome}
                type="button"
                onClick={() => setBiome(b.biome)}
                className={cn(
                  "flex items-baseline gap-2 border px-2.5 py-1.5 transition-colors",
                  b.biome === sky.biome
                    ? "border-border bg-accent"
                    : "border-transparent hover:bg-accent/50",
                )}
              >
                <span
                  className={cn("text-xs", b.biome === sky.biome ? "text-bone" : "text-bone-dim")}
                >
                  {b.biome.replace(/_/g, " ")}
                </span>
                <span className="readout text-xs text-bone-faint">
                  {Math.round(b.temperature)}°
                </span>
                {b.state !== "default" && (
                  <span className="readout text-xs text-ember">{b.state}</span>
                )}
              </button>
            ))}
          </div>
        </div>

        <div className="min-w-0 flex-1 space-y-8">
          <div className="space-y-2">
            <span className="stencil">Presets</span>
            <div className="flex flex-wrap gap-1">
              {WEATHER_PRESETS.map((preset) => (
                <Button
                  key={preset.label}
                  variant="ghost"
                  size="sm"
                  className="px-2.5 text-bone"
                  onClick={() => applyPreset(preset.label, preset.knobs)}
                >
                  {preset.label}
                </Button>
              ))}
              <Button
                variant="ghost"
                size="sm"
                className="px-2.5 text-bone-dim"
                onClick={() => {
                  setPending({});
                  onRun("Overrides cleared", () => api.resetWeather(serverId));
                }}
              >
                Hand it back
              </Button>
            </div>
          </div>

          <div className="space-y-5">
            <div>
              <span className="stencil">Force</span>
              <p className="mt-1.5 max-w-prose text-xs text-bone-faint">
                Drag to set — there is nothing to press afterwards. These are the values forced on
                every biome at once, which is not the same thing as what the world is doing: the
                scene shows that. The server cannot tell a cleared override from one set to zero, so
                a nought here means nobody is forcing it.
              </p>
            </div>

            <div className="grid max-w-3xl gap-x-10 gap-y-5 sm:grid-cols-2">
              {WEATHER.map((knob) => (
                <WeatherMeter
                  key={knob.value}
                  label={knob.label}
                  value={reading(knob)}
                  min={knob.min}
                  max={knob.max}
                  describe={knob.describe}
                  onCommit={(v) => {
                    const r = round(v, knob);
                    force(knob.value, r, `${knob.label} forced to ${knob.describe(r)}`);
                  }}
                />
              ))}
            </div>
          </div>

          <div className="space-y-2">
            <span className="stencil">Storm</span>
            <p className="max-w-prose text-xs text-bone-faint">
              The one thing here that is not global: a storm builds over a single biome, whichever
              one is selected beside the sky.
            </p>
            <div className="flex flex-wrap items-center gap-2 pt-1">
              <Label htmlFor="storm-hours" className="stencil">
                Hours
              </Label>
              <Input
                id="storm-hours"
                inputMode="numeric"
                className="h-8 w-16"
                value={hours}
                onChange={(e) => setHours(e.target.value)}
              />
              <Button
                variant="outline"
                className="h-8"
                onClick={() =>
                  onAsk(
                    "Start a storm?",
                    `${hours} in-game hours over ${biomeName}.`,
                    `Storm building over ${biomeName}`,
                    () => api.storm(serverId, sky.biome, Number(hours || 1)),
                  )
                }
              >
                <CloudLightning className="size-3.5" />
                Build over {biomeName}
              </Button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

/**
 * A dragged value, rounded to something a person would have typed.
 *
 * A bar can hand back 0.6173469, and sending that makes the console history
 * unreadable for no gain — the game does not resolve weather that finely.
 */
function round(value: number, knob: (typeof WEATHER)[number]): number {
  return knob.max > 1 ? Math.round(value) : Math.round(value * 20) / 20;
}

/* ----------------------------------------------------------------- spawn -- */

/**
 * The spawn catalogue is not a flat list.
 *
 * Two hundred and sixty classes come back, but a third of them are the same
 * creature again at a different difficulty: zombieArlene, zombieArleneFeral,
 * zombieArleneRadiated, zombieArleneCharged. Shown flat, four near-identical
 * rows sit next to each other and the name you actually want is buried. Split
 * apart, it is seventy creatures with a difficulty to pick, which is how
 * somebody thinks about it anyway.
 *
 * The game's own manualSpawnType is no help here — every spawnable class
 * reports "Menu" — so the grouping comes off the naming convention. The four
 * difficulty suffixes were counted off a live server rather than guessed at:
 * Feral and Radiated on thirty-three creatures each, Charged on twenty-four,
 * Infernal on twenty-three, and never two of them on the same name.
 */
const VARIANTS = ["Normal", "Feral", "Radiated", "Charged", "Infernal"] as const;
type Variant = (typeof VARIANTS)[number];

const KINDS = [
  { id: "zombie", label: "Zombies" },
  { id: "animal", label: "Animals" },
  { id: "vehicle", label: "Vehicles" },
  { id: "other", label: "Other" },
] as const;
type Kind = (typeof KINDS)[number]["id"];

interface Creature {
  /** The bare name, without a difficulty suffix. */
  base: string;
  kind: Kind;
  label: string;
  /** Which difficulties this one actually has, and the class each maps to. */
  forms: Partial<Record<Variant, string>>;
}

function kindOf(name: string): Kind {
  if (name.startsWith("zombie")) return "zombie";
  // Deliberately before the zombie test in effect: animalZombieBear is an
  // animal that happens to be undead, and belongs with the animals.
  if (name.startsWith("animal")) return "animal";
  if (name.startsWith("vehicle")) return "vehicle";
  return "other";
}

/** "animalMountainLion" reads as "Mountain Lion". */
function readable(base: string): string {
  const stripped = base.replace(/^(zombie|animal|vehicle|npc|junk)/, "");
  const spaced = stripped.replace(/[_]/g, " ").replace(/([a-z0-9])([A-Z])/g, "$1 $2");
  return spaced.trim() || base;
}

/** Folds the flat class list into creatures with difficulties. */
function toCreatures(classes: EntityClass[]): Creature[] {
  const byBase = new Map<string, Creature>();
  for (const entity of classes) {
    const suffix = VARIANTS.find((v) => v !== "Normal" && entity.name.endsWith(v));
    const base = suffix ? entity.name.slice(0, -suffix.length) : entity.name;
    const creature = byBase.get(base) ?? {
      base,
      kind: kindOf(base),
      label: readable(base),
      forms: {},
    };
    creature.forms[suffix ?? "Normal"] = entity.name;
    byBase.set(base, creature);
  }
  return [...byBase.values()].sort((a, b) => a.label.localeCompare(b.label));
}

function SpawnTab({ serverId, onRun, onAsk }: { serverId: string; onRun: Run; onAsk: Ask }) {
  const [query, setQuery] = useState("");
  const [kind, setKind] = useState<Kind>("zombie");
  const [base, setBase] = useState("");
  const [variant, setVariant] = useState<Variant>("Normal");
  const [count, setCount] = useState(1);
  const [target, setTarget] = useState("");

  const { data, isLoading } = useQuery({
    queryKey: ["entities", serverId],
    queryFn: () => api.spawnableEntities(serverId),
    enabled: serverId !== "",
    staleTime: 10 * 60 * 1000,
  });

  const { data: playerData } = usePlayers();
  const online = useMemo(
    () => (playerData?.players ?? []).filter((p) => p.online && p.position),
    [playerData],
  );

  const creatures = useMemo(() => toCreatures(data?.entities ?? []), [data]);
  const counts = useMemo(() => {
    const out: Record<string, number> = {};
    for (const c of creatures) out[c.kind] = (out[c.kind] ?? 0) + 1;
    return out;
  }, [creatures]);

  const needle = query.trim().toLowerCase();
  const shown = useMemo(
    () =>
      creatures.filter(
        (c) =>
          // A search looks across every kind, because somebody typing "bear"
          // does not know or care which bucket it was filed under.
          (needle ? c.kind !== "vehicle" : c.kind === kind) &&
          (needle === "" ||
            c.label.toLowerCase().includes(needle) ||
            c.base.toLowerCase().includes(needle)),
      ),
    [creatures, kind, needle],
  );

  useEffect(() => {
    if (target === "" && online.length > 0) setTarget(String(online[0].entityId));
  }, [online, target]);

  const at: Player | undefined = online.find((p) => String(p.entityId) === target);
  const picked = creatures.find((c) => c.base === base);

  // Moving to a creature that has no Charged form must not leave Charged
  // chosen: the spawn would quietly fall back to the plain one while the row
  // still showed the nastier pick greyed out and nothing else lit.
  useEffect(() => {
    if (picked && !picked.forms[variant]) setVariant("Normal");
  }, [picked, variant]);
  // A creature that has no Charged form must not be left showing Charged after
  // the selection moves to it.
  const form = picked?.forms[variant] ?? picked?.forms.Normal;
  const ready = Boolean(form && at);

  return (
    <div className="p-4 md:p-8">
      <div className="flex flex-wrap items-start gap-x-10 gap-y-8">
        <div className="w-[24rem] max-w-full shrink-0 space-y-3">
          <div className="flex items-baseline justify-between">
            <span className="stencil">What</span>
            <span className="readout text-xs text-bone-faint">
              {shown.length} of {creatures.length}
            </span>
          </div>

          <Input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search every kind"
            className="h-8"
          />

          {!needle && (
            <div className="flex flex-wrap gap-1">
              {KINDS.map((k) => (
                <button
                  key={k.id}
                  type="button"
                  onClick={() => setKind(k.id)}
                  className={cn(
                    "flex items-baseline gap-1.5 border px-2.5 py-1 text-xs transition-colors",
                    k.id === kind
                      ? "border-border bg-accent text-bone"
                      : "border-transparent text-bone-dim hover:bg-accent/50",
                  )}
                >
                  {k.label}
                  <span className="readout text-2xs text-bone-faint">{counts[k.id] ?? 0}</span>
                </button>
              ))}
            </div>
          )}

          {isLoading ? (
            <Skeleton className="h-80 w-full rounded-none" />
          ) : (
            <ul className="max-h-[22rem] overflow-y-auto border border-border">
              {shown.length === 0 ? (
                <li className="p-3 text-xs text-bone-faint">Nothing matches.</li>
              ) : (
                shown.map((c) => (
                  <li key={c.base}>
                    <button
                      type="button"
                      onClick={() => setBase(c.base)}
                      className={cn(
                        "flex w-full items-baseline justify-between gap-3 border-b border-border/60 px-3 py-1.5 text-left last:border-b-0 transition-colors",
                        c.base === base ? "bg-accent" : "hover:bg-accent/50",
                      )}
                    >
                      <span
                        className={cn("text-xs", c.base === base ? "text-bone" : "text-bone-dim")}
                      >
                        {c.label}
                      </span>
                      {/* How many difficulties it comes in, which is the one
                          thing worth knowing before clicking it. */}
                      <span className="readout shrink-0 text-2xs text-bone-faint">
                        {Object.keys(c.forms).length > 1
                          ? `${Object.keys(c.forms).length} forms`
                          : ""}
                      </span>
                    </button>
                  </li>
                ))
              )}
            </ul>
          )}
        </div>

        <div className="min-w-0 flex-1 space-y-8">
          <div className="space-y-2">
            <span className="stencil">Arrives on its own</span>
            <p className="max-w-prose text-xs text-bone-faint">
              Neither of these needs a target. The game decides where they land.
            </p>
            <div className="flex flex-wrap gap-1 pt-1">
              <Button
                variant="ghost"
                size="sm"
                className="gap-1.5 px-2.5 text-bone"
                onClick={() =>
                  onAsk(
                    "Send a wandering horde?",
                    "A roaming group will head for the players.",
                    "Wandering horde on its way",
                    () => api.wanderingHorde(serverId),
                  )
                }
              >
                <Users className="size-3.5" />
                Wandering horde
              </Button>
              <Button
                variant="ghost"
                size="sm"
                className="gap-1.5 px-2.5 text-bone"
                onClick={() => onRun("Air drop inbound", () => api.airDrop(serverId))}
              >
                <PackageOpen className="size-3.5" />
                Air drop
              </Button>
            </div>
          </div>

          <div className="space-y-2">
            <span className="stencil">Screamers</span>
            {/*
              Not in the row above, because unlike those two this one has to
              have somebody to arrive at: the bare command is documented as
              usable only by an issuing player, never a remote console.
            */}
            <p className="max-w-prose text-xs text-bone-faint">
              Scouts that shriek and call a horde down on whoever they find.
            </p>
            <Button
              variant="ghost"
              size="sm"
              disabled={!at}
              className="mt-1 gap-1.5 px-2.5 text-bone"
              onClick={() =>
                onAsk(
                  "Send screamers?",
                  `They will arrive on ${at!.name} and start calling.`,
                  `Screamers sent to ${at!.name}`,
                  () => api.spawnScouts(serverId, at!.entityId),
                )
              }
            >
              <Siren className="size-3.5" />
              {at ? `Screamers on ${at.name}` : "Nobody to send them to"}
            </Button>
          </div>

          <div className="space-y-3">
            <span className="stencil">Where</span>
            {/*
              Spawning happens at a player, not at coordinates typed by hand.
              The old default was 0, -1, 0, which on most worlds is nowhere near
              anybody — and entities only persist in chunks a player has loaded,
              so that spawn did nothing and gave no clue why.
            */}
            {online.length === 0 ? (
              <p className="max-w-prose text-xs text-bone-faint">
                Nobody is online, so there is no loaded chunk to spawn into. Anything sent now would
                vanish without saying why.
              </p>
            ) : (
              <div className="flex flex-wrap gap-1">
                {online.map((p) => (
                  <button
                    key={p.entityId}
                    type="button"
                    onClick={() => setTarget(String(p.entityId))}
                    className={cn(
                      "flex items-baseline gap-2 border px-2.5 py-1.5 transition-colors",
                      String(p.entityId) === target
                        ? "border-border bg-accent"
                        : "border-transparent hover:bg-accent/50",
                    )}
                  >
                    <span
                      className={cn(
                        "text-xs",
                        String(p.entityId) === target ? "text-bone" : "text-bone-dim",
                      )}
                    >
                      {p.name}
                    </span>
                    <span className="readout text-2xs text-bone-faint">
                      {Math.round(p.position!.x)}, {Math.round(p.position!.z)}
                    </span>
                  </button>
                ))}
              </div>
            )}
          </div>

          <div className="space-y-3">
            <span className="stencil">How</span>
            <div className="flex flex-wrap items-center gap-6">
              <div className="flex flex-wrap gap-1">
                {VARIANTS.map((v) => {
                  const available = Boolean(picked?.forms[v]);
                  return (
                    <button
                      key={v}
                      type="button"
                      disabled={!available}
                      onClick={() => setVariant(v)}
                      className={cn(
                        "border px-2.5 py-1 text-xs transition-colors",
                        !available
                          ? "border-transparent text-bone-faint/40"
                          : v === variant
                            ? "border-border bg-accent text-bone"
                            : "border-transparent text-bone-dim hover:bg-accent/50",
                        v !== "Normal" && available && "text-ember",
                      )}
                    >
                      {v}
                    </button>
                  );
                })}
              </div>

              <div className="flex items-center gap-1">
                {[1, 3, 5, 10, 25].map((n) => (
                  <button
                    key={n}
                    type="button"
                    onClick={() => setCount(n)}
                    className={cn(
                      "readout border px-2.5 py-1 text-xs transition-colors",
                      n === count
                        ? "border-border bg-accent text-bone"
                        : "border-transparent text-bone-dim hover:bg-accent/50",
                    )}
                  >
                    {n}
                  </button>
                ))}
              </div>
            </div>
          </div>

          {/*
            The whole order in one sentence, above the button that sends it.
            A spawn is not undoable, and the previous layout put the count, the
            target and the class in three separate boxes with nothing stating
            what pressing the button would actually do.
          */}
          <div className="space-y-3 border-t border-border pt-6">
            <p className={cn("text-sm", ready ? "text-bone" : "text-bone-faint")}>
              {ready ? (
                <>
                  {count} × <span className="readout text-ember">{form}</span> next to{" "}
                  <span className="text-bone">{at!.name}</span>
                </>
              ) : !at ? (
                "Pick somebody to spawn next to."
              ) : (
                "Pick something to spawn."
              )}
            </p>
            <Button
              disabled={!ready}
              onClick={() =>
                onAsk(
                  "Spawn these?",
                  `${count} × ${form} next to ${at!.name}. This cannot be undone.`,
                  `${count} × ${form} spawned on ${at!.name}`,
                  () =>
                    api.spawn(
                      serverId,
                      form!,
                      Math.round(at!.position!.x),
                      Math.round(at!.position!.y),
                      Math.round(at!.position!.z),
                      count,
                    ),
                )
              }
            >
              Spawn
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}

/* ---------------------------------------------------------------- upkeep -- */

/**
 * The jobs that are about the world itself rather than anything in it.
 *
 * They have their own tab because they do not belong to a time, a sky or a
 * spawn: saving before a restart, clearing a horde that has wedged itself in
 * geometry, and resetting chunks so loot and resources come back. The last of
 * those permanently discards saved data, so it is kept well away from the
 * other two and behind the same type-to-confirm the console uses.
 */
function UpkeepTab({ serverId, onRun, onAsk }: { serverId: string; onRun: Run; onAsk: Ask }) {
  const KILL_SCOPES = [
    { scope: "" as const, label: "Hostiles", note: "zombies and other enemies" },
    { scope: "alive" as const, label: "Everything alive", note: "adds animals, keeps vehicles" },
    { scope: "all" as const, label: "Every entity", note: "vehicles and turrets too" },
  ];

  return (
    <div className="max-w-3xl space-y-10 p-4 md:p-8">
      <div className="space-y-2">
        <span className="stencil">Save</span>
        <p className="max-w-prose text-xs text-bone-faint">
          Everything else in this panel takes effect immediately but lives in memory. This is what
          makes it survive a restart.
        </p>
        <Button
          variant="outline"
          className="mt-1 gap-1.5"
          onClick={() => onRun("World saved", () => api.saveWorld(serverId))}
        >
          <HardDriveDownload className="size-3.5" />
          Save the world
        </Button>
      </div>

      <div className="space-y-2">
        <span className="stencil">Clear the world</span>
        <p className="max-w-prose text-xs text-bone-faint">
          For a horde that has wedged itself into geometry, or a screamer that will not stop. The
          game never kills players with this, whichever reach is chosen.
        </p>
        <div className="flex flex-wrap gap-1 pt-1">
          {KILL_SCOPES.map((s) => (
            <Button
              key={s.label}
              variant="ghost"
              size="sm"
              className="gap-1.5 px-2.5"
              onClick={() =>
                onAsk(
                  `Kill ${s.label.toLowerCase()}?`,
                  `Removes ${s.note}. Players are never affected.`,
                  `Cleared ${s.label.toLowerCase()}`,
                  () => api.killAll(serverId, s.scope),
                )
              }
            >
              <Skull className="size-3.5 text-bone-dim" />
              <span className="text-bone">{s.label}</span>
              <span className="text-xs text-bone-faint">{s.note}</span>
            </Button>
          ))}
        </div>
      </div>

      <div className="space-y-2 border-t border-crimson-deep pt-8">
        <span className="stencil text-crimson-lit">Reset chunks</span>
        <p className="max-w-prose text-xs text-bone-faint">
          Puts loot, resources and terrain back as they were generated, across every chunk nobody
          has claimed. It permanently deletes the saved data for those chunks — anything built on
          unclaimed ground goes with them, and there is no undo.
        </p>
        <p className="max-w-prose text-xs text-bone-faint">
          Land claims and the ground around online players are respected. The panel deliberately
          does not offer the modes that override that: the game documents them as experimental and
          able to ignore claims, which is a way to delete somebody&apos;s base without warning.
        </p>
        <Button
          variant="outline"
          className="mt-1 gap-1.5 border-crimson text-crimson-lit hover:bg-crimson-deep/30"
          onClick={() =>
            onAsk(
              "Reset every unclaimed chunk?",
              "Loot and resources come back. Anything built on unclaimed ground is deleted, permanently. Claimed land is left alone.",
              "Unclaimed chunks reset",
              () => api.resetChunks(serverId),
            )
          }
        >
          <Trash2 className="size-3.5" />
          Reset unclaimed chunks
        </Button>
      </div>
    </div>
  );
}
