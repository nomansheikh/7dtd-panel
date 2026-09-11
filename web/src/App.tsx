import { AppShell } from "@/components/app-shell";
import { DashboardPage } from "@/pages/dashboard";
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
    <AppShell>
      <DashboardPage />
    </AppShell>
  );
}
