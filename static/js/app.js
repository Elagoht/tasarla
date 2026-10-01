// The shell: the mobile menu, the account menu and fading toasts.

const shell = document.querySelector("[data-shell]");

if (shell) {
  const menu = shell.querySelector("[data-menu]");
  menu?.addEventListener("click", () => {
    const open = shell.classList.toggle("is-menu-open");
    menu.setAttribute("aria-expanded", String(open));
  });

  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && shell.classList.contains("is-menu-open")) {
      shell.classList.remove("is-menu-open");
      menu?.setAttribute("aria-expanded", "false");
      menu?.focus();
    }
  });

  document.addEventListener("click", (event) => {
    if (!shell.classList.contains("is-menu-open")) return;
    const target = event.target;
    if (target instanceof Element && !target.closest(".sidebar") && !target.closest("[data-menu]")) {
      shell.classList.remove("is-menu-open");
      menu?.setAttribute("aria-expanded", "false");
    }
  });
}

// Enter sends a one-line text box, such as a new card's title; Shift+Enter
// is a new line.
document.addEventListener("keydown", (e) => {
  const el = e.target;
  if (!(el instanceof HTMLTextAreaElement) || !el.matches("[data-submit-on-enter]") || el.closest("form[data-autosave]")) return;
  if (e.key === "Enter" && !e.shiftKey && !e.isComposing) {
    e.preventDefault();
    if (el.value.trim() !== "") el.form?.requestSubmit();
  }
});

// A section opened by the reader stays open when collage-live puts in a new
// copy of the page around it, which would otherwise drop its open attribute.
const kept = new Set();
document.addEventListener("toggle", (e) => {
  const d = e.target;
  if (!(d instanceof HTMLDetailsElement) || !d.dataset.keepOpen) return;
  if (d.open) kept.add(d.dataset.keepOpen);
  else kept.delete(d.dataset.keepOpen);
}, true);
document.addEventListener("collage:swap", (e) => {
  if (!(e.target instanceof Element)) return;
  for (const d of e.target.querySelectorAll("details[data-keep-open]")) {
    if (kept.has(d.dataset.keepOpen) && !d.open) d.open = true;
  }
});

// The account menu closes on a click elsewhere and on Escape.
const meMenu = document.querySelector("[data-me-menu]");
if (meMenu) {
  document.addEventListener("click", (e) => {
    if (meMenu.open && e.target instanceof Node && !meMenu.contains(e.target)) meMenu.open = false;
  });
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape" && meMenu.open) {
      meMenu.open = false;
      meMenu.querySelector("summary").focus();
    }
  });
}

// The link the reader follows grows into the title of the page it opens:
// both are named "target" for the view transition (base.css), this page's own
// title standing aside so the name stays unique. Tabs and toggles change the
// page under the same title, and are left out.
let followed = null;
document.addEventListener("click", (e) => {
  const link = e.target instanceof Element && e.target.closest("a[href]");
  followed = link && !link.matches(".tabs__tab, .segmented__item, [aria-pressed]") ? link : null;
}, true);

addEventListener("pageswap", (e) => {
  const url = e.activation?.entry?.url;
  if (!e.viewTransition || !url || !followed) return;
  const to = new URL(url), from = new URL(followed.href, location.href);
  if (to.origin !== from.origin || to.pathname !== from.pathname) return;
  for (const el of document.querySelectorAll(".page-head h1, .card-panel--page .cardp__title, .confirm-page h1, .solo__card h1")) {
    el.style.viewTransitionName = "none";
  }
  (followed.closest(".card") ?? followed).style.viewTransitionName = "target";
});

for (const toast of document.querySelectorAll("[data-toast]")) {
  setTimeout(() => {
    toast.classList.add("is-leaving");
    toast.addEventListener("transitionend", () => toast.remove(), { once: true });
    setTimeout(() => toast.remove(), 600);
  }, toast.classList.contains("toast--error") ? 9000 : 4500);
}
