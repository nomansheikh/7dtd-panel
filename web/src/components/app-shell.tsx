import type { ReactNode } from "react";
import { LogOut } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ConnectionStatus } from "@/components/connection-status";
import { useAuth } from "@/hooks/use-auth";
import { useDashboard } from "@/hooks/use-dashboard";

export function AppShell({ children }: { children: ReactNode }) {
  const { user, signOut } = useAuth();
  const { data } = useDashboard();

  return (
    <div className="min-h-dvh">
      {/*
        A thin status rail rather than a sidebar. With one page there is nothing
        to navigate between, and the connection state is the thing worth keeping
        permanently in view.
      */}
      <header className="border-b border-border">
        <div className="mx-auto flex max-w-5xl flex-wrap items-center gap-x-6 gap-y-2 px-6 py-3">
          <span className="font-semibold">7 Days to Die admin</span>

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

      <main className="mx-auto max-w-5xl px-6 py-8">{children}</main>
    </div>
  );
}
