const fs = require("fs");
const path = require("path");

const pluginDir = path.join(path.dirname(process.execPath), "claudeload-plugins");
if (fs.existsSync(pluginDir)) {
  for (const file of fs.readdirSync(pluginDir).sort().filter(f => f.endsWith(".js"))) {
    try {
      eval(fs.readFileSync(path.join(pluginDir, file), "utf8"));
    } catch (e) {}
  }
}
