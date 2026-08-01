import type { KeyboardEvent } from "react";

export function moveRovingFocus(event: KeyboardEvent<HTMLElement>) {
  const supported = ["ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown", "Home", "End"];
  if (!supported.includes(event.key)) return;
  const container = event.currentTarget;
  const items = Array.from(container.querySelectorAll<HTMLElement>("button:not(:disabled)"));
  const current = items.indexOf(document.activeElement as HTMLElement);
  if (current < 0 || items.length === 0) return;
  event.preventDefault();
  let next = current;
  if (event.key === "Home") next = 0;
  else if (event.key === "End") next = items.length - 1;
  else if (event.key === "ArrowLeft" || event.key === "ArrowUp") next = (current - 1 + items.length) % items.length;
  else next = (current + 1) % items.length;
  items[next].focus();
  items[next].click();
}
