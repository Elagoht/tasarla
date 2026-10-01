# 004 — collage-live: a swap cannot be wrapped in a view transition

- **Type:** feature request (nothing documented is broken)
- **Packages:** `github.com/Elagoht/collage-live` v0.3.0
- **Found while:** Kanban, animating the board. When another reader moves a card, the board's push puts in the new columns; the cards should slide to their new places (View Transitions API) rather than jump.

## What the documentation offers

README, "The client":

> - Dispatches `collage:swap` and `collage:stale` events on the element.

`collage:swap` fires after the new HTML is in place (`client.js`, `swap`). A view transition has to be started *before* the DOM changes — `document.startViewTransition(update)` captures the old state, runs `update`, then captures the new one — so an event after the swap is too late, and nothing runs before it.

The page can wrap the swaps it starts itself (`collageLive.put`), and does. It cannot wrap the swaps the client makes on its own: pushes, polls and `data-collage-target` form answers.

## What would help

Any one of these:

1. **An attribute:** `data-collage-transition` on a fragment element makes the client run its swap inside `document.startViewTransition` when the browser has it (and not under `prefers-reduced-motion: reduce`).
2. **A cancelable event before the swap:** `collage:before-swap` with `detail.swap()`; a listener that calls `preventDefault()` takes over and runs `detail.swap()` itself, inside its own transition.
3. **A hook:** `collageLive.onSwap(el, (swap) => …)` with the same contract as 2.

Option 2 or 3 also lets the page name the elements that should move (for a board, each card by its id) just before and just after the swap, which a view transition needs.

## Where the app stands

- Cross-document transitions (`@view-transition`) and the transitions around the board's own `put` calls (a refused drop, a new card) are in place.
- Cards moved by someone else still jump into place. No workaround is written: the app waits for one of the above.
