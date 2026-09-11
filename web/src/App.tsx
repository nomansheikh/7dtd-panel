import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { AppShell } from "@/components/app-shell";
import { DashboardPage } from "@/pages/dashboard";
import { ConsolePage } from "@/pages/console";
import { EventsPage } from "@/pages/events";
import { WorldPage } from "@/pages/world";
import { LoginPage } from "@/pages/login";
import { useAuth } from "@/hooks/use-auth";

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
        <Routes>
          <Route path="/" element={<DashboardPage />} />
          <Route path="/console" element={<ConsolePage />} />
          <Route path="/events" element={<EventsPage />} />
          <Route path="/world" element={<WorldPage />} />
          {/* Unknown paths go home rather than showing nothing. */}
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </AppShell>
    </BrowserRouter>
  );
}
