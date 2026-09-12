import { Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

/**
 * The list of console lines something will run.
 *
 * Numbered, because the order is the whole meaning: warn, then save, then shut
 * down is a restart, and the same three in another order is a mess.
 */
export function CommandLines({
  lines,
  onChange,
  placeholder,
}: {
  lines: string[];
  onChange: (next: string[]) => void;
  placeholder: string;
}) {
  return (
    <div className="space-y-1.5">
      {lines.map((line, i) => (
        <div key={i} className="flex items-center gap-1">
          <span className="readout w-4 shrink-0 text-2xs text-bone-faint">{i + 1}</span>
          <Input
            value={line}
            onChange={(e) => onChange(lines.map((l, at) => (at === i ? e.target.value : l)))}
            placeholder={placeholder}
            className="readout h-8 flex-1 text-xs"
          />
          <Button
            variant="ghost"
            size="icon"
            className="size-8 shrink-0 text-bone-faint"
            aria-label={`Remove line ${i + 1}`}
            onClick={() => onChange(lines.filter((_, at) => at !== i))}
          >
            <Trash2 className="size-3" />
          </Button>
        </div>
      ))}
      <Button
        variant="outline"
        size="sm"
        className="h-7 gap-1 text-xs"
        onClick={() => onChange([...lines, ""])}
      >
        <Plus className="size-3" />
        {lines.length === 0 ? "Add a command" : "Another"}
      </Button>
    </div>
  );
}
