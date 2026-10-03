export {};
function registerPlugin() { return 1; }
const unused = registerPlugin();
function computedKey() { return "method"; }
class RiskyClass { [computedKey()]() {} }
