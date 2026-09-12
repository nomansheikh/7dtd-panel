import type { ReactNode } from "react";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { AppShell } from "@/components/app-shell";
import { DashboardPage } from "@/pages/dashboard";
import { ConsolePage } from "@/pages/console";
import { EventsPage } from "@/pages/events";
import { PlayerPage } from "@/pages/player";
import { PlayersPage } from "@/pages/players";
import { WorldPage } from "@/pages/world";
import { ChatPage } from "@/pages/chat";
import { AutomationPage } from "@/pages/automation";
import { SettingsPage } from "@/pages/settings";
import { SetupPage } from "@/pages/setup";
import { ServersPage } from "@/pages/servers";
import { LoginPage } from "@/pages/login";
import { useAuth } from "@/hooks/use-auth";
import { useServers } from "@/hooks/use-servers";

export default function App() {
  const { user, loading } = useAuth();

  // Rendering the sign-in form before the session check finishes would make a
  // reload flash the login page at someone who is already signed in.
  if (loading) {
    return <div className="min-h-dvh" aria-busy="true" />;
  }

  if (!user) {
    return <LoginPage />;
  }

  return (
    <BrowserRouter>
      <AppShell>
        <RequireServer>
          <Routes>
            <Route path="/" element={<DashboardPage />} />
            <Route path="/players" element={<PlayersPage />} />
            <Route path="/players/:platformId" element={<PlayerPage />} />
            <Route path="/console" element={<ConsolePage />} />
            <Route path="/events" element={<EventsPage />} />
            <Route path="/world" element={<WorldPage />} />
            <Route path="/chat" element={<ChatPage />} />
            <Route path="/automation" element={<AutomationPage />} />
            <Route path="/settings" element={<SettingsPage />} />
            <Route path="/servers" element={<ServersPage />} />
            {/* Unknown paths go home rather than showing nothing. */}
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </RequireServer>
      </AppShell>
    </BrowserRouter>
  );
}

/**
 * Holds page content back until a game server has been chosen.
 *
 * Every page is scoped to a server, and the list of servers arrives over the
 * network. Rendering before it does meant each page briefly decided it had no
 * data and showed a hard error, which looked like a failure rather than a
 * first paint.
 *
 * A panel with no servers at all is a different case, and used to be an
 * impossible one: the process would not start without them. Now it is how
 * every install begins, so it gets the setup page rather than a spinner that
 * never resolves — which is what the old condition gave it, since "no current
 * server and no servers" stayed true forever once loading finished.
 */
function RequireServer({ children }: { children: ReactNode }) {
  const { currentId, servers, loading } = useServers();

  if (loading) {
    return (
      <div className="py-12 text-sm text-muted-foreground" aria-busy="true">
        Loading servers…
      </div>
    );
  }

  if (servers.length === 0) {
    return <SetupPage />;
  }

  if (!currentId) {
    return (
      <div className="py-12 text-sm text-muted-foreground" aria-busy="true">
        Choosing a server…
      </div>
    );
  }

  return <>{children}</>;
}
