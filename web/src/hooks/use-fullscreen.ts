import { useCallback, useEffect, useState, type RefObject } from "react";

/* The browser can leave full screen on its own — Escape, switching away — so
   the state is read from the document rather than remembered from the click. */
export function useFullscreen(target: RefObject<HTMLElement | null>) {
  const [isFullscreen, setIsFullscreen] = useState(false);

  useEffect(() => {
    /* The null check matters: before first paint both are null, and
       null === null would claim the page is already full screen. */
    const sync = () =>
      setIsFullscreen(
        document.fullscreenElement !== null && document.fullscreenElement === target.current,
      );
    document.addEventListener("fullscreenchange", sync);
    sync();
    return () => document.removeEventListener("fullscreenchange", sync);
  }, [target]);

  const toggle = useCallback(async () => {
    const element = target.current;
    if (!element) return;
    try {
      if (document.fullscreenElement) {
        await document.exitFullscreen();
      } else {
        await element.requestFullscreen();
      }
    } catch {
      /* Refused: untrusted gesture, or disabled. Leaving the map is the
         whole recovery. */
    }
  }, [target]);

  /* Safari on iPhone has no requestFullscreen, so hide the control rather
     than offer one that fails. */
  const supported = typeof document !== "undefined" && document.fullscreenEnabled === true;

  return { isFullscreen, toggle, supported };
}
