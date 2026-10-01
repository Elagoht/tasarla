// board.js: drag and drop between columns (spec §8). Sortable is loaded
// before this module as a classic script (window.Sortable); collage-live's
// client provides window.collageLive.

const board = document.getElementById("board");

function csrfToken() {
  const input = document.querySelector('input[name="_csrf"]');
  return input ? input.value : "";
}

function live() {
  return window.collageLive;
}

// setupSortables attaches a Sortable to every column list that has none. The
// board is morphed in place by collage-live, which drops attributes the server
// did not send, so the check asks Sortable itself rather than a data attribute;
// otherwise every push would stack another instance, and one drop would post
// one move per instance.
function setupSortables() {
  if (!board || !window.Sortable) return;
  for (const list of board.querySelectorAll(".column__cards")) {
    if (window.Sortable.get(list)) continue;
    window.Sortable.create(list, {
      group: "cards",
      animation: 150,
      draggable: ".card",
      // A done column is always empty now: let a card drop anywhere over it,
      // not only near its edge.
      emptyInsertThreshold: 60,
      ghostClass: "card--ghost",
      chosenClass: "card--chosen",
      dragClass: "card--drag",
      onStart: () => live()?.pause(board),
      onEnd: onDrop,
    });
  }
}

// Every change to the board — a drop, a refused one, a new card, and what
// someone else did, arriving by push — moves the cards to their new places in
// a view transition. collage-live asks first (collage:before-swap); the board
// takes the swap over, names each card by its id on both sides of it (the
// swap drops the names with the rest of what the server did not send; the
// CSP keeps them out of the markup), and runs it inside the transition.
function nameCards(name) {
  for (const c of board.querySelectorAll(".card")) c.style.viewTransitionName = name ? "card-" + c.dataset.card : "";
}

function swapInTransition(e) {
  if (e.target !== board || !document.startViewTransition || reduced()) return;
  e.preventDefault();
  nameCards(true);
  const t = document.startViewTransition(() => {
    e.detail.swap();
    nameCards(true);
  });
  t.ready.catch(() => {});
  t.finished.finally(() => nameCards(false));
}

// inTransition puts html into the board and lets it go on: the board is held
// while a card is dragged or added, and takes the answer on resume — through
// swapInTransition. It settles once the board shows the answer.
function inTransition(html) {
  const shown = new Promise((resolve) => {
    board.addEventListener("collage:swap", resolve, { once: true });
    setTimeout(resolve, 1500);
  });
  live()?.put(board, html);
  live()?.resume(board);
  return shown;
}

// Completing: a card dropped into a done column is ticked off — a ring draws
// round a tick — and, once the server agrees, folds away; the board's answer
// no longer holds it. Refused, the mark goes and the card goes back.
const reduced = () => window.matchMedia("(prefers-reduced-motion: reduce)").matches;
const pause = (ms) => new Promise((r) => setTimeout(r, ms));

function startCompleting(card) {
  const mark = document.createElement("span");
  mark.className = "complete-burst";
  mark.setAttribute("aria-hidden", "true");
  mark.innerHTML =
    '<svg viewBox="0 0 52 52"><circle class="complete-burst__ring" cx="26" cy="26" r="22"/>' +
    '<path class="complete-burst__tick" d="M16 27l7 7 14-15"/></svg>';
  card.append(mark);
  card.classList.add("card--completing");
}

async function finishCompleting(card, started) {
  const shown = reduced() ? 150 : 800;
  await pause(Math.max(0, shown - (performance.now() - started)));
  card.style.height = card.offsetHeight + "px";
  card.classList.add("card--leaving");
  await pause(reduced() ? 50 : 380);
}

function stopCompleting(card) {
  card.classList.remove("card--completing", "card--leaving");
  card.querySelector(".complete-burst")?.remove();
}

async function onDrop(evt) {
  let resumed = false;
  const card = evt.item;
  const form = new FormData();
  form.set("op", "move");
  form.set("card", card.dataset.card);
  form.set("to_column", evt.to.dataset.column);
  form.set("to_index", String(evt.newIndex));
  form.set("expected_from", card.dataset.column);
  form.set("expected_version", card.dataset.version);
  form.set("_csrf", csrfToken());
  const completing = evt.from !== evt.to && evt.to.closest(".column")?.hasAttribute("data-done");
  try {
    // Dropped where it started: nothing to tell the server.
    if (evt.from === evt.to && evt.oldIndex === evt.newIndex) return;
    const started = performance.now();
    if (completing) startCompleting(card);
    const res = await fetch(board.dataset.moveUrl, {
      method: "POST",
      body: form,
      headers: { "Collage-Fetch": "1", "X-CSRF-Token": csrfToken() },
      credentials: "same-origin",
    });
    // 200: the board as moved. 409 and 422: the board as it really is, with a
    // notice; the card goes back where it was without any undo code here.
    if (res.ok || res.status === 409 || res.status === 422) {
      const html = await res.text();
      if (completing && res.ok) await finishCompleting(card, started);
      else if (completing) stopCompleting(card);
      inTransition(html);
      resumed = true;
    } else {
      if (completing) stopCompleting(card);
      live()?.refresh(board);
    }
  } catch {
    if (completing) stopCompleting(card);
    live()?.refresh(board);
  } finally {
    if (!resumed) live()?.resume(board);
    setupSortables();
  }
}

// "Add card" is sent from here, and its answer — the columns — put in place.
// A refused card keeps its form open with the title typed, under the reason;
// a page load would lose both to the first live copy of the board.
async function addCard(form) {
  const column = form.querySelector('input[name="column"]')?.value;
  const title = form.querySelector('textarea[name="title"]')?.value ?? "";
  live()?.pause(board);
  try {
    const res = await fetch(form.action, {
      method: "POST",
      body: new FormData(form),
      headers: { "Collage-Fetch": "1", Accept: "text/html" },
      credentials: "same-origin",
    });
    if (res.ok || res.status === 422) {
      await inTransition(await res.text());
      const again = board.querySelector(`.add-card__form input[name="column"][value="${CSS.escape(column ?? "")}"]`)?.closest("details");
      if (again) {
        const area = again.querySelector("textarea");
        again.open = true;
        if (area) {
          area.value = res.ok ? "" : title;
          area.focus();
        }
      }
      return;
    }
    live()?.resume(board);
    live()?.refresh(board);
  } catch {
    live()?.resume(board);
    live()?.refresh(board);
  }
}

// A refused move is explained above the columns for a few seconds.
function dismissAlerts() {
  for (const alert of board.querySelectorAll("[data-board-alert]")) {
    setTimeout(() => {
      alert.classList.add("is-leaving");
      setTimeout(() => alert.remove(), 250);
    }, 6000);
  }
}

if (board) {
  board.addEventListener("collage:before-swap", swapInTransition);
  board.addEventListener("submit", (e) => {
    const form = e.target;
    if (!(form instanceof HTMLFormElement) || !form.matches(".add-card__form")) return;
    e.preventDefault();
    addCard(form);
  });
  board.addEventListener("collage:swap", () => {
    setupSortables();
    dismissAlerts();
  });
  // A column's "Add card" opens with the title field ready.
  board.addEventListener("toggle", (e) => {
    const details = e.target;
    if (details instanceof HTMLDetailsElement && details.open) details.querySelector("textarea")?.focus();
  }, true);
  setupSortables();
  dismissAlerts();
}
