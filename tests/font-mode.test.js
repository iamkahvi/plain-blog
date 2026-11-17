import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";
import vm from "node:vm";

const script = fs.readFileSync("public/font-mode.js", "utf8");

function createPage(address, hrefs = [], readyState = "loading") {
  const location = new URL(address);
  let font;
  let onReady;
  const links = hrefs.map((href) => ({
    href,
    getAttribute(name) {
      assert.equal(name, "href");
      return this.href;
    },
    setAttribute(name, value) {
      assert.equal(name, "href");
      this.href = value;
    },
  }));
  const document = {
    baseURI: location.href,
    readyState,
    documentElement: {
      setAttribute(name, value) {
        assert.equal(name, "data-font");
        font = value;
      },
    },
    querySelectorAll(selector) {
      assert.equal(selector, "a[href]");
      return links;
    },
    addEventListener(event, callback) {
      assert.equal(event, "DOMContentLoaded");
      onReady = callback;
    },
  };
  vm.runInNewContext(script, { document, window: { location }, URL, URLSearchParams });
  return { get font() { return font; }, links, ready() { onReady?.(); } };
}

test("valid fonts apply before the page is ready", () => {
  for (const font of ["baskerville", "geist", "times"]) {
    const page = createPage(`https://blog.example/?font=${font}`, ["/writing/"]);
    assert.equal(page.font, font);
    assert.equal(page.links[0].href, "/writing/");
    page.ready();
    assert.equal(page.links[0].href, `https://blog.example/writing/?font=${font}`);
  }
});

test("absent and invalid fonts use the default without changing links", () => {
  for (const query of ["", "?font=", "?font=other", "?font=GEIST"]) {
    const page = createPage(`https://blog.example/${query}`, ["/writing/"], "complete");
    assert.equal(page.font, undefined);
    assert.equal(page.links[0].href, "/writing/");
  }
});

test("internal links retain the font without changing other destinations", () => {
  const hrefs = [
    "/writing/?sort=new#top",
    "../projects/",
    "https://blog.example/books/",
    "#section",
    "/?font=times",
    "https://other.example/path",
    "//other.example/path",
    "mailto:hello@example.com",
    "javascript:void(0)",
    "blob:https://blog.example/id",
    "http://[invalid",
  ];
  const page = createPage("https://blog.example/writing/?font=geist", hrefs, "complete");
  assert.deepEqual(page.links.map((link) => link.href), [
    "https://blog.example/writing/?sort=new&font=geist#top",
    "https://blog.example/projects/?font=geist",
    "https://blog.example/books/?font=geist",
    "#section",
    "/?font=times",
    "https://other.example/path",
    "//other.example/path",
    "mailto:hello@example.com",
    "javascript:void(0)",
    "blob:https://blog.example/id",
    "http://[invalid",
  ]);
});
