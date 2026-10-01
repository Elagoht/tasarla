// confirm.js: a destructive form asks first, in a dialog (spec §2.6). The
// server asks too, on a page of its own, when no script runs.

const template = document.getElementById("confirm-template");
let dialog = null;
let pending = null;

function ensureDialog() {
  if (dialog) return dialog;
  dialog = template.content.firstElementChild.cloneNode(true);
  document.body.append(dialog);
  dialog.addEventListener("close", () => {
    const form = pending;
    pending = null;
    if (!form || dialog.returnValue !== "yes") return;
    const input = document.createElement("input");
    input.type = "hidden";
    input.name = "confirm";
    input.value = "1";
    form.append(input);
    form.requestSubmit();
    // The submission has read the form; the next one on it asks again, even
    // if this one failed and the form stays on the page.
    input.remove();
  });
  return dialog;
}

// Capture: this runs before collage-live sends a data-collage-target form.
document.addEventListener("submit", (e) => {
  const form = e.target;
  if (!template || !(form instanceof HTMLFormElement) || !form.dataset.confirm) return;
  if (form.querySelector('input[name="confirm"]')) return;
  e.preventDefault();
  e.stopImmediatePropagation();
  const d = ensureDialog();
  d.querySelector("[data-confirm-question]").textContent = form.dataset.confirm;
  const hint = d.querySelector("[data-confirm-hint]");
  hint.textContent = form.dataset.confirmHint || hint.dataset.default;
  pending = form;
  d.returnValue = "";
  d.showModal();
  d.querySelector('button[value="no"]')?.focus();
}, true);
