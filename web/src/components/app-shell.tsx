import type { ReactNode } from "react";
import { NavLink } from "react-router-dom";
import { LogOut } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ConnectionStatus } from "@/components/connection-status";
import { ServerSwitcher } from "@/components/server-switcher";
import { useAuth } from "@/hooks/use-auth";
import { useDashboard } from "@/hooks/use-dashboard";
import { cn } from "@/lib/utils";

const NAV = [
  { to: "/", label: "Dashboard" },
  { to: "/console", label: "Console" },
  { to: "/world", label: "World" },
  { to: "/events", label: "Events" },
];

export function AppShell({ children }: { children: ReactNode }) {
  const { user, signOut } = useAuth();
  const { data } = useDashboard();

  return (
    <div className="min-h-dvh">
      <header className="border-b border-border">
        <div className="mx-auto flex max-w-6xl flex-wrap items-center gap-x-6 gap-y-2 px-6 py-3">
          <span className="font-semibold">7 Days to Die admin</span>

          <ServerSwitcher />

          <nav className="flex gap-1">
            {NAV.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.to === "/"}
                className={({ isActive }) =>
                  cn(
                    "rounded-md px-3 py-1.5 text-sm",
                    isActive
                      ? "bg-accent text-accent-foreground"
                      : "text-muted-foreground hover:text-foreground",
                  )
                }
              >
                {item.label}
              </NavLink>
            ))}
          </nav>

          {data && (
            <ConnectionStatus
              status={data.status}
              ageSeconds={data.ageSeconds}
              stale={data.stale}
            />
          )}

          <div className="ml-auto flex items-center gap-3">
            {user && <span className="text-sm text-muted-foreground">{user.username}</span>}
            <Button variant="ghost" size="sm" onClick={() => void signOut()}>
              <LogOut />
              Sign out
            </Button>
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-6xl px-6 py-8">{children}</main>
    </div>
  );
}
