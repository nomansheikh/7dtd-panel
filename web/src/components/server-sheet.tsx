import { toast } from "sonner";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { ServerForm } from "@/components/server-form";
import { useAddServer, useUpdateServer } from "@/hooks/use-server-admin";
import { type ServerSummary } from "@/lib/api";

/**
 * Adding or changing a game server, from anywhere that needs it.
 *
 * The same form the first-run page uses, so there is one place that knows what
 * a server needs and one place that tests it before saving.
 */
export function ServerSheet({
  editing,
  onClose,
}: {
  editing?: ServerSummary;
  onClose: () => void;
}) {
  const add = useAddServer();
  const update = useUpdateServer();
  const busy = add.isPending || update.isPending;

  return (
    <Sheet open onOpenChange={(open) => !open && onClose()}>
      <SheetContent side="right" className="w-full gap-0 overflow-y-auto p-0 sm:max-w-xl">
        <SheetHeader className="region-head shrink-0 space-y-0 p-3 md:px-4">
          <SheetTitle className="stencil">
            {editing ? `Edit ${editing.name}` : "Add a game server"}
          </SheetTitle>
          <SheetDescription className="sr-only">
            Where to reach the server, and a token it will accept.
          </SheetDescription>
        </SheetHeader>

        <div className="p-4 md:px-6">
          <ServerForm
            editing={editing}
            submitting={busy}
            submitLabel={editing ? "Save changes" : "Add it"}
            onSubmit={(input) => {
              const done = {
                onSuccess: () => {
                  toast.success(
                    editing
                      ? `${input.name || input.host} saved`
                      : `${input.name || input.host} added`,
                  );
                  onClose();
                },
                onError: (err: Error) =>
                  toast.error(editing ? "Not saved" : "Not added", { description: err.message }),
              };
              if (editing) update.mutate({ ...input, id: editing.id }, done);
              else add.mutate(input, done);
            }}
          />
        </div>
      </SheetContent>
    </Sheet>
  );
}
