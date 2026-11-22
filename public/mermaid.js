const blocks = document.querySelectorAll("main pre > code.language-mermaid");

if (blocks.length) {
  try {
    const { mount } = await import("./vendor/pele/1.13.0/pele.js");
    for (const code of blocks) {
      const source = code.parentElement;
      const diagram = document.createElement("div");
      diagram.className = "mermaid-diagram";
      source.before(diagram);
      mount(diagram, code.textContent, {
        onRender() { source.hidden = true; },
        onError() { source.hidden = false; },
      });
    }
  } catch (error) {
    console.warn("Mermaid diagrams could not load.", error);
  }
}
