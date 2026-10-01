// board.js: drag and drop between columns, and the card dialog (spec §8).
// Sortable is loaded before this module as a classic script (window.Sortable);
// collage-live's client provides window.collageLive.

const board = document.getElementById("board");
const dialog = document.getElementById("card-dialog");

function csrfToken() {
  const input = document.querySelector('input[name="_csrf"]');
  return input ? input.value : "";
}

function live() {
  return window.collageLive;
}

// setupSortables attaches a Sortable to every column list. The board is
// morphed in place by collage-live, so lists that already have one are skipped.
function setupSortables() {
  if (!board || !window.Sortable) return;
  for (const list of board.querySelectorAll(".column__cards")) {
    if (list.dataset.sortable) continue;
    list.dataset.sortable = "1";
    window.Sortable.create(list, {
      group: "cards",
      animation: 120,
      onStart: () => live()?.pause(board),
      onEnd: onDrop,
    });
  }
}

async function onDrop(evt) {
  const card = evt.item;
  const form = new FormData();
  form.set("op", "move");
  form.set("card", card.dataset.card);
  form.set("to_column", evt.to.dataset.column);
  form.set("to_index", String(evt.newIndex));
  form.set("expected_from", card.dataset.column);
  form.set("expected_version", card.dataset.version);
  form.set("_csrf", csrfToken());
  try {
    // Dropped where it started: nothing to tell the server.
    if (evt.from === evt.to && evt.oldIndex === evt.newIndex) return;
    const res = await fetch(board.dataset.moveUrl, {
      method: "POST",
      body: form,
      headers: { "Collage-Fetch": "1", "X-CSRF-Token": csrfToken() },
      credentials: "same-origin",
    });
    // 200: the board as moved. 409 and 422: the board as it really is, with a
    // notice; the card goes back where it was without any undo code here.
    if (res.ok || res.status === 409 || res.status === 422) {
      live()?.put(board, await res.text());
    } else {
      live()?.refresh(board);
    }
  } catch {
    live()?.refresh(board);
  } finally {
    live()?.resume(board);
    setupSortables();
  }
}

// openCard loads a card's panel into the dialog and puts its URL in the
// address bar. Without a script the link opens the card page.
async function openCard(href, push) {
  const res = await fetch(href.replace(/\/?$/, "/panel"), { credentials: "same-origin" });
  if (!res.ok) {
    window.location.href = href;
    return;
  }
  const body = dialog.querySelector(".card-dialog__body");
  body.innerHTML = "";
  const panel = document.createElement("div");
  panel.id = "card-panel";
  panel.className = "card-panel";
  panel.dataset.collageFragment = new URL(href.replace(/\/?$/, "/panel"), window.location.href).pathname;
  panel.dataset.collagePush = "";
  panel.dataset.collageSwap = "morph";
  panel.innerHTML = await res.text();
  body.append(panel);
  live()?.scan();
  if (push) history.pushState({ card: href }, "", href);
  if (!dialog.open) dialog.showModal();
}

if (board && dialog) {
  board.addEventListener("click", (e) => {
    const link = e.target.closest("a.card__title");
    if (!link || e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
    e.preventDefault();
    openCard(link.href, true);
  });
  dialog.addEventListener("close", () => {
    if (history.state && history.state.card) history.back();
  });
  window.addEventListener("popstate", () => {
    if (dialog.open && !(history.state && history.state.card)) dialog.close();
  });
  board.addEventListener("collage:swap", setupSortables);
  setupSortables();
}
