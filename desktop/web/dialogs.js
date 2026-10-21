import { escape } from "./ui.js";

export function confirmAction(options) {
  return new Promise((resolve) => {
    const trigger = document.activeElement;
    const dialog = document.createElement("dialog");
    dialog.className = "confirm-dialog";
    dialog.setAttribute("aria-labelledby", "confirm-title");
    dialog.innerHTML = `<form method="dialog"><h2 id="confirm-title">${escape(options.title)}</h2><p>${escape(options.message)}</p><div class="form-footer"><button class="button" value="cancel" autofocus>取消</button><button class="button danger-primary" value="confirm">${escape(options.action)}</button></div></form>`;
    dialog.addEventListener("close", () => {
      const confirmed = dialog.returnValue === "confirm";
      dialog.remove();
      trigger?.focus({ preventScroll: true });
      resolve(confirmed);
    }, { once: true });
    document.body.append(dialog);
    dialog.showModal();
  });
}
