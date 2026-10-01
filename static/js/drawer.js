// drawer.js: a card opens in a panel from the right of the board, with its
// own address (spec §4.3). Without a script the link opens the card page.

const board = document.getElementById("board");
const drawer = document.getElementById("card-drawer");

function live() {
  return window.collageLive;
}

function panelURL(href) {
  const url = new URL(href, location.href);
  url.hash = "";
  url.pathname = url.pathname.replace(/\/?$/, "/panel");
  return url;
}

async function openCard(href, push) {
  const source = panelURL(href);
  const res = await fetch(source, { credentials: "same-origin" });
  if (!res.ok) {
    location.href = href;
    return;
  }
  const body = drawer.querySelector(".drawer__body");
  const panel = document.createElement("div");
  panel.id = "card-panel";
  panel.className = "card-panel";
  panel.dataset.collageFragment = source.pathname;
  panel.dataset.collagePush = "";
  panel.dataset.collageSwap = "morph";
  panel.innerHTML = await res.text();
  body.replaceChildren(panel);
  live()?.scan();
  // A card opened from inside the panel takes the open card's place in the
  // history, so closing the panel is always one step back to the board.
  if (push && drawer.open) history.replaceState({ card: href }, "", href);
  else if (push) history.pushState({ card: href }, "", href);
  if (!drawer.open) drawer.showModal();
  panel.dispatchEvent(new CustomEvent("card:panel", { bubbles: true }));
  panel.querySelector("[data-drawer-close]")?.focus({ preventScroll: true });
}

function closed() {
  drawer.querySelector(".drawer__body").replaceChildren();
  document.dispatchEvent(new CustomEvent("card:closed"));
  live()?.scan();
}

if (board && drawer) {
  document.addEventListener("click", (e) => {
    if (!(e.target instanceof Element)) return;
    if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || e.button !== 0) return;
    const close = e.target.closest("[data-drawer-close]");
    if (close && drawer.contains(close)) {
      e.preventDefault();
      drawer.close();
      return;
    }
    const link = e.target.closest("a[data-card-link]");
    if (!link || !(board.contains(link) || drawer.contains(link))) return;
    e.preventDefault();
    openCard(link.href, true);
  });
  // A click on the dimmed board beside the panel closes it.
  drawer.addEventListener("click", (e) => {
    if (e.target === drawer) drawer.close();
  });
  drawer.addEventListener("close", () => {
    closed();
    if (history.state && history.state.card) history.back();
  });
  window.addEventListener("popstate", () => {
    const card = history.state && history.state.card;
    if (card) openCard(card, false);
    else if (drawer.open) drawer.close();
  });
}
