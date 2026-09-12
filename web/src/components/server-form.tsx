import { useEffect, useState } from "react";
import { Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Field } from "@/components/field";
import { useTestServer } from "@/hooks/use-server-admin";
import { type ServerInput, type ServerSummary } from "@/lib/api";

/**
 * The details of one game server, and a way to find out whether they work
 * before committing to them.
 *
 * Test before save is the whole point. The panel used to take these from the
 * environment, which meant a typo was discovered by restarting a container and
 * finding an offline server — with no way to tell a wrong token from a server
 * that was simply switched off.
 */
export function ServerForm({
  editing,
  onSubmit,
  submitting,
  submitLabel,
}: {
  editing?: ServerSummary;
  onSubmit: (input: ServerInput) => void;
  submitting: boolean;
  submitLabel: string;
}) {
  const [id, setId] = useState(editing?.id ?? "");
  const [name, setName] = useState(editing?.name ?? "");
  const [host, setHost] = useState("");
  const [port, setPort] = useState("8080");
  const [tokenName, setTokenName] = useState("");
  const [tokenSecret, setTokenSecret] = useState("");

  const test = useTestServer();
  const input = (): ServerInput => ({
    id: id.trim(),
    name: name.trim(),
    host: host.trim(),
    port: Number(port) || 8080,
    tokenName: tokenName.trim(),
    tokenSecret,
  });

  // Any edit invalidates a previous answer: a green tick next to details that
  // have since changed is worse than no tick at all.
  useEffect(() => test.reset(), [id, name, host, port, tokenName, tokenSecret]);

  const ready = host.trim() !== "" && tokenName.trim() !== "" && id.trim() !== "";

  return (
    <div className="space-y-5">
      <div className="grid gap-5 sm:grid-cols-2">
        <Field label="Address" hint="Where this panel reaches the server, not what players type.">
          <div className="flex gap-1">
            <Input
              value={host}
              onChange={(e) => setHost(e.target.value)}
              placeholder="10.0.0.5"
              className="h-8 flex-1 text-sm"
              autoFocus
            />
            <Input
              value={port}
              onChange={(e) => setPort(e.target.value.replace(/\D/g, ""))}
              inputMode="numeric"
              className="readout h-8 w-20 text-center text-sm"
              aria-label="Port"
            />
          </div>
        </Field>

        <Field label="Called" hint="What you will see in the switcher.">
          <Input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Main survival"
            className="h-8 text-sm"
          />
        </Field>

        <Field label="Token name" hint="The name you gave it in webtokens add.">
          <Input
            value={tokenName}
            onChange={(e) => setTokenName(e.target.value)}
            placeholder="panel"
            className="h-8 text-sm"
          />
        </Field>

        <Field
          label="Token secret"
          hint={
            editing
              ? "Leave blank to keep the one already stored."
              : "The secret from the same command. It never leaves this panel."
          }
        >
          <Input
            type="password"
            value={tokenSecret}
            onChange={(e) => setTokenSecret(e.target.value)}
            placeholder={editing ? "unchanged" : ""}
            className="h-8 text-sm"
          />
        </Field>
      </div>

      <Field
        label="Id"
        hint="Appears in the panel's own URLs. Lowercase letters, digits and dashes; it cannot be changed later."
      >
        <Input
          value={id}
          onChange={(e) => setId(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ""))}
          placeholder="main"
          className="readout h-8 text-sm"
          disabled={Boolean(editing)}
        />
      </Field>

      <ProbeResult test={test} />

      <div className="flex flex-wrap gap-2">
        <Button
          variant="outline"
          disabled={!ready || test.isPending}
          onClick={() => test.mutate(input())}
        >
          {test.isPending && <Loader2 className="size-3 animate-spin" />}
          Test connection
        </Button>
        <Button disabled={!ready || submitting} onClick={() => onSubmit(input())}>
          {submitLabel}
        </Button>
      </div>
    </div>
  );
}

/** What the server said, or why it could not be asked. */
function ProbeResult({ test }: { test: ReturnType<typeof useTestServer> }) {
  if (test.isPending) {
    return <p className="text-xs text-bone-faint">Asking the server…</p>;
  }
  if (test.error) {
    return <p className="text-xs text-destructive">{test.error.message}</p>;
  }
  if (!test.data) return null;

  if (!test.data.ok) {
    return <p className="max-w-prose text-xs text-destructive">{test.data.problem}</p>;
  }

  const found = test.data.found;
  return (
    <p className="text-xs text-bone-dim">
      Found <span className="text-bone">{found?.name || "the server"}</span>
      {found?.world && <> running {found.world}</>}
      {found?.version && <span className="readout text-bone-faint"> {found.version}</span>}
      {typeof found?.players === "number" && <>, {found.players} online</>}.
    </p>
  );
}
