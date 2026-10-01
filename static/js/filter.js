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
const target = document.getElementById("board") || document.getElementById("gantt");
// The Gantt page's scale and grouping links carry the filter in their own
// URLs, made on the server: a filter changed there loads the page, so the
// links are made again; its text box is sent with Enter only.
const navigates = target?.id === "gantt";

function query() {
  const params = new URLSearchParams();
  for (const [k, v] of new FormData(bar)) {
    if (typeof v === "string" && v.trim() !== "") params.append(k, v.trim());
  }
  return params.toString();
}

function withQuery(url, q) {
  const base = url.split("?")[0];
  return q ? base + "?" + q : base;
}

let timer = 0;
function apply() {
  clearTimeout(timer);
  const q = query();
  history.replaceState(history.state, "", withQuery(location.pathname, q));
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
