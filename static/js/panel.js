// panel.js: the card panel, on the board's drawer and on the card page.
// Each field saves itself when it changes (spec §2.2); the sections are tabs.

const TABS = ["details", "comments", "activity"];

function tabFromHash() {
  const m = location.hash.match(/^#card-(\w+)$/);
  return m && TABS.includes(m[1]) ? m[1] : "details";
}

// The open tab lives on the panel element itself, which collage-live keeps
// while it patches what is inside.
function initPanel(panel) {
  if (!panel.dataset.tab) panel.dataset.tab = tabFromHash();
}

// Saves go one at a time: each carries the card's version, which only the
// answer to the one before brings. A field changed meanwhile waits its turn.
let inflight = null;
const queue = [];
let timer = 0;

function save(form) {
  if (inflight) {
    if (form !== inflight && !queue.includes(form)) queue.push(form);
    return;
  }
  inflight = form;
  clearTimeout(timer);
  // An answer identical to the one before puts nothing in and says nothing.
  timer = setTimeout(next, 8000);
  form.requestSubmit();
}

function next() {
  clearTimeout(timer);
  inflight = null;
  const form = queue.shift();
  if (form && form.isConnected) save(form);
}

// What the server answered is what a field shows, except the one being typed
// into and those still waiting to be saved: a field the reader changed keeps
// its own value through a patch, so after every answer each is set back to
// what the page now says.
function syncFields(panel) {
  for (const form of panel.querySelectorAll("form[data-autosave]")) {
    if (queue.includes(form)) continue;
    for (const el of form.elements) {
      if (el === document.activeElement) continue;
      if (el instanceof HTMLSelectElement) {
        for (const o of el.options) o.selected = o.defaultSelected;
      } else if (el instanceof HTMLInputElement && (el.type === "checkbox" || el.type === "radio")) {
        el.checked = el.defaultChecked;
      } else if (el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement) {
        el.value = el.defaultValue;
      }
    }
  }
  if (!panel.querySelector(".desc__form textarea:focus")) delete panel.dataset.editing;
}

document.addEventListener("change", (e) => {
  const el = e.target;
  if (!(el instanceof Element)) return;
  const form = el.closest("form[data-autosave]");
  if (form) save(form);
});

// Enter in a one-line field saves it; Shift+Enter is a new line where one is
// allowed.
document.addEventListener("keydown", (e) => {
  const el = e.target;
  if (!(el instanceof HTMLTextAreaElement) || !el.matches(".cardp__title")) return;
  if (e.key === "Enter" && !e.shiftKey && !e.isComposing) {
    e.preventDefault();
    el.blur();
  } else if (e.key === "Escape") {
    // Escape takes the title back, rather than closing the panel.
    e.preventDefault();
    e.stopPropagation();
    el.value = el.defaultValue;
    el.blur();
  }
});

document.addEventListener("click", (e) => {
  if (!(e.target instanceof Element)) return;
  const panel = e.target.closest("#card-panel");
  if (!panel) return;
  const tab = e.target.closest("[data-tab-link]");
  if (tab) {
    e.preventDefault();
    panel.dataset.tab = tab.dataset.tabLink;
    history.replaceState(history.state, "", "#card-" + tab.dataset.tabLink);
    return;
  }
  // The description shows as text; a click on it, not on a link in it, edits it.
  const desc = e.target.closest("[data-desc-open]");
  if (desc && !e.target.closest("a")) {
    panel.dataset.editing = "description";
    const area = panel.querySelector(".desc__form textarea");
    area?.focus();
  }
});

document.addEventListener("focusout", (e) => {
  const el = e.target;
  if (!(el instanceof HTMLTextAreaElement) || !el.closest(".desc__form")) return;
  const panel = el.closest("#card-panel");
  // Left unchanged: back to the text. Changed: the save's answer puts it back.
  if (panel && el.value === el.defaultValue) delete panel.dataset.editing;
});

// The answer to a save marks the field saved; the mark is kept on the panel,
// which collage-live leaves alone, so the push that follows does not take it
// away before it is seen.
let savedTimer = 0;
function markSaved(panel) {
  const just = panel.querySelector("[data-just-saved]");
  if (!just) return;
  panel.dataset.saved = just.dataset.savedFor;
  clearTimeout(savedTimer);
  savedTimer = setTimeout(() => delete panel.dataset.saved, 2600);
}

document.addEventListener("collage:swap", (e) => {
  const panel = e.target;
  if (panel instanceof HTMLElement && panel.id === "card-panel") {
    initPanel(panel);
    syncFields(panel);
    markSaved(panel);
    next();
  }
});

// A save that failed outright leaves the panel as it was, marked stale.
document.addEventListener("collage:stale", (e) => {
  const panel = e.target;
  if (panel instanceof HTMLElement && panel.id === "card-panel") next();
});

document.addEventListener("card:panel", (e) => {
  if (e.target instanceof HTMLElement) initPanel(e.target);
});

const page = document.getElementById("card-panel");
if (page) initPanel(page);
