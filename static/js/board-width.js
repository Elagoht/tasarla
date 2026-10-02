// board-width.js: the board keeps the reading width of the other pages; this
// button lets it take the whole width. The choice goes into a cookie, so the
// server draws the next page at that width from the start; the width itself
// animates in CSS.

const button = document.querySelector("[data-board-width]");
const main = document.getElementById("main");

if (button && main) {
  const label = button.querySelector("[data-board-width-label]");
  button.hidden = false;
  button.addEventListener("click", () => {
    const wide = main.classList.toggle("is-expanded");
    button.setAttribute("aria-pressed", String(wide));
    if (label) label.textContent = (wide ? button.dataset.labelOn : button.dataset.labelOff) ?? "";
    document.cookie = wide
      ? "board_wide=1; path=/; max-age=31536000; samesite=lax"
      : "board_wide=; path=/; max-age=0; samesite=lax";
  });
}
