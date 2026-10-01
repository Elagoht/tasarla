// columns-editor.js: the column table of the board settings (spec §2.3).
// Rows are reordered by dragging or with their arrows, added with "Add column",
// and saved together; the order fields are what a page without a script uses.

const form = document.querySelector("[data-columns-editor]");

if (form) {
  const body = form.querySelector("[data-col-rows]");
  const template = form.querySelector("[data-col-template]");
  const count = form.querySelector("[data-col-count]");
  const add = form.querySelector("[data-col-add]");
  const dirty = form.querySelector("[data-col-dirty]");
  const initial = new WeakMap();
  let submitting = false;

  const rows = () => [...body.querySelectorAll("[data-col-row]:not([data-col-template])")];

  // A row's state: every field's value, and whether each box is ticked.
  const state = (row) =>
    [...row.querySelectorAll("input")]
      .map((i) => (i.type === "checkbox" || i.type === "radio" ? `${i.name}=${i.checked}` : `${i.name}=${i.value}`))
      .join("&");

  const changes = () =>
    rows().filter((row) => {
      if (!initial.has(row)) return row.querySelector('input[name$="_name"]').value.trim() !== "";
      return initial.get(row) !== state(row);
    }).length;

  const renumber = () => {
    rows().forEach((row, i) => (row.querySelector("[data-col-order]").value = String(i)));
    template.querySelector("[data-col-order]").value = "999";
    refresh();
  };

  const refresh = () => {
    const n = changes();
    dirty.hidden = n === 0;
    dirty.textContent = dirty.dataset.template.replace("{count}", String(n));
  };

  // The blank row stays for a page without a script; here "Add column" adds rows.
  template.hidden = true;
  add.hidden = false;
  for (const row of rows()) initial.set(row, state(row));

  add.addEventListener("click", () => {
    const index = Number(count.value);
    const from = template.querySelector('input[name$="_id"]').name.match(/^col_(\d+)_/)[1];
    const row = template.cloneNode(true);
    row.hidden = false;
    row.removeAttribute("data-col-template");
    row.classList.remove("table__new");
    for (const input of row.querySelectorAll("input")) {
      input.name = input.name.replace(`col_${from}_`, `col_${index}_`);
      if (input.type === "radio") input.value = String(index);
      if (input.type !== "radio" && input.type !== "checkbox" && input.type !== "hidden") input.value = "";
      input.checked = false;
    }
    row.querySelector("[data-col-handle]").innerHTML = form.querySelector("[data-col-row] [data-col-handle]")?.innerHTML ?? "";
    body.insertBefore(row, template);
    count.value = String(index + 1);
    renumber();
    row.querySelector('input[name$="_name"]').focus();
  });

  body.addEventListener("click", (e) => {
    const button = e.target instanceof Element && e.target.closest("[data-col-up], [data-col-down]");
    if (!button) return;
    const row = button.closest("[data-col-row]");
    if (button.matches("[data-col-up]")) {
      const prev = row.previousElementSibling;
      if (prev) body.insertBefore(row, prev);
    } else {
      const next = row.nextElementSibling;
      if (next && next !== template) body.insertBefore(next, row);
    }
    button.focus();
    renumber();
  });

  form.addEventListener("input", refresh);
  form.addEventListener("change", refresh);
  form.addEventListener("submit", () => (submitting = true));
  window.addEventListener("beforeunload", (e) => {
    if (!submitting && changes() > 0) e.preventDefault();
  });

  if (window.Sortable) {
    window.Sortable.create(body, {
      handle: "[data-col-handle]",
      draggable: "tr:not([data-col-template])",
      animation: 150,
      ghostClass: "row--ghost",
      onEnd: renumber,
    });
  }
  renumber();
}
