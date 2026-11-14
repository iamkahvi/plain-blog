(() => {
  const themes = {
    classic: {
      "--c-main": "#ff8552",
      "--c-second": "#297373",
      "--c-third": "#eee",
      "--c-background": "white",
      "--c-link": "#297373",
      "--c-text": "#222",
    },
    bubblegum: {
      "--c-main": "#FFA9A3",
      "--c-second": "#5C95FF",
      "--c-third": "#eeee",
      "--c-background": "white",
      "--c-link": "#4875c9",
      "--c-text": "#222",
    },
    pastel: {
      "--c-main": "#FFA9A3",
      "--c-second": "#7D938A",
      "--c-third": "#c9dfe650",
      "--c-background": "white",
      "--c-link": "#687a73",
      "--c-text": "#222",
    },
    artic: {
      "--c-main": "#7CDEDC",
      "--c-second": "#6C698D",
      "--c-third": "#E8C1C550",
      "--c-background": "white",
      "--c-link": "#6C698D",
      "--c-text": "#222",
    },
    forest: {
      "--c-main": "#B5DFCA",
      "--c-second": "#627264",
      "--c-third": "#A1CDA850",
      "--c-background": "white",
      "--c-link": "#627264",
      "--c-text": "#222",
    },
    purple: {
      "--c-main": "#40C9A2",
      "--c-second": "#B88C9E",
      "--c-third": "#8EB8E550",
      "--c-background": "white",
      "--c-link": "#906d7c",
      "--c-text": "#222",
    },
    blue: {
      "--c-main": "#DA95C4",
      "--c-second": "#DAE6F5",
      "--c-third": "#DA95C450",
      "--c-background": "#326273",
      "--c-link": "#DAE6F5",
      "--c-text": "#fff",
    },
    dark: {
      "--c-main": "#D5ACA9",
      "--c-second": "#9AC2C5",
      "--c-third": "#D5ACA950",
      "--c-background": "#424B54",
      "--c-link": "#9AC2C5",
      "--c-text": "#fff",
    },
  };
  const names = Object.keys(themes);

  function initialTheme() {
    try {
      const stored = window.localStorage.getItem("color-theme");
      if (Object.prototype.hasOwnProperty.call(themes, stored)) return stored;
    } catch {
      // Storage can be unavailable in privacy-restricted contexts.
    }

    try {
      if (window.matchMedia("(prefers-color-scheme: dark)").matches) return "dark";
    } catch {
      // Use the light default when system preferences are unavailable.
    }
    return "classic";
  }

  function applyTheme(name) {
    const root = document.documentElement;
    const palette = themes[name];
    for (const [property, value] of Object.entries(palette)) {
      root.style.setProperty(property, value);
      if (property === "--c-main") {
        root.style.setProperty("--c-main-selection", `${value}50`);
      }
    }
  }

  let currentTheme = initialTheme();
  applyTheme(currentTheme);

  function enableButton() {
    const button = document.getElementById("color-theme-toggle");
    if (!button) return;

    button.addEventListener("click", () => {
      const alternatives = names.filter((name) => name !== currentTheme);
      currentTheme = alternatives[Math.floor(Math.random() * alternatives.length)];
      applyTheme(currentTheme);
      try {
        window.localStorage.setItem("color-theme", currentTheme);
      } catch {
        // The selected theme still applies when storage is unavailable.
      }
    });
    button.hidden = false;
    const separator = document.getElementById("color-theme-separator");
    if (separator) separator.hidden = false;
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", enableButton, { once: true });
  } else {
    enableButton();
  }
})();
