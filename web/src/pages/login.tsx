import { useState, type FormEvent } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useAuth } from "@/hooks/use-auth";
import { ApiError } from "@/lib/api";
import { BLOOD_MOON_CYCLE } from "@/lib/format";

export function LoginPage() {
  const { signIn, signingIn } = useAuth();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setError(null);
    try {
      await signIn(username, password);
    } catch (cause) {
      // Show what the panel actually said, never a generic failure.
      setError(cause instanceof ApiError ? cause.message : "Sign in failed for an unknown reason.");
    }
  }

  return (
    <main className="flex min-h-dvh items-center justify-center p-6">
      <div className="w-full max-w-sm">
        {/* The cycle motif, carried over from the dashboard. */}
        <div className="flex gap-1" aria-hidden>
          {Array.from({ length: BLOOD_MOON_CYCLE }, (_, i) => (
            <span
              key={i}
              className={
                i === BLOOD_MOON_CYCLE - 1
                  ? "h-1 flex-1 rounded-full bg-blood"
                  : "h-1 flex-1 rounded-full bg-muted"
              }
            />
          ))}
        </div>

        <h1 className="mt-6 text-2xl font-semibold">7 Days to Die admin</h1>
        <p className="mt-1 text-sm text-muted-foreground">Sign in to manage your server.</p>

        <form onSubmit={handleSubmit} className="mt-8 space-y-4" noValidate>
          <div className="space-y-2">
            <Label htmlFor="username">Username</Label>
            <Input
              id="username"
              name="username"
              autoComplete="username"
              autoFocus
              required
              value={username}
              onChange={(e) => setUsername(e.target.value)}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="password">Password</Label>
            <Input
              id="password"
              name="password"
              type="password"
              autoComplete="current-password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </div>

          {error && (
            <p role="alert" className="text-sm text-destructive">
              {error}
            </p>
          )}

          <Button type="submit" className="w-full" disabled={signingIn}>
            {signingIn ? "Signing in…" : "Sign in"}
          </Button>
        </form>

        <p className="mt-6 text-xs text-muted-foreground">
          The admin account comes from PANEL_ADMIN_PASSWORD. Change that variable and restart to
          reset it.
        </p>
      </div>
    </main>
  );
}
