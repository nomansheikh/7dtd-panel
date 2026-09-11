import type { ReactNode } from "react";
import { NavLink, useLocation } from "react-router-dom";
import { Gauge, LogOut, Radio, ScrollText, SlidersHorizontal, Terminal, Users } from "lucide-react";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
  SidebarRail,
  SidebarTrigger,
} from "@/components/ui/sidebar";
import { Separator } from "@/components/ui/separator";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Atmosphere } from "@/components/atmosphere";
import { ConnectionStatus } from "@/components/connection-status";
import { HordeMeter } from "@/components/horde-meter";
import { ServerSwitcher } from "@/components/server-switcher";
import { useAuth } from "@/hooks/use-auth";
import { useBloodMoon } from "@/hooks/use-blood-moon";
import { useDashboard } from "@/hooks/use-dashboard";
import { bloodMoonProgress, formatGameClock } from "@/lib/format";

const NAV = [
  { to: "/", label: "Overview", icon: Gauge },
  { to: "/players", label: "Players", icon: Users },
  { to: "/world", label: "World", icon: Radio },
  { to: "/console", label: "Console", icon: Terminal },
  { to: "/events", label: "Events", icon: ScrollText },
  { to: "/settings", label: "Settings", icon: SlidersHorizontal },
];

const TITLES: Record<string, string> = {
  "/": "Overview",
  "/players": "Players",
  "/world": "World",
  "/console": "Console",
  "/events": "Events",
  "/settings": "Settings",
};

export function AppShell({ children }: { children: ReactNode }) {
  // Publishes --moon, which every atmospheric layer in the panel reads from.
  useBloodMoon();

  return (
    <SidebarProvider>
      <Atmosphere />
      <PanelSidebar />
      <SidebarInset className="bg-transparent">
        <TopBar />
        <main className="mx-auto w-full max-w-7xl flex-1 px-6 py-8 md:px-10">{children}</main>
      </SidebarInset>
    </SidebarProvider>
  );
}

function PanelSidebar() {
  const { user, signOut } = useAuth();
  const { data } = useDashboard();

  return (
    <Sidebar collapsible="icon" className="border-r">
      <SidebarHeader className="gap-3 p-3">
        <Wordmark />
        <ServerSwitcher />
      </SidebarHeader>

      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel className="stencil">Manage</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {NAV.map((item) => (
                <SidebarMenuItem key={item.to}>
                  <NavLink to={item.to} end={item.to === "/"}>
                    {({ isActive }) => (
                      <SidebarMenuButton
                        isActive={isActive}
                        tooltip={item.label}
                        // The active item is marked by a crimson edge rather
                        // than a filled block, so the nav never competes with
                        // the horde meter directly below it.
                        className="relative data-[active=true]:bg-sidebar-accent data-[active=true]:before:absolute data-[active=true]:before:inset-y-1 data-[active=true]:before:-left-3 data-[active=true]:before:w-0.5 data-[active=true]:before:bg-crimson-lit"
                      >
                        <item.icon />
                        <span>{item.label}</span>
                      </SidebarMenuButton>
                    )}
                  </NavLink>
                </SidebarMenuItem>
              ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>

      <SidebarFooter className="gap-3 p-3">
        <SidebarCountdown />
        <Separator className="group-data-[collapsible=icon]:hidden" />
        <div className="flex items-center justify-between gap-2 group-data-[collapsible=icon]:flex-col">
          <span className="truncate text-xs text-bone-faint group-data-[collapsible=icon]:hidden">
            {user?.username}
          </span>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="size-7 text-bone-faint hover:text-foreground"
                onClick={() => void signOut()}
                aria-label="Sign out"
              >
                <LogOut className="size-3.5" />
              </Button>
            </TooltipTrigger>
            <TooltipContent side="right">Sign out</TooltipContent>
          </Tooltip>
        </div>
        {data && (
          <div className="group-data-[collapsible=icon]:hidden">
            <ConnectionStatus
              status={data.status}
              ageSeconds={data.ageSeconds}
              stale={data.stale}
            />
          </div>
        )}
      </SidebarFooter>

      <SidebarRail />
    </Sidebar>
  );
}

/**
 * The wordmark.
 *
 * Seven tallies, the last one struck through, which is both the game's title
 * and the thing the whole panel is counting. Collapses to the tallies alone.
 */
function Wordmark() {
  return (
    <div className="flex items-center gap-2.5 px-1">
      <span aria-hidden className="flex h-6 items-end gap-[2px]">
        {Array.from({ length: 7 }, (_, i) => (
          <span
            key={i}
            className={i === 6 ? "w-[2px] bg-crimson-lit" : "w-[2px] bg-bone-faint"}
            style={{ height: `${45 + i * 9}%` }}
          />
        ))}
      </span>
      <span className="font-display text-base leading-none font-bold tracking-[0.18em] uppercase group-data-[collapsible=icon]:hidden">
        7 Days
      </span>
    </div>
  );
}

/** The horde countdown, small enough to live under the nav on every page. */
function SidebarCountdown() {
  const { data } = useDashboard();
  if (!data?.bloodMoon) return null;

  const { world, bloodMoon } = data;
  const { daysAway } = bloodMoonProgress(world.day, bloodMoon.nextDay);

  return (
    <div className="space-y-2 group-data-[collapsible=icon]:hidden">
      <div className="flex items-baseline justify-between">
        <span className="stencil">Blood moon</span>
        <span className={cnCountdown(bloodMoon.active, daysAway)}>
          {bloodMoon.active ? "TONIGHT" : `${daysAway}d`}
        </span>
      </div>
      <HordeMeter
        size="compact"
        currentDay={world.day}
        nextDay={bloodMoon.nextDay}
        active={bloodMoon.active}
      />
    </div>
  );
}

function cnCountdown(active: boolean, daysAway: number) {
  const base = "font-display text-sm leading-none font-bold tracking-wider tabular-nums";
  if (active) return `${base} animate-breathe text-crimson-lit`;
  if (daysAway <= 1) return `${base} text-crimson-lit`;
  return `${base} text-bone-dim`;
}

/**
 * The bar above the page content: the in-game clock, which is the one number
 * that belongs on every screen, and the page name.
 */
function TopBar() {
  const { pathname } = useLocation();
  const { data } = useDashboard();

  return (
    <header className="sticky top-0 z-30 flex h-14 shrink-0 items-center gap-3 border-b border-border bg-background/80 px-6 backdrop-blur-md md:px-10">
      <SidebarTrigger className="-ml-2 text-bone-faint" />
      <h1 className="font-display text-lg leading-none font-bold tracking-[0.12em] uppercase">
        {TITLES[pathname] ?? "Panel"}
      </h1>

      {data && (
        <div className="ml-auto flex items-baseline gap-3">
          <span className="stencil hidden sm:inline">{data.world.name}</span>
          <span className="font-display text-lg leading-none font-bold tracking-wider">
            DAY {data.world.day}
          </span>
          <span className="readout text-lg leading-none text-bone-dim">
            {formatGameClock(data.world.hour, data.world.minute)}
          </span>
        </div>
      )}
    </header>
  );
}
