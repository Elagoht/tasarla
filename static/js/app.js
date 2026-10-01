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

for (const toast of document.querySelectorAll("[data-toast]")) {
  setTimeout(() => {
    toast.classList.add("is-leaving");
    toast.addEventListener("transitionend", () => toast.remove(), { once: true });
    setTimeout(() => toast.remove(), 600);
  }, toast.classList.contains("toast--error") ? 9000 : 4500);
}
