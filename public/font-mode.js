(() => {
  const font = new URLSearchParams(window.location.search).get("font");
  if (!["baskerville", "geist", "times"].includes(font)) return;

  document.documentElement.setAttribute("data-font", font);

  function updateLinks() {
    for (const link of document.querySelectorAll("a[href]")) {
      let url;
      try {
        url = new URL(link.getAttribute("href"), document.baseURI);
      } catch {
        continue;
      }
      if ((url.protocol !== "http:" && url.protocol !== "https:") ||
          url.origin !== window.location.origin || url.searchParams.has("font")) continue;
      url.searchParams.set("font", font);
      link.setAttribute("href", url.href);
    }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", updateLinks, { once: true });
  } else {
    updateLinks();
  }
})();
