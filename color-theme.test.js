const assert = require("node:assert/strict");
const fs = require("node:fs");
const test = require("node:test");
const vm = require("node:vm");

const source = fs.readFileSync("public/color-theme.js", "utf8");
const palettes = {
  classic: ["#ff8552", "#297373", "#eee", "white", "#297373", "#222"],
  bubblegum: ["#FFA9A3", "#5C95FF", "#eeee", "white", "#4875c9", "#222"],
  pastel: ["#FFA9A3", "#7D938A", "#c9dfe650", "white", "#687a73", "#222"],
  artic: ["#7CDEDC", "#6C698D", "#E8C1C550", "white", "#6C698D", "#222"],
  forest: ["#B5DFCA", "#627264", "#A1CDA850", "white", "#627264", "#222"],
  purple: ["#40C9A2", "#B88C9E", "#8EB8E550", "white", "#906d7c", "#222"],
  blue: ["#DA95C4", "#DAE6F5", "#DA95C450", "#326273", "#DAE6F5", "#fff"],
  dark: ["#D5ACA9", "#9AC2C5", "#D5ACA950", "#424B54", "#9AC2C5", "#fff"],
};

function memoryStorage(value = null) {
  return {
    value,
    getItem(key) {
      assert.equal(key, "color-theme");
      return this.value;
    },
    setItem(key, next) {
      assert.equal(key, "color-theme");
      this.value = next;
    },
  };
}

function createPage({ storage = memoryStorage(), prefersDark = false, random = () => 0, readyState = "complete" } = {}) {
  const properties = new Map();
  let onReady;
  let onClick;
  const button = {
    hidden: true,
    addEventListener(event, callback) {
      if (event === "click") onClick = callback;
    },
  };
  const separator = { hidden: true };
  const document = {
    readyState,
    documentElement: { style: { setProperty: (key, value) => properties.set(key, value) } },
    getElementById(id) {
      if (id === "color-theme-toggle") return button;
      if (id === "color-theme-separator") return separator;
      return null;
    },
    addEventListener(event, callback) {
      if (event === "DOMContentLoaded") onReady = callback;
    },
  };
  const window = { localStorage: storage, matchMedia: () => ({ matches: prefersDark }) };
  vm.runInNewContext(source, { document, window, Math: { random, floor: Math.floor } });
  return {
    properties,
    button,
    separator,
    ready() {
      if (onReady) onReady();
    },
    click() {
      assert.ok(onClick, "theme button has a click handler");
      onClick();
    },
  };
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

test("preserve palettes and precomputed accessible colors", () => {
  const lightLinkColors = new Set();
  for (const [name, [main, second, third, background, link, text]] of Object.entries(palettes)) {
    const { properties } = createPage({ storage: memoryStorage(name) });
    assert.equal(properties.get("--c-main"), main, `${name} main palette value`);
    assert.equal(properties.get("--c-second"), second, `${name} second palette value`);
    assert.equal(properties.get("--c-third"), third, `${name} third palette value`);
    assert.equal(properties.get("--c-background"), background, `${name} background palette value`);
    assert.equal(properties.get("--c-link"), link, `${name} fixed link color`);
    assert.equal(properties.get("--c-text"), text, `${name} readable text color`);
    assert.equal(properties.get("--c-main-selection"), `${main}50`, `${name} selection color`);
    assert.ok(contrast(text, background) >= 4.5, `${name} text contrast`);
    assert.ok(contrast(link, background) >= 4.5, `${name} link contrast`);
    if (background === "white") lightLinkColors.add(link);
  }
  assert.equal(lightLinkColors.size, 6, "light theme links must look distinct");
});

test("click selects a different theme and reveals the control", () => {
  const storage = memoryStorage("classic");
  const page = createPage({ storage, random: () => 0, readyState: "loading" });
  assert.equal(page.button.hidden, true);
  assert.equal(page.separator.hidden, true);

  page.ready();
  assert.equal(page.button.hidden, false);
  assert.equal(page.separator.hidden, false);
  page.click();

  assert.equal(storage.value, "bubblegum");
  assert.equal(page.properties.get("--c-main"), palettes.bubblegum[0]);
  assert.notEqual(page.properties.get("--c-main"), palettes.classic[0]);
});

test("selected theme persists on a new page", () => {
  const storage = memoryStorage("classic");
  createPage({ storage, random: () => 0 }).click();
  const nextPage = createPage({ storage });

  assert.equal(storage.value, "bubblegum");
  assert.equal(nextPage.properties.get("--c-main"), palettes.bubblegum[0]);
  assert.equal(nextPage.properties.get("--c-link"), palettes.bubblegum[4]);
});

test("invalid stored theme values fall back to the light default", () => {
  for (const value of ["unknown", "__proto__"]) {
    const page = createPage({ storage: memoryStorage(value) });
    assert.equal(page.properties.get("--c-main"), palettes.classic[0]);
  }
});

test("system dark preference selects the dark default", () => {
  const page = createPage({ storage: memoryStorage(), prefersDark: true });
  assert.equal(page.properties.get("--c-main"), palettes.dark[0]);
  assert.equal(page.properties.get("--c-text"), palettes.dark[5]);
});

test("storage read and write errors do not block theme use", () => {
  let writes = 0;
  const storage = {
    getItem() {
      throw new Error("storage read failed");
    },
    setItem() {
      writes++;
      throw new Error("storage write failed");
    },
  };
  const page = createPage({ storage, prefersDark: true, random: () => 0 });
  assert.equal(page.properties.get("--c-main"), palettes.dark[0]);

  page.click();
  assert.equal(page.properties.get("--c-main"), palettes.classic[0]);
  assert.equal(writes, 1);
});
