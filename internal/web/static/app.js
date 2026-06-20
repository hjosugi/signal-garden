(() => {
  const body = document.body;
  const toggle = document.getElementById("crt-toggle");
  const stored = localStorage.getItem("signal-garden-crt");
  if (stored === "off") {
    body.classList.add("crt-off");
    if (toggle) {
      toggle.textContent = "crt:off";
      toggle.setAttribute("aria-pressed", "false");
    }
  }

  toggle?.addEventListener("click", () => {
    const off = body.classList.toggle("crt-off");
    localStorage.setItem("signal-garden-crt", off ? "off" : "on");
    toggle.textContent = off ? "crt:off" : "crt:on";
    toggle.setAttribute("aria-pressed", String(!off));
  });

  document.addEventListener("keydown", (event) => {
    const target = event.target;
    const typing = target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement || target instanceof HTMLSelectElement;
    if (event.key === "/" && !typing) {
      const search = document.getElementById("signal-search");
      if (search) {
        event.preventDefault();
        search.focus();
      }
    }
  });

  if ("EventSource" in window) {
    const stream = new EventSource("/events");
    stream.addEventListener("viewer_count", (event) => {
      try {
        const envelope = JSON.parse(event.data);
        const count = envelope.data?.count ?? 0;
        document.querySelectorAll("[data-viewer-count]").forEach((node) => {
          node.textContent = String(count);
        });
      } catch (_) {
        // Ignore malformed optional status events.
      }
    });
    stream.addEventListener("collection_started", () => {
      const state = document.getElementById("collection-state");
      if (state) state.textContent = "collecting";
    });
    stream.addEventListener("collection_finished", () => {
      const state = document.getElementById("collection-state");
      if (state) state.textContent = "idle";
    });
  }
})();
