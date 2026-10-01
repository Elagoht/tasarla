// rules.js: a condition's own fields show only for the condition chosen — the
// count for attachments, the labels for "one of these labels". Without a
// script they all show, and the server takes what the condition needs.

for (const form of document.querySelectorAll("form[data-rule-condition]")) {
  const kind = form.querySelector("[data-rule-kind]");
  const show = () => {
    for (const el of form.querySelectorAll("[data-rule-for]")) el.hidden = el.dataset.ruleFor !== kind.value;
  };
  kind.addEventListener("change", show);
  show();
}
