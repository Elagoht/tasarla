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
      ghostClass: "card--ghost",
      chosenClass: "card--chosen",
      dragClass: "card--drag",
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
      live()?.put(board, await res.text());
      live()?.resume(board);
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
