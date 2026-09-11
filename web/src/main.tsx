import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { QueryCache, QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { toast } from "sonner";
import { Toaster } from "@/components/ui/sonner";
import { AuthProvider } from "@/hooks/use-auth";
import { ServersProvider } from "@/hooks/use-servers";
import { ApiError } from "@/lib/api";
import App from "./App";
import "./index.css";

const queryClient = new QueryClient({
  queryCache: new QueryCache({
    onError: (error, query) => {
      // A lapsed session is expected rather than exceptional, and the app
      // already falls back to the sign-in form, so do not shout about it.
      if (error instanceof ApiError && error.isUnauthenticated) {
        queryClient.setQueryData(["session"], null);
        return;
      }
      // When a refetch fails but stale data is still on screen, the page body
      // stays useful, so the failure belongs in a toast rather than replacing
      // the content. Always the real message, never "something went wrong".
      if (query.state.data !== undefined) {
        toast.error("Could not refresh", { description: error.message });
      }
    },
  }),
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
});

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <ServersProvider>
          <App />
          <Toaster />
        </ServersProvider>
      </AuthProvider>
    </QueryClientProvider>
  </StrictMode>,
);
