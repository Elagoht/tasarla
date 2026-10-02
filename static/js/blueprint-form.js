// blueprint-form.js: the create-board form turns off the parts the chosen
// blueprint does not hold, and recurring cards without card templates. The
// server applies the same rules, so the form works without this script.

const form = document.querySelector("[data-blueprint-form]");

if (form) {
  const boxes = [...form.querySelectorAll('input[type="checkbox"][name="include"]')];
  const box = (name) => boxes.find((b) => b.value === name);

  const update = () => {
    const chosen = form.querySelector('input[name="blueprint"]:checked');
    const has = new Set((chosen?.dataset.has ?? "").split(" ").filter(Boolean));
    for (const b of boxes) b.disabled = !has.has(b.value);
    const templates = box("templates");
    const recurring = box("recurring");
    if (templates && recurring && has.has("recurring")) {
      recurring.disabled = !templates.checked;
      if (!templates.checked) recurring.checked = false;
    }
  };

  form.addEventListener("change", update);
  update();
}
