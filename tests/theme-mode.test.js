import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";
import vm from "node:vm";

const css = fs.readFileSync("public/style.css", "utf8");
const script = fs.readFileSync("public/theme-mode.js", "utf8");
const light = css.match(/^:root\s*\{([^}]*)\}/);
const dark = css.match(/@media\s*\(prefers-color-scheme:\s*dark\)\s*\{\s*:root:not\(\[data-theme="light"\]\)\s*\{([^}]*)\}\s*\}/);
const explicitDark = css.match(/:root\[data-theme="dark"\]\s*\{([^}]*)\}/);

function properties(block) {
  assert.ok(block, "theme CSS exists");
  return Object.fromEntries(Array.from(block[1].matchAll(/(--[\w-]+):\s*([^;]+);/g), ([, key, value]) => [key, value.trim()]));
}

function rgb(hex) {
  if (hex === "white") return [255, 255, 255];
  if (hex.length === 4) return [1, 2, 3].map((index) => parseInt(hex[index] + hex[index], 16));
  return [1, 3, 5].map((index) => parseInt(hex.slice(index, index + 2), 16));
}

function luminance(hex) {
  const channels = rgb(hex).map((value) => {
    const channel = value / 255;
    return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * channels[0] + 0.7152 * channels[1] + 0.0722 * channels[2];
}

function contrast(foreground, background) {
  const values = [luminance(foreground), luminance(background)].sort((a, b) => b - a);
  return (values[0] + 0.05) / (values[1] + 0.05);
}

test("pastel is the default and dark follows the dark device preference", () => {
  assert.deepEqual(properties(light), {
    "--c-main": "#FFA9A3",
    "--c-second": "#7D938A",
    "--c-third": "#c9dfe650",
    "--c-background": "white",
    "--c-main-selection": "#FFA9A350",
    "--c-text": "#222",
    "--c-link": "#687a73",
  });
  assert.deepEqual(properties(dark), {
    "--c-main": "#D5ACA9",
    "--c-second": "#9AC2C5",
    "--c-third": "#D5ACA950",
    "--c-background": "#424B54",
    "--c-main-selection": "#D5ACA950",
    "--c-text": "#fff",
    "--c-link": "#9AC2C5",
  });
  assert.deepEqual(properties(explicitDark), properties(dark));
});

test("both themes have sufficient text and link contrast", () => {
  for (const block of [light, dark]) {
    const palette = properties(block);
    assert.ok(contrast(palette["--c-text"], palette["--c-background"]) >= 4.5);
    assert.ok(contrast(palette["--c-link"], palette["--c-background"]) >= 4.5);
  }
});

function memoryStorage(value = null) {
  return {
    value,
    getItem(key) {
      assert.equal(key, "color-mode");
      return this.value;
    },
    setItem(key, value) {
      assert.equal(key, "color-mode");
      this.value = value;
    },
  };
}

function createPage({ storage = memoryStorage(), prefersDark = false, hasButton = true, readyState = "complete" } = {}) {
  let theme;
  let onReady;
  let onClick;
  let onChange;
  const button = {
    hidden: true,
    textContent: "",
    addEventListener(event, callback) {
      assert.equal(event, "click");
      onClick = callback;
    },
  };
  const deviceTheme = {
    matches: prefersDark,
    addEventListener(event, callback) {
      assert.equal(event, "change");
      onChange = callback;
    },
  };
  const document = {
    readyState,
    documentElement: {
      setAttribute(name, value) {
        assert.equal(name, "data-theme");
        theme = value;
      },
    },
    getElementById: (id) => id === "theme-toggle" && hasButton ? button : null,
    addEventListener(event, callback) {
      assert.equal(event, "DOMContentLoaded");
      onReady = callback;
    },
  };
  const window = {
    localStorage: storage,
    matchMedia(query) {
      assert.equal(query, "(prefers-color-scheme: dark)");
      return deviceTheme;
    },
  };
  vm.runInNewContext(script, { document, window });
  return {
    button,
    get theme() { return theme; },
    ready() { onReady(); },
    click() { onClick(); },
    setDeviceTheme(dark) {
      deviceTheme.matches = dark;
      if (onChange) onChange();
    },
  };
}

test("the button follows the device until the visitor selects a mode", () => {
  const storage = memoryStorage();
  const page = createPage({ storage, readyState: "loading" });
  assert.equal(page.button.hidden, true);
  assert.equal(page.theme, undefined);
  page.ready();
  assert.equal(page.button.hidden, false);
  assert.equal(page.button.textContent, "dark mode");
  page.setDeviceTheme(true);
  assert.equal(page.button.textContent, "light mode");
  assert.equal(page.theme, undefined);
  page.click();
  assert.equal(page.theme, "light");
  assert.equal(storage.value, "light");
  assert.equal(page.button.textContent, "dark mode");
  page.setDeviceTheme(false);
  page.setDeviceTheme(true);
  assert.equal(page.theme, "light");
  assert.equal(page.button.textContent, "dark mode");
  page.click();
  assert.equal(page.theme, "dark");
  assert.equal(storage.value, "dark");
});

test("saved modes apply before the page is ready and on pages without a button", () => {
  for (const mode of ["light", "dark"]) {
    const storage = memoryStorage(mode);
    const page = createPage({ storage, prefersDark: mode === "light", readyState: "loading" });
    assert.equal(page.theme, mode);
    page.ready();
    assert.equal(page.button.textContent, mode === "dark" ? "light mode" : "dark mode");
    assert.equal(createPage({ storage, hasButton: false }).theme, mode);
  }
});

test("invalid values and storage errors do not block theme use", () => {
  const invalid = createPage({ storage: memoryStorage("classic"), prefersDark: true });
  assert.equal(invalid.theme, undefined);
  assert.equal(invalid.button.textContent, "light mode");
  const storage = {
    getItem() { throw new Error("storage read failed"); },
    setItem() { throw new Error("storage write failed"); },
  };
  const page = createPage({ storage, prefersDark: true });
  page.click();
  assert.equal(page.theme, "light");
  assert.equal(page.button.textContent, "dark mode");
});
