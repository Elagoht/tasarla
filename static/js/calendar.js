// calendar.js: dates are changed on the calendar. Dragging a card to another
// day moves both its dates by as many days; the arrow keys on a focused card do
// the same, a day left or right, a week up or down. Each change is saved as the
// card's dates together, at the version the grid was drawn from. Clicking a
// day's empty space opens its new-card field.

const container = document.getElementById("calendar");

const live = () => window.collageLive;
const csrf = () => document.querySelector('input[name="_csrf"]')?.value ?? "";

// Dates are days: "YYYY-MM-DD" in, the same out.
const parse = (s) => (s ? new Date(s + "T00:00:00Z") : null);
const shift = (d, days) => (d ? new Date(d.getTime() + days * 86400000) : null);
const format = (d) => (d ? d.toISOString().slice(0, 10) : "");
const daysBetween = (a, b) => Math.round((parse(b) - parse(a)) / 86400000);

function toast(text, error) {
  const host = document.querySelector("[data-toasts]");
  if (!host) return;
  const p = document.createElement("p");
  p.className = "toast" + (error ? " toast--error" : "");
  p.textContent = text;
  host.append(p);
  setTimeout(() => {
    p.classList.add("is-leaving");
    setTimeout(() => p.remove(), 300);
  }, error ? 6000 : 2500);
}

const editable = () => container?.querySelector("[data-cal]")?.hasAttribute("data-editable");

// The day under a point, through the bars drawn over it.
const dayAt = (x, y) => document.elementsFromPoint(x, y).find((el) => el.matches?.(".cal__day[data-date]")) ?? null;

let focusAfter = null;

async function save(bar, days) {
  const frame = bar.closest("[data-cal]");
  const body = new FormData();
  body.set("op", "set_dates");
  body.set("start", format(shift(parse(bar.dataset.start), days)));
  body.set("due", format(shift(parse(bar.dataset.due), days)));
  body.set("expected_version", bar.dataset.version);
  body.set("_csrf", csrf());
  bar.classList.add("is-saving");
  focusAfter = document.activeElement?.closest?.(".cal__bar")?.dataset.card ?? bar.dataset.card;
  try {
    const res = await fetch(bar.dataset.url, {
      method: "POST",
      body,
      headers: { "Collage-Fetch": "1", "X-CSRF-Token": csrf() },
      credentials: "same-origin",
    });
    if (res.status === 409) toast(frame.dataset.errorConflict, true);
    else if (res.status === 422) toast(frame.dataset.errorOrder, true);
    else if (!res.ok) toast(frame.dataset.error, true);
  } catch {
    toast(frame.dataset.error, true);
  } finally {
    live()?.resume(container);
    // The grid as it now is: saved, or put back.
    live()?.refresh(container);
  }
}

// The card a save was about is focused again once the grid is redrawn.
container?.addEventListener("collage:swap", () => {
  if (!focusAfter) return;
  container.querySelector(`.cal__bar[data-card="${CSS.escape(focusAfter)}"]`)?.focus();
  focusAfter = null;
});

// Dragging: the card is picked up on the day under the pointer and put down on
// the day under it at the end; the move is the days between the two.
let drag = null;
let dragged = false;

const clearDrop = () => container?.querySelectorAll(".cal__day.is-drop").forEach((d) => d.classList.remove("is-drop"));

container?.addEventListener("pointerdown", (e) => {
  const bar = e.target instanceof Element && e.target.closest(".cal__bar");
  if (!bar || !editable() || e.button !== 0) return;
  const from = dayAt(e.clientX, e.clientY);
  if (!from) return;
  drag = { bar, from: from.dataset.date, x: e.clientX, y: e.clientY, id: e.pointerId, over: from };
  dragged = false;
});

container?.addEventListener("pointermove", (e) => {
  if (!drag || e.pointerId !== drag.id) return;
  if (!dragged && Math.hypot(e.clientX - drag.x, e.clientY - drag.y) < 4) return;
  if (!dragged) {
    dragged = true;
    live()?.pause(container);
    drag.bar.classList.add("is-dragging");
    drag.bar.setPointerCapture?.(e.pointerId);
  }
  e.preventDefault();
  const over = dayAt(e.clientX, e.clientY);
  if (over && over !== drag.over) {
    clearDrop();
    over.classList.add("is-drop");
    drag.over = over;
  }
});

function endDrag(e) {
  if (!drag || e.pointerId !== drag.id) return;
  const d = drag;
  drag = null;
  if (!dragged) return;
  d.bar.classList.remove("is-dragging");
  clearDrop();
  const days = d.over ? daysBetween(d.from, d.over.dataset.date) : 0;
  if (days === 0 || e.type === "pointercancel") {
    live()?.resume(container);
    return;
  }
  save(d.bar, days);
}
container?.addEventListener("pointerup", endDrag);
container?.addEventListener("pointercancel", endDrag);

// A drag is not a click: the card's link opens it only when not dragged.
container?.addEventListener("click", (e) => {
  if (dragged && e.target instanceof Element && e.target.closest(".cal__bar")) {
    e.preventDefault();
    dragged = false;
  }
}, true);

// Keys: moves add up while they are pressed, and save a moment after.
const steps = { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -7, ArrowDown: 7 };
let pending = null;
container?.addEventListener("keydown", (e) => {
  const bar = e.target instanceof Element && e.target.closest(".cal__bar");
  if (!bar || !editable() || !(e.key in steps)) return;
  e.preventDefault();
  if (!pending || pending.bar !== bar) {
    // A move still pending on another card is saved now, not dropped; each
    // pending move holds its own pause.
    if (pending) {
      clearTimeout(pending.timer);
      clearDrop();
      if (pending.days !== 0) save(pending.bar, pending.days);
      else live()?.resume(container);
    }
    live()?.pause(container);
    pending = { bar, days: 0 };
  }
  pending.days += steps[e.key];
  // Where the card will land, shown on its day.
  clearDrop();
  const to = format(shift(parse(bar.dataset.due || bar.dataset.start), pending.days));
  container.querySelector(`.cal__day[data-date="${to}"]`)?.classList.add("is-drop");
  clearTimeout(pending.timer);
  const p = pending;
  p.timer = setTimeout(() => {
    pending = null;
    clearDrop();
    if (p.days === 0) {
      live()?.resume(container);
      return;
    }
    save(p.bar, p.days);
  }, 600);
});

// While a day's new-card field is open the grid holds still, so a push from
// someone else's change does not wipe what is being typed. One pause for all
// the fields: the DOM says whether one is open.
let adding = false;
function holdWhileAdding() {
  const open = container?.querySelector("[data-cal-add][open]") != null;
  if (open === adding) return;
  adding = open;
  if (open) live()?.pause(container);
  else live()?.resume(container);
}

// New cards: a click on a day's empty space opens its field; Escape closes it.
// The form is sent as it is, and the grid comes back with the card or the reason
// it was refused.
container?.addEventListener("click", (e) => {
  if (!(e.target instanceof Element) || !editable()) return;
  if (e.target.closest("a, summary, form, details, button")) return;
  const dayEl = e.target.closest(".cal__day");
  const add = dayEl?.querySelector("[data-cal-add]");
  if (!add) return;
  container.querySelectorAll("[data-cal-add][open]").forEach((d) => d !== add && d.removeAttribute("open"));
  add.setAttribute("open", "");
  add.querySelector('input[name="title"]')?.focus();
});

container?.addEventListener("toggle", (e) => {
  const add = e.target;
  if (!(add instanceof HTMLDetailsElement) || !add.matches("[data-cal-add]")) return;
  if (add.open) add.querySelector('input[name="title"]')?.focus();
  holdWhileAdding();
}, true);

// A redraw may take an open field away with it.
container?.addEventListener("collage:swap", holdWhileAdding);

container?.addEventListener("keydown", (e) => {
  if (e.key !== "Escape") return;
  const add = e.target instanceof Element && e.target.closest("[data-cal-add]");
  if (!add) return;
  add.removeAttribute("open");
  add.querySelector("summary")?.focus();
  holdWhileAdding();
});

container?.addEventListener("submit", async (e) => {
  const form = e.target;
  const add = form instanceof HTMLFormElement && form.closest("[data-cal-add]");
  if (!add) return;
  e.preventDefault();
  const frame = form.closest("[data-cal]");
  try {
    const res = await fetch(form.action, {
      method: "POST",
      body: new FormData(form),
      headers: { "Collage-Fetch": "1", "X-CSRF-Token": csrf() },
      credentials: "same-origin",
    });
    if (res.status === 422) {
      const doc = new DOMParser().parseFromString(await res.text(), "text/html");
      const reasons = [...doc.querySelectorAll("[data-cal-alert] li, .field__error")].map((li) => li.textContent.trim());
      toast(reasons.join(" ") || frame.dataset.error, true);
      return;
    }
    if (!res.ok) toast(frame.dataset.error, true);
  } catch {
    toast(frame.dataset.error, true);
    return;
  }
  // Saved: the field closes, the grid moves again and shows the card.
  add.removeAttribute("open");
  holdWhileAdding();
  live()?.refresh(container);
});
