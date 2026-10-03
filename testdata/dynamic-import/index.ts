export {};
async function liveLoader() { return import("./plugin"); }
liveLoader();
const pluginName = "registeredPlugin";
function registeredPlugin() {}
