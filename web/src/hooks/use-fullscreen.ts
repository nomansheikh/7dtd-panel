import { useCallback, useEffect, useState } from "react";

/*
Fills the screen with one page, by taking the whole document full screen and
hiding the app's own chrome with a class.

The obvious implementation — requesting full screen on just the page's own
element — is wrong. A browser paints only the full-screen element's subtree, and
every overlay in this app is portalled to document.body: the map's context menu
rendered into nothing, and so would every toast. Taking the document full screen
keeps all of them inside it.

The browser can also leave full screen without being asked, so the state is read
from the document rather than remembered from the last click.
*/
const CHROME_HIDDEN = "immersive";

export function useFullscreen() {
  const [isFullscreen, setIsFullscreen] = useState(false);

  useEffect(() => {
    const sync = () => {
      const on = document.fullscreenElement === document.documentElement;
      setIsFullscreen(on);
      document.documentElement.classList.toggle(CHROME_HIDDEN, on);
    };
    document.addEventListener("fullscreenchange", sync);
    sync();
    return () => {
      document.removeEventListener("fullscreenchange", sync);
      document.documentElement.classList.remove(CHROME_HIDDEN);
    };
  }, []);

  const toggle = useCallback(async () => {
    try {
      if (document.fullscreenElement) {
        await document.exitFullscreen();
      } else {
        await document.documentElement.requestFullscreen();
      }
    } catch {
      /* Refused: untrusted gesture, or disabled. Leaving the page as it was is
         the whole recovery. */
    }
  }, []);

  /* Safari on iPhone has no requestFullscreen, so hide the control rather than
     offer one that fails. */
  const supported = typeof document !== "undefined" && document.fullscreenEnabled === true;

  return { isFullscreen, toggle, supported };
}
