(() => {
  const params = new URLSearchParams(window.location.search);
  if (!params.has("local")) return;

  function updateLinks() {
    for (const link of document.querySelectorAll("[data-newsletter-substack]")) {
      link.hidden = true;
    }
    for (const link of document.querySelectorAll("[data-newsletter-local]")) {
      link.hidden = false;
    }
    for (const link of document.querySelectorAll("a[href]")) {
      let url;
      try {
        url = new URL(link.getAttribute("href"), document.baseURI);
      } catch {
        continue;
      }
      if ((url.protocol !== "http:" && url.protocol !== "https:") ||
          url.origin !== window.location.origin || url.searchParams.has("local")) continue;
      url.searchParams.set("local", params.get("local"));
      link.setAttribute("href", url.href);
    }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", updateLinks, { once: true });
  } else {
    updateLinks();
  }
})();
