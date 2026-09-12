# 7dtd-panel — interface guidelines

The rulebook for anything that renders, so a page built next year by
somebody who has never read this repository comes out looking like a page
built today. Where a rule has a reason, the reason matters more: a rule
applied without it is how a design drifts.

Every token named here lives in `web/src/index.css`. Do not invent one
without adding it there first.

---

## 1. What this thing is

An instrument panel for a server trying to kill the people on it. Dark,
flat, ruled in hairlines, set in condensed capitals and monospace figures —
something bolted to a wall in a badly lit room, not a SaaS dashboard.

Three non-negotiable consequences:

- **No rounded corners.** `--radius: 0`. A rounded control in a
  hairline-ruled list reads as something that came from another product.
- **No shadows.** Depth comes from hairlines and from surfaces that differ
  by a few percent of lightness, never from a drop shadow.
- **Regions, not cards.** See §4.

## 2. Colour

Four ramps, all in oklch, all defined for both themes. Never write a raw
colour in a component; use the token.

| Ramp | What it is for |
| --- | --- |
| `--ash-950` … `--ash-700` | Surfaces, from the page behind everything to the lightest raised thing |
| `--bone`, `--bone-dim`, `--bone-faint` | Text, from what you read first to what you read last |
| `--crimson-deep`, `--crimson`, `--crimson-lit` | The blood moon, destruction, and anything that cannot be undone |
| `--ember` | One warm accent. Attention, not alarm |

**Crimson is not a decoration.** It means the blood moon, or that this will
destroy something: a `destructive` tier, a delete, a ban. Reaching for it
because something feels important is wrong — use `--ember`.

The light theme **inverts the ash and bone ramps** rather than remapping
semantic tokens. Define any new semantic token in terms of a ramp and both
themes get it for free.

### The sky is data

`--moon` (0–1, how close the blood moon is), `--moon-heat` and
`--moon-night` come from the live game clock. Mix against them rather than
inventing a schedule. The panel looking different at midnight on day 7 than
at noon on day 2 is the point.

## 3. Type

Three families, each with one job:

- `--font-display` (Big Shoulders Display) — headings and `.stencil` labels.
- `--font-sans` (Archivo) — prose.
- `--font-mono` (IBM Plex Mono) — anything the game said or that gets typed
  into it: commands, ids, coordinates, counts, timestamps.

**Never hardcode a font size.** Every `--text-*` step is a `clamp()` that
grows with the viewport. A hardcoded `text-[13px]` breaks the scale on
every screen but the one you were looking at.

Four utility classes carry the voice; use them rather than reassembling
their parts:

- `.stencil` — a section label: small, condensed, uppercase, wide-tracked.
  Frames data; never competes with it.
- `.readout` — monospace, tabular figures. **Any number that changes** gets
  this, or the digits jitter as it ticks.
- `.figure` — a numeral read from across the room.
- `.region-head` — the label on a region's top edge.

## 4. Layout

**Regions, not cards.** An early pass boxed everything — bordered, rounded,
shadowed, eight stacked down a page. Eight identical boxes carry no
hierarchy, so nothing looked more important than anything else. Regions are
separated by hairlines and bleed to the edges of the work area, the way an
instrument panel and a newspaper page are laid out.

- `.region` — a bordered area with a subtle gradient, labelled by
  `.region-head`.
- `.panel` — for grids of discrete controls, where a boundary is the point.

The work area is capped and centred with `--shell-gutter`. The sidebar is
`fixed`, so it must be offset by the same gutter or it detaches on a wide
screen.

**Use the width you have.** On `lg` and up, a list with a reference beside
it is two columns, not a stack — see Chat and Automation: settings left,
"what it looks like in game" right.

## 5. Controls

### Pick the right one

| Options | Control |
| --- | --- |
| 2, and they are opposites | `Switch` |
| 2–3, and seeing them all helps | A row of bordered buttons |
| 4 or more | `Select` |
| A number with a sensible range | `InputGroup` stepper, never a bare field |

**Four or more choices is a dropdown.** A row of seven chips was the worst
thing in this panel's history: two rows per item, most of the screen, six
of the seven always wrong. More than three options side by side — stop.

### Sizing, and one trap

Compact rows use **`h-7` (28px)** — steppers, selects, icon buttons. Forms
inside a sheet use `h-8`.

The trap: shadcn's `SelectTrigger` sizes itself with
`data-[size=default]:h-9`. tailwind-merge does not treat an attribute
variant as conflicting with a bare class, so `className="h-7"` **does
nothing** — both survive and the variant wins. Always write:

```tsx
className="h-7 ... data-[size=default]:h-7"
```

Measure, do not eyeball. `getBoundingClientRect().height` in the browser.

### Settings appear when they apply

A switched-off thing shows its name, description and switch. Settings
appear only once it is on — showing a cooldown for a command nobody enabled
invites the reading that it is doing something.

Reveal them **on the row the switch is already on**, not by unfolding rows
underneath. A switch changes what is on a line, not how many lines exist.

### Sheets and dialogs

- A right `Sheet` for browsing or building: `sm:max-w-3xl` when there is a
  grid to see, `sm:max-w-xl` for a form.
- `AlertDialog` for anything irreversible, with the consequence spelled out
  — "anybody who types `!kit starter` will be told there is no such kit",
  not "this cannot be undone".

## 6. Words

The panel talks like a person who knows the game and respects the reader.

- **Sentence case.** Not Title Case. Not ALL CAPS outside `.stencil`.
- **Say what happens, not what the control is.** "Hands over a kit the
  panel has saved" beats "Kit command".
- **Never invent official-looking game vocabulary.** The server exposes no
  tier names, so item quality reads `Crude · 1`, `Flawless · 6`: the panel's
  word beside the game's number. A word alone is invented vocabulary dressed
  up as the game's; a number alone says nothing. **Name the thing, keep the
  number beside it** — admin levels do the same (`Owner 0`, `Moderator 2`).
- **Durations are phrases.** "Once an hour", "30 minutes before the blood
  moon". Never `3600`.
- **Explain the why under the control**, in `text-2xs text-bone-faint`.
  Every setting here has a reason a name cannot carry.
- **Empty states teach.** Say what the thing is for and give two or three
  concrete examples, then the button. Never just "No items".
- **Name the danger.** A destructive line is labelled `destructive` in
  crimson before the switch beside it is touched. Never hide power; show
  it and let the operator decide.

## 7. Honesty

The panel must never assert something it does not know.

- Cached readings say how old they are. A blip holds the last good value
  rather than blanking the screen or flipping to zero.
- If a value could not be read, say so. Do not render `0`.
- If a figure is inferred rather than reported, label it — "an example of
  the shape, not a live reading".

## 8. Accessibility

- Every icon-only control gets an `aria-label` naming its target: "Delete
  the starter kit", not "Delete".
- Contrast is measured, not judged. oklch does not parse as RGB — paint the
  colour to a 1×1 canvas and read the pixel back.
- Colour is never the only signal: a destructive line is crimson **and**
  says `destructive`.

## 9. Files

No file over **200 lines**. Split by responsibility: a page splits into its
sections, a component into the part that decides and the part that draws.
Generated code and vendored `web/src/components/ui/*` are exempt.

---

## Checklist before opening a PR that touches the interface

- [ ] No hardcoded font sizes, colours, radii or shadows
- [ ] Numbers that change use `.readout`
- [ ] More than three options is a dropdown
- [ ] Selects that need a height also override `data-[size=default]`
- [ ] Control heights match their neighbours — measured, not eyeballed
- [ ] Icon-only buttons have labels naming their target
- [ ] Empty states teach; destructive things are named
- [ ] Durations and enums read as words with the number beside them
- [ ] Checked at a narrow width as well as wide
- [ ] No file over 200 lines
