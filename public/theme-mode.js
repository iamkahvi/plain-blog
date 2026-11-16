(() => {
  let preference = null;
  try {
    const saved = window.localStorage.getItem("color-mode");
    if (saved === "light" || saved === "dark") preference = saved;
  } catch {
    // Use the device preference when storage is unavailable.
  }

  if (preference) document.documentElement.setAttribute("data-theme", preference);

  let deviceTheme;
  try {
    deviceTheme = window.matchMedia("(prefers-color-scheme: dark)");
  } catch {
    // Use the light default when the device preference is unavailable.
  }

  function currentTheme() {
    return preference || (deviceTheme?.matches ? "dark" : "light");
  }

  function enableButton() {
    const button = document.getElementById("theme-toggle");
    if (!button) return;

    function updateLabel() {
      button.textContent = currentTheme() === "dark" ? "light mode" : "dark mode";
    }

    button.addEventListener("click", () => {
      preference = currentTheme() === "dark" ? "light" : "dark";
      document.documentElement.setAttribute("data-theme", preference);
      updateLabel();
      try {
        window.localStorage.setItem("color-mode", preference);
      } catch {
        // The selected mode still applies on this page.
      }
    });
    deviceTheme?.addEventListener?.("change", updateLabel);
    updateLabel();
    button.hidden = false;
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", enableButton, { once: true });
  } else {
    enableButton();
  }
})();
