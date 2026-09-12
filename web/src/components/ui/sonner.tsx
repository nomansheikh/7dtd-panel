import { Toaster as Sonner, type ToasterProps } from "sonner";

/**
 * Toasts, in the panel's own hand.
 *
 * Inheriting the colour tokens was not enough: sonner still drew an 8px
 * radius and set the text in its own ui-sans stack, so the one element that
 * appears over everything else was the only rounded, differently-lettered
 * thing on screen. Square, bordered and set in Archivo, it reads as another
 * panel surface arriving rather than as a notification from another website.
 *
 * The description is always a raw server command, so it is set in the mono
 * face the console and the readouts use. That is the point of it — the title
 * says what happened in the page's words, and the line underneath is the
 * evidence, in the same type as everywhere else a command is shown.
 */
export function Toaster(props: ToasterProps) {
  return (
    <Sonner
      theme="dark"
      className="toaster group"
      position="bottom-right"
      // Long enough to read a command off, short enough not to sit in the way.
      duration={5000}
      toastOptions={{
        classNames: {
          toast: "!rounded-none !border !border-border !bg-popover !font-sans !gap-3 !shadow-none",
          title: "!text-sm !font-medium !text-bone",
          description: "readout !mt-1 !text-xs !text-bone-faint",
          icon: "!mr-0 !size-4 !shrink-0 !self-start !mt-0.5",
          success: "[&_[data-icon]]:!text-bone-dim",
          error:
            "!border-crimson [&_[data-icon]]:!text-crimson-lit [&_[data-description]]:!font-sans [&_[data-description]]:!tracking-normal [&_[data-description]]:!text-bone-dim",
          closeButton: "!rounded-none !border-border !bg-popover !text-bone-dim",
        },
      }}
      style={
        {
          "--normal-bg": "var(--popover)",
          "--normal-text": "var(--popover-foreground)",
          "--normal-border": "var(--border)",
          "--error-bg": "var(--popover)",
          "--error-text": "var(--bone)",
          "--error-border": "var(--crimson)",
        } as React.CSSProperties
      }
      {...props}
    />
  );
}
