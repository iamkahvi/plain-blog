import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";
import vm from "node:vm";

const newsletterScript = fs.readFileSync("public/newsletter-links.js", "utf8");
const fontScript = fs.readFileSync("public/font-mode.js", "utf8");

function createPage(address, entries, readyState = "loading", withFont = false) {
  const location = new URL(address);
  const callbacks = [];
  const links = entries.map(({ href, kind, hidden = false }) => ({
    href, kind, hidden,
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
    documentElement: { setAttribute() {} },
    querySelectorAll(selector) {
      if (selector === "a[href]") return links;
      if (selector === "[data-newsletter-substack]") return links.filter((link) => link.kind === "substack");
      if (selector === "[data-newsletter-local]") return links.filter((link) => link.kind === "local");
      throw new Error(`Unexpected selector: ${selector}`);
    },
    addEventListener(event, callback) {
      assert.equal(event, "DOMContentLoaded");
      callbacks.push(callback);
    },
  };
  const context = { document, window: { location }, URL, URLSearchParams };
  if (withFont) vm.runInNewContext(fontScript, context);
  vm.runInNewContext(newsletterScript, context);
  return { links, ready() { callbacks.forEach((callback) => callback()); } };
}

const entries = [
  { href: "https://example.com/p/news", kind: "substack" },
  { href: "/writing/news/", kind: "local", hidden: true },
  { href: "/writing/other/" },
];

test("Substack remains the default without the local parameter", () => {
  for (const query of ["", "?font=geist", "?locally=1"]) {
    const page = createPage(`https://blog.example/${query}`, entries, "complete");
    assert.deepEqual(page.links.map(({ href, hidden }) => ({ href, hidden })), [
      { href: "https://example.com/p/news", hidden: false },
      { href: "/writing/news/", hidden: true },
      { href: "/writing/other/", hidden: false },
    ]);
  }
});

test("the local parameter selects local links after the document is ready", () => {
  for (const query of ["?local", "?local=", "?local=false"]) {
    const page = createPage(`https://blog.example/${query}`, entries);
    assert.equal(page.links[0].hidden, false);
    assert.equal(page.links[1].hidden, true);
    page.ready();
    assert.equal(page.links[0].hidden, true);
    assert.equal(page.links[1].hidden, false);
    assert.equal(new URL(page.links[1].href).pathname, "/writing/news/");
    assert.equal(new URL(page.links[1].href).searchParams.get("local"), new URLSearchParams(query).get("local"));
    assert.equal(page.links[2].hidden, false);
    assert.equal(new URL(page.links[2].href).pathname, "/writing/other/");
  }
});

test("internal links retain the parameter without changing other destinations", () => {
  const hrefs = [
    "/writing/?sort=new#top", "../projects/", "https://blog.example/books/", "#section",
    "/?local=existing", "https://other.example/path", "//other.example/path",
    "mailto:hello@example.com", "javascript:void(0)", "blob:https://blog.example/id", "http://[invalid",
  ];
  const page = createPage("https://blog.example/writing/?local=yes", hrefs.map((href) => ({ href })), "complete");
  assert.deepEqual(page.links.map((link) => link.href), [
    "https://blog.example/writing/?sort=new&local=yes#top", "https://blog.example/projects/?local=yes",
    "https://blog.example/books/?local=yes", "#section", "/?local=existing",
    "https://other.example/path", "//other.example/path", "mailto:hello@example.com",
    "javascript:void(0)", "blob:https://blog.example/id", "http://[invalid",
  ]);
});

test("local and font preferences work together", () => {
  const page = createPage("https://blog.example/?local&font=geist", entries, "loading", true);
  page.ready();
  assert.equal(page.links[0].hidden, true);
  assert.equal(page.links[1].hidden, false);
  for (const link of page.links.slice(1)) {
    const url = new URL(link.href);
    assert.equal(url.searchParams.get("font"), "geist");
    assert.equal(url.searchParams.get("local"), "");
  }
});
