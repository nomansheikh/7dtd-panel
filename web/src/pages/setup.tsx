import { toast } from "sonner";
import { ServerForm } from "@/components/server-form";
import { useAddServer } from "@/hooks/use-server-admin";

/**
 * The first thing a new install shows.
 *
 * Before this the panel refused to start without a game server in its
 * environment, so the first experience of it was a container that would not
 * boot and a compose file to guess at. Now it starts, says it has nothing to
 * manage yet, and tells you exactly how to get the one thing it needs.
 */
export function SetupPage() {
  const add = useAddServer();

  return (
    <div className="mx-auto max-w-2xl px-4 py-10 md:px-6">
      <span className="stencil">First run</span>
      <h1 className="mt-2 font-display text-2xl text-bone">Add a game server</h1>
      <p className="mt-2 max-w-prose text-sm text-bone-dim">
        The panel has nothing to manage yet. It needs somewhere to reach your 7 Days to Die server,
        and a token that server will accept.
      </p>

      <section className="panel mt-6 p-4">
        <span className="stencil">Making a token</span>
        <p className="mt-2 max-w-prose text-xs text-bone-dim">
          On the game server's own console, run:
        </p>
        <pre className="readout mt-2 overflow-x-auto border border-border p-3 text-xs text-bone">
          webtokens add panel a-long-random-secret 0
        </pre>
        <p className="mt-2 max-w-prose text-2xs text-bone-faint">
          A level of 0 is maximum permission, which is what the panel's write commands need. This
          needs the Allocs web interface mod enabled. The secret is stored by the panel and never
          sent to a browser — every call to the game goes through the panel.
        </p>
      </section>

      <section className="mt-8">
        <ServerForm
          submitting={add.isPending}
          submitLabel="Add it"
          onSubmit={(input) =>
            add.mutate(input, {
              onSuccess: () => toast.success(`${input.name || input.host} added`),
              onError: (err) => toast.error("Not added", { description: err.message }),
            })
          }
        />
      </section>

      <p className="mt-8 max-w-prose text-2xs text-bone-faint">
        You can add more servers later, and environment variables still work if you would rather
        manage this as code — anything set there is imported the first time the panel sees it.
      </p>
    </div>
  );
}
