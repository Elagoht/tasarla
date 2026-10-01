// gantt.js: dates are changed on the chart. Dragging a bar moves both its
// dates; dragging its ends moves one; a milestone moves its one date. The arrow
// keys do the same on a focused bar: alone they move it a day, with Shift the
// due date, with Alt the start date. Each change is saved as the card's dates
// together, at the version the chart was drawn from.

const container = document.getElementById("gantt");

const live = () => window.collageLive;
const csrf = () => document.querySelector('input[name="_csrf"]')?.value ?? "";

// Dates are days: "YYYY-MM-DD" in, the same out.
const parse = (s) => (s ? new Date(s + "T00:00:00Z") : null);
const shift = (d, days) => (d ? new Date(d.getTime() + days * 86400000) : null);
const format = (d) => (d ? d.toISOString().slice(0, 10) : "");

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

// The new dates of a bar after it moved by days, in mode: "move", "start" or "end".
function moved(bar, mode, days) {
  let start = parse(bar.dataset.start);
  let due = parse(bar.dataset.due);
  if (mode === "move") return [shift(start, days), shift(due, days)];
  if (mode === "start") {
    start = shift(start, days);
    if (due && start > due) start = due;
    return [start, due];
  }
  due = shift(due, days);
  if (start && due < start) due = start;
  return [start, due];
}

// Draws a bar as it would be after moving by days, without saving.
function preview(bar, mode, days, dw) {
  const rect = bar.querySelector(".gantt__rect");
  if (!rect || mode === "move") {
    bar.setAttribute("transform", `translate(${days * dw} 0)`);
    return;
  }
  const x0 = Number(rect.dataset.x ?? rect.getAttribute("x"));
  const w0 = Number(rect.dataset.w ?? rect.getAttribute("width"));
  rect.dataset.x = String(x0);
  rect.dataset.w = String(w0);
  const [start, due] = moved(bar, mode, days);
  const span = Math.round((due - start) / 86400000) + 1;
  const x = mode === "start" ? x0 + w0 - span * dw : x0;
  rect.setAttribute("x", String(x));
  rect.setAttribute("width", String(span * dw));
  bar.querySelector(".gantt__handle--start")?.setAttribute("x", String(x));
  bar.querySelector(".gantt__handle--end")?.setAttribute("x", String(x + span * dw - 8));
}

async function save(bar, mode, days) {
  const [start, due] = moved(bar, mode, days);
  const frame = bar.closest("[data-gantt]");
  const body = new FormData();
  body.set("op", "set_dates");
  body.set("start", format(start));
  body.set("due", format(due));
  body.set("expected_version", bar.dataset.version);
  body.set("_csrf", csrf());
  bar.classList.add("is-saving");
  focusAfter = bar.dataset.card;
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
    // The chart as it now is: saved, or put back.
    live()?.refresh(container);
  }
}

// The bar a save was about is focused again once the chart is redrawn.
let focusAfter = null;
container?.addEventListener("collage:swap", () => {
  if (!focusAfter) return;
  container.querySelector(`.gantt__bar[data-card="${CSS.escape(focusAfter)}"]`)?.focus();
  focusAfter = null;
});

let drag = null;
let dragged = false;

container?.addEventListener("pointerdown", (e) => {
  const bar = e.target instanceof Element && e.target.closest(".gantt__bar");
  const frame = bar?.closest("[data-gantt]");
  if (!bar || !frame?.hasAttribute("data-editable") || e.button !== 0) return;
  const handle = e.target.closest("[data-handle]")?.dataset.handle;
  drag = { bar, mode: handle ?? "move", x: e.clientX, days: 0, dw: Number(frame.dataset.dayWidth), id: e.pointerId };
  dragged = false;
});

container?.addEventListener("pointermove", (e) => {
  if (!drag || e.pointerId !== drag.id) return;
  const dx = e.clientX - drag.x;
  if (!dragged && Math.abs(dx) < 4) return;
  if (!dragged) {
    dragged = true;
    live()?.pause(container);
    drag.bar.classList.add("is-dragging");
    drag.bar.setPointerCapture?.(e.pointerId);
  }
  e.preventDefault();
  drag.days = Math.round(dx / drag.dw);
  preview(drag.bar, drag.mode, drag.days, drag.dw);
});

function endDrag(e) {
  if (!drag || e.pointerId !== drag.id) return;
  const d = drag;
  drag = null;
  if (!dragged) return;
  d.bar.classList.remove("is-dragging");
  if (d.days === 0) {
    preview(d.bar, d.mode, 0, d.dw);
    live()?.resume(container);
    return;
  }
  save(d.bar, d.mode, d.days);
}
container?.addEventListener("pointerup", endDrag);
container?.addEventListener("pointercancel", endDrag);

// A drag is not a click: the bar's link opens the card only when not dragged.
container?.addEventListener("click", (e) => {
  if (dragged && e.target instanceof Element && e.target.closest(".gantt__bar")) {
    e.preventDefault();
    dragged = false;
  }
}, true);

// Keys: changes add up while they are pressed, and save a moment after.
let pending = null;
container?.addEventListener("keydown", (e) => {
  const bar = e.target instanceof Element && e.target.closest(".gantt__bar");
  const frame = bar?.closest("[data-gantt]");
  if (!bar || !frame?.hasAttribute("data-editable") || (e.key !== "ArrowLeft" && e.key !== "ArrowRight")) return;
  e.preventDefault();
  const step = e.key === "ArrowLeft" ? -1 : 1;
  const isMilestone = bar.classList.contains("is-milestone");
  const mode = isMilestone ? "move" : e.shiftKey ? "end" : e.altKey ? "start" : "move";
  if (!pending || pending.bar !== bar || pending.mode !== mode) {
    if (pending) clearTimeout(pending.timer);
    else live()?.pause(container);
    pending = { bar, mode, days: 0, dw: Number(frame.dataset.dayWidth) };
  }
  pending.days += step;
  preview(bar, mode, pending.days, pending.dw);
  clearTimeout(pending.timer);
  const p = pending;
  p.timer = setTimeout(() => {
    pending = null;
    if (p.days === 0) {
      live()?.resume(container);
      return;
    }
    save(p.bar, p.mode, p.days);
  }, 600);
});
