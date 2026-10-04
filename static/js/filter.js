// filter.js: the board filter (spec 2026-10-01 filtre… §3.1). The bar is a GET
// form that works without this script; here it is sent as it changes, without
// a page load: the address bar takes the new query, and so does the live
// element's fragment URL.
//
// collage-live finds an element by its exact fragment URL and reads the URLs
// it watches only when its stream opens, so after the URL changes scan() must
// reopen the stream; refresh() alone would leave pushes going to the old URL,
// which no element shows any more.

const bar = document.querySelector("[data-filter-bar]");
const target = document.getElementById("board") || document.getElementById("gantt") || document.getElementById("calendar");
// The Gantt page's scale and grouping links, and the calendar's month links,
// carry the filter in their own URLs, made on the server: a filter changed
// there loads the page, so the links are made again; its text box is sent
// with Enter only.
const navigates = target?.id === "gantt" || target?.id === "calendar";

function query() {
  const params = new URLSearchParams();
  for (const [k, v] of new FormData(bar)) {
    if (typeof v === "string" && v.trim() !== "") params.append(k, v.trim());
  }
  return params.toString();
}

// The bar's settings (the lanes, the Gantt chart's scale…) are not the filter:
// they stay in the URL, but do not count towards "a filter is on" and survive
// Clear.
function params(settingsOnly) {
  const out = new URLSearchParams();
  for (const el of bar.elements) {
    if (!(el instanceof HTMLInputElement || el instanceof HTMLSelectElement) || !el.name) continue;
    if (settingsOnly !== el.hasAttribute("data-filter-setting")) continue;
    if (el instanceof HTMLInputElement && (el.type === "checkbox" || el.type === "radio") && !el.checked) continue;
    if (el.value.trim() !== "") out.append(el.name, el.value.trim());
  }
  return out;
}

function withQuery(url, q) {
  const base = url.split("?")[0];
  return q ? base + "?" + q : base;
}

// The bar sits outside the live fragment, so the server's rendering of it is
// that of the page load: the Clear link and the pickers' marks follow the form.
const clear = bar?.querySelector("[data-filter-clear]");
function syncBar() {
  if (clear) {
    clear.hidden = params(false).toString() === "";
    clear.href = withQuery(location.pathname, params(true).toString());
  }
  for (const pick of bar.querySelectorAll(".filter-pick")) {
    pick.classList.toggle("is-on", pick.querySelector("input:checked") !== null);
  }
}

let timer = 0;
function apply() {
  clearTimeout(timer);
  const q = query();
  history.replaceState(history.state, "", withQuery(location.pathname, q));
  syncBar();
  target.dataset.collageFragment = withQuery(target.dataset.collageFragment, q);
  if (target.dataset.moveUrl) target.dataset.moveUrl = withQuery(target.dataset.moveUrl, q);
  const live = window.collageLive;
  if (live && !navigates) {
    live.scan();
    live.refresh(target);
  } else {
    location.search = q;
  }
}

if (bar && target) {
  bar.addEventListener("submit", (e) => {
    e.preventDefault();
    apply();
  });
  bar.addEventListener("change", (e) => {
    if (e.target instanceof HTMLInputElement && e.target.matches("[data-filter-text]")) return;
    apply();
  });
  bar.querySelector("[data-filter-text]")?.addEventListener("input", () => {
    if (navigates) return;
    clearTimeout(timer);
    timer = setTimeout(apply, 300);
  });
}

// The pickers are dropdowns: one open at a time, closed by a click outside or
// Escape, and opened towards the left when they would leave the screen.
if (bar) {
  const pickers = () => bar.querySelectorAll("details.filter-pick");
  const closeAll = (except) => {
    for (const p of pickers()) if (p !== except) p.open = false;
  };
  bar.addEventListener("toggle", (e) => {
    const picker = e.target;
    if (!(picker instanceof HTMLDetailsElement) || !picker.matches(".filter-pick") || !picker.open) return;
    closeAll(picker);
    const list = picker.querySelector(".filter-pick__list");
    if (!list) return;
    list.classList.remove("filter-pick__list--end");
    if (list.getBoundingClientRect().right > document.documentElement.clientWidth - 16) {
      list.classList.add("filter-pick__list--end");
    }
  }, true);
  document.addEventListener("click", (e) => {
    if (!(e.target instanceof Node)) return;
    for (const p of pickers()) if (p.open && !p.contains(e.target)) p.open = false;
  });
  document.addEventListener("keydown", (e) => {
    if (e.key !== "Escape") return;
    for (const p of pickers()) {
      if (p.open) {
        p.open = false;
        p.querySelector("summary")?.focus();
      }
    }
  });
}
