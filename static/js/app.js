// The shell: collapsing the sidebar, the mobile menu and fading toasts.

const shell = document.querySelector("[data-shell]");

function remembered() {
  try {
    return localStorage.getItem("sidebar") === "collapsed";
  } catch {
    return false;
  }
}

function remember(collapsed) {
  try {
    localStorage.setItem("sidebar", collapsed ? "collapsed" : "open");
  } catch {
    // Storage can be off; the sidebar then simply opens each time.
  }
}

if (shell) {
  if (remembered()) shell.classList.add("is-collapsed");

  shell.querySelector("[data-collapse]")?.addEventListener("click", (event) => {
    const collapsed = shell.classList.toggle("is-collapsed");
    event.currentTarget.setAttribute("aria-expanded", String(!collapsed));
    remember(collapsed);
  });

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

for (const toast of document.querySelectorAll("[data-toast]")) {
  setTimeout(() => {
    toast.classList.add("is-leaving");
    toast.addEventListener("transitionend", () => toast.remove(), { once: true });
    setTimeout(() => toast.remove(), 600);
  }, toast.classList.contains("toast--error") ? 9000 : 4500);
}
