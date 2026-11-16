import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";

const css = fs.readFileSync("style.css", "utf8");
const light = css.match(/^:root\s*\{([^}]*)\}/);
const dark = css.match(/@media\s*\(prefers-color-scheme:\s*dark\)\s*\{\s*:root\s*\{([^}]*)\}\s*\}/);

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
});

test("both themes have sufficient text and link contrast", () => {
  for (const block of [light, dark]) {
    const palette = properties(block);
    assert.ok(contrast(palette["--c-text"], palette["--c-background"]) >= 4.5);
    assert.ok(contrast(palette["--c-link"], palette["--c-background"]) >= 4.5);
  }
});
