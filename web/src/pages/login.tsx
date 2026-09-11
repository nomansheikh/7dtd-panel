import { useState, type FormEvent } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Atmosphere } from "@/components/atmosphere";
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
    <main className="relative flex min-h-dvh items-center justify-center p-6">
      {/*
        Signed out there is no game clock to read, so the sign-in screen sits at
        the end of the cycle: full crimson, the night itself. It is the one
        place the panel is allowed to be purely atmospheric.
      */}
      <div style={{ "--moon": 1 } as React.CSSProperties}>
        <Atmosphere />
      </div>

      <div className="animate-rise relative w-full max-w-sm">
        {/* Seven tallies. Six days gone, and the seventh burning. */}
        <div className="flex items-end gap-1.5" aria-hidden>
          {Array.from({ length: BLOOD_MOON_CYCLE }, (_, i) => (
            <span
              key={i}
              className={
                i === BLOOD_MOON_CYCLE - 1
                  ? "animate-breathe h-8 flex-1 bg-crimson-lit shadow-[0_0_16px_var(--crimson)]"
                  : "h-8 flex-1 bg-ash-raised"
              }
              style={{ animationDelay: `${i * 60}ms` }}
            />
          ))}
        </div>

        <h1 className="mt-7 font-display text-4xl leading-none font-bold tracking-[0.1em] uppercase">
          7 Days to Die
        </h1>
        <p className="stencil mt-3">Server administration</p>

        <form onSubmit={handleSubmit} className="mt-9 space-y-4" noValidate>
          <div className="space-y-2">
            <Label htmlFor="username" className="stencil">
              Username
            </Label>
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
            <Label htmlFor="password" className="stencil">
              Password
            </Label>
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

          <Button
            type="submit"
            className="w-full font-display tracking-[0.14em] uppercase"
            disabled={signingIn}
          >
            {signingIn ? "Signing in…" : "Sign in"}
          </Button>
        </form>

        <p className="mt-8 text-xs text-bone-faint">
          The admin account comes from PANEL_ADMIN_PASSWORD. Change that variable and restart to
          reset it.
        </p>
      </div>
    </main>
  );
}
