import { createContext, useContext, useMemo, type ReactNode } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, type User } from "@/lib/api";

interface AuthValue {
  user: User | null;
  /** True only while the very first session check is in flight. */
  loading: boolean;
  signIn: (username: string, password: string) => Promise<void>;
  signOut: () => Promise<void>;
  signingIn: boolean;
}

const AuthContext = createContext<AuthValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();

  const session = useQuery({
    queryKey: ["session"],
    queryFn: api.me,
    // A 401 is the expected answer when nobody is signed in, so retrying it
    // just delays showing the sign-in form.
    retry: false,
    staleTime: 60_000,
  });

  const signInMutation = useMutation({
    mutationFn: ({ username, password }: { username: string; password: string }) =>
      api.login(username, password),
    onSuccess: (user) => {
      queryClient.setQueryData(["session"], user);
    },
  });

  const value = useMemo<AuthValue>(
    () => ({
      user: session.data ?? null,
      loading: session.isLoading,
      signingIn: signInMutation.isPending,
      signIn: async (username, password) => {
        await signInMutation.mutateAsync({ username, password });
      },
      signOut: async () => {
        await api.logout();
        // Drop every cached answer, not just the session: they were fetched
        // as this user and should not flash on screen for the next one.
        queryClient.clear();
        queryClient.setQueryData(["session"], null);
      },
    }),
    [session.data, session.isLoading, signInMutation, queryClient],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthValue {
  const value = useContext(AuthContext);
  if (!value) {
    throw new Error("useAuth must be used inside AuthProvider");
  }
  return value;
}

/** True when an error means the session has gone. */
export function isSessionExpired(error: unknown): boolean {
  return error instanceof ApiError && error.isUnauthenticated;
}
