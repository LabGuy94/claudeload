// This plugin intercepts fetch calls to anthropic.com, logging the request URL, body, and response (including streaming chunks) to a file named "claude-intercept.log" in the same directory as the executable.
const fs = require("fs");
const logFile = require("path").join(
  require("path").dirname(process.execPath),
  "claude-intercept.log",
);

function log(...args) {
  const line = args
    .map((a) => (typeof a === "string" ? a : JSON.stringify(a)))
    .join(" ");
  fs.appendFileSync(logFile, line + "\n");
}

const origFetch = globalThis.fetch;
globalThis.fetch = new Proxy(origFetch, {
  apply(target, thisArg, args) {
    const url =
      typeof args[0] === "string" ? args[0] : args[0]?.url || String(args[0]);
    const opts = args[1] || {};

    if (url.includes("anthropic.com") && !url.includes("data:")) {
      log("[REQUEST]", url);
      if (opts.body) {
        const body = typeof opts.body === "string" ? opts.body : "(non-string)";
        log("[BODY]", body.slice(0, 2000));
      }

      return Reflect.apply(target, thisArg, args).then((response) => {
        const clone = response.clone();

        // Handle SSE streaming
        const contentType = response.headers.get("content-type") || "";
        if (contentType.includes("text/event-stream")) {
          const reader = clone.body.getReader();
          const decoder = new TextDecoder();
          function pump() {
            reader
              .read()
              .then(({ done, value }) => {
                if (done) return;
                log("[STREAM CHUNK]", decoder.decode(value));
                pump();
              })
              .catch((e) => {
                log("[STREAM ERROR]", String(e));
              });
          }
          pump();
        } else {
          clone.text().then((text) => log("[RESPONSE]", text.slice(0, 2000)));
        }

        return response;
      });
    }

    return Reflect.apply(target, thisArg, args);
  },
});

log("[PAYLOAD LOADED]", new Date().toISOString());
