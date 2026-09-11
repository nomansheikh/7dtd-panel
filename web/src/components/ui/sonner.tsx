import { Toaster as Sonner, type ToasterProps } from "sonner";

/**
 * Toasts inherit the panel's own tokens rather than sonner's light defaults.
 * The theme is fixed to dark on <html>, so there is no theme provider to read.
 */
export function Toaster(props: ToasterProps) {
  return (
    <Sonner
      theme="dark"
      className="toaster group"
      position="bottom-right"
      style={
        {
          "--normal-bg": "var(--popover)",
          "--normal-text": "var(--popover-foreground)",
          "--normal-border": "var(--border)",
          "--error-bg": "var(--popover)",
          "--error-text": "var(--destructive-foreground)",
          "--error-border": "var(--destructive)",
        } as React.CSSProperties
      }
      {...props}
    />
  );
}
