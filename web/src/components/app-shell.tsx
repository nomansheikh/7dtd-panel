import type { ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { NavLink, useLocation } from "react-router-dom";
import {
  Clock,
  Gauge,
  LogOut,
  MessageSquare,
  Monitor,
  Moon,
  Radio,
  ScrollText,
  SlidersHorizontal,
  Sun,
  Terminal,
  Users,
} from "lucide-react";
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
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Atmosphere } from "@/components/atmosphere";
import { ConnectionStatus } from "@/components/connection-status";
import { CycleStrip } from "@/components/cycle-strip";
import { ServerSwitcher } from "@/components/server-switcher";
import { useAuth } from "@/hooks/use-auth";
import { useBloodMoon } from "@/hooks/use-blood-moon";
import { useDashboard } from "@/hooks/use-dashboard";
import { useTheme, type Theme } from "@/hooks/use-theme";
import { api } from "@/lib/api";

const NAV = [
  { to: "/", label: "Overview", icon: Gauge },
  { to: "/players", label: "Players", icon: Users },
  { to: "/world", label: "World", icon: Radio },
  { to: "/chat", label: "Chat", icon: MessageSquare },
  { to: "/automation", label: "Automation", icon: Clock },
  { to: "/console", label: "Console", icon: Terminal },
  { to: "/events", label: "Events", icon: ScrollText },
  { to: "/settings", label: "Settings", icon: SlidersHorizontal },
];

/**
 * What the bar says.
 *
 * An exact lookup alone left every nested route titled "Panel", which is the
 * one word that tells a reader nothing. A path under a section is still that
 * section.
 */
function title(pathname: string): string {
  if (TITLES[pathname]) return TITLES[pathname];
  const section = Object.keys(TITLES).find(
    (path) => path !== "/" && pathname.startsWith(path + "/"),
  );
  return section ? TITLES[section] : "Panel";
}

const TITLES: Record<string, string> = {
  "/": "Overview",
  "/players": "Players",
  "/world": "World",
  "/console": "Console",
  "/events": "Events",
  "/settings": "Settings",
};

/**
 * The shell.
 *
 * The work area is a fixed-height column — top bar, cycle strip, then the page
 * — and the page fills what is left rather than stacking down a scrolling
 * document. An admin console that leaves the bottom third of a 1440px screen
 * empty is wasting the only thing it has.
 *
 * It is capped in width for the opposite reason. Every page here is a deck of
 * dense rows, and a row stretched across an ultrawide puts its label at one
 * end and its value at the other with a foot of nothing between them, which is
 * unreadable however much screen it fills.
 */
export function AppShell({ children }: { children: ReactNode }) {
  // Publishes --moon, which every atmospheric layer in the panel reads from.
  useBloodMoon();

  return (
    <SidebarProvider
      /*
        The gutter the whole shell is inset by once the viewport is wider than
        it needs. Published as a variable because the sidebar is positioned
        against the viewport rather than against this container, so it has to
        be pushed by the same amount by hand or it detaches from the column it
        belongs to.
      */
      style={
        {
          "--shell-max": "112rem",
          "--shell-gutter": "max(0px, (100vw - var(--shell-max)) / 2)",
        } as React.CSSProperties
      }
      className="mx-auto max-w-(--shell-max)"
    >
      <Atmosphere />
      <PanelSidebar />
      <SidebarInset className="h-dvh min-w-0 overflow-hidden bg-transparent">
        {/*
          Capped and centred, with the bar and the strip inside the cap rather
          than spanning past it.

          Constraining only the page would have left the top bar running the
          full width of a 34-inch monitor with the content floating in the
          middle of it, which reads as a bug.

          The cap lives on the provider above so that the sidebar sits inside
          it too, and the whole shell centres as one piece.
        */}
        <div className="flex h-full w-full min-w-0 flex-col overflow-hidden">
          <TopBar />
          <CycleStrip />
          <main className="min-h-0 flex-1 overflow-hidden">{children}</main>
        </div>
      </SidebarInset>
    </SidebarProvider>
  );
}

function PanelSidebar() {
  const { user, signOut } = useAuth();
  const { data } = useDashboard();
  // The panel's own build, not the game's. Served by its liveness endpoint,
  // which answers whether or not a game server is reachable.
  const { data: health } = useQuery({
    queryKey: ["panel-health"],
    queryFn: () => api.panelHealth(),
    staleTime: Infinity,
  });

  return (
    <Sidebar collapsible="icon" className="left-(--shell-gutter) border-r">
      <SidebarHeader className="gap-4 p-3">
        {/* On the nav rather than in the top bar: a control that collapses the
            sidebar reads as part of the page it is sitting on otherwise, and
            when the sidebar is closed the reopen lands nowhere near it. */}
        <div className="flex items-center gap-2 group-data-[collapsible=icon]:justify-center">
          <span className="min-w-0 group-data-[collapsible=icon]:hidden">
            <Wordmark />
          </span>
          <SidebarTrigger className="ml-auto shrink-0 text-bone-faint group-data-[collapsible=icon]:ml-0" />
        </div>
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
                        // Marked by a crimson edge rather than a filled block,
                        // so the nav never competes with the cycle strip.
                        className="relative data-[active=true]:bg-sidebar-accent data-[active=true]:before:absolute data-[active=true]:before:inset-y-0 data-[active=true]:before:-left-3 data-[active=true]:before:w-0.5 data-[active=true]:before:bg-crimson-lit"
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

      <SidebarFooter className="gap-2 border-t border-sidebar-border p-3">
        {data && (
          <div className="group-data-[collapsible=icon]:hidden">
            <ConnectionStatus
              status={data.status}
              ageSeconds={data.ageSeconds}
              stale={data.stale}
            />
          </div>
        )}
        {/* What this is and where it came from. An open-source panel that
            never says which build it is leaves every bug report guessing. */}
        <div className="flex items-center gap-1 group-data-[collapsible=icon]:flex-col">
          <ThemeToggle />
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                asChild
                variant="ghost"
                size="icon"
                className="size-7 text-bone-faint hover:text-foreground"
              >
                <a href={REPO} target="_blank" rel="noreferrer" aria-label="Source on GitHub">
                  <GithubMark className="size-3.5" />
                </a>
              </Button>
            </TooltipTrigger>
            <TooltipContent side="right">Source on GitHub</TooltipContent>
          </Tooltip>
          <span className="readout ml-auto text-xs text-bone-faint group-data-[collapsible=icon]:hidden">
            {health?.version ?? ""}
          </span>
        </div>

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
      </SidebarFooter>

      <SidebarRail />
    </Sidebar>
  );
}

/**
 * The wordmark: seven tallies, the last one lit. Both the game's title and the
 * thing the whole panel is counting.
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

/** Page name on the left, world identity on the right. */
function TopBar() {
  const { pathname } = useLocation();
  const { data } = useDashboard();

  return (
    <header className="flex h-12 shrink-0 items-center gap-3 border-b border-border px-4 md:px-6">
      <h1 className="font-display text-base leading-none font-bold tracking-[0.14em] uppercase">
        {title(pathname)}
      </h1>

      {data && (
        <div className="ml-auto flex items-baseline gap-3">
          <span className="stencil hidden sm:inline">{data.world.name}</span>
          <span className="font-display text-base leading-none font-bold tracking-wider">
            DAY {data.world.day}
          </span>
        </div>
      )}
    </header>
  );
}

/** Where this came from. */
const REPO = "https://github.com/nomansheikh/7dtd-panel";

/**
 * The GitHub mark, drawn here rather than imported.
 *
 * lucide dropped its brand icons, and pulling a whole icon package in for one
 * glyph on a panel that ships as a single binary is not a trade worth making.
 */
function GithubMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 16 16" fill="currentColor" aria-hidden className={className}>
      <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27s1.36.09 2 .27c1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8Z" />
    </svg>
  );
}

/**
 * Dark, light, or whatever the machine says.
 *
 * One button that cycles rather than a menu: there are three states, it lives
 * in a rail that collapses to forty-eight pixels, and the current one is
 * legible from the icon without opening anything.
 */
function ThemeToggle() {
  const { theme, setTheme } = useTheme();
  const next: Record<Theme, Theme> = { dark: "light", light: "system", system: "dark" };
  const Icon = theme === "dark" ? Moon : theme === "light" ? Sun : Monitor;

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          className="size-7 text-bone-faint hover:text-foreground"
          onClick={() => setTheme(next[theme])}
          aria-label={`Theme: ${theme}. Switch to ${next[theme]}.`}
        >
          <Icon className="size-3.5" />
        </Button>
      </TooltipTrigger>
      <TooltipContent side="right" className="capitalize">
        {theme}
      </TooltipContent>
    </Tooltip>
  );
}
