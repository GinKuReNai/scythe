import readline from "node:readline";
import { pathToFileURL } from "node:url";
import { loadProject } from "./program.js";
import { discover } from "./symbols.js";
import { references } from "./references.js";
import { emit, type Request } from "./protocol.js";
export async function analyze(
  request: Request,
  out: NodeJS.WritableStream = process.stdout,
) {
  if (
    request.protocolVersion !== 1 ||
    request.method !== "analyze" ||
    typeof request.root !== "string" ||
    !Array.isArray(request.exclude) ||
    request.exclude.some((x) => typeof x !== "string")
  )
    throw new Error("Invalid analyzer request (protocol version 1 required)");
  const project = loadProject(request);
  const symbols = discover(project);
  const graph = references(project, symbols);
  await emit(out as import("node:stream").Writable, "project", {
    root_dir: project.root,
    discovered_files: project.discoveredFiles,
    source_roots: project.sourceRoots,
    tsconfig_path: project.configPath,
    framework: project.framework,
    files: project.files.length,
    complete: project.warnings.length === 0,
    warnings: project.warnings,
    dynamic_risk: graph.dynamicRisk,
  });
  for (const symbol of symbols.records)
    await emit(out as import("node:stream").Writable, "symbol", symbol);
  for (const edge of graph.edges)
    await emit(out as import("node:stream").Writable, "edge", edge);
  for (const id of graph.roots)
    await emit(out as import("node:stream").Writable, "root", { id });
  await emit(out as import("node:stream").Writable, "done", {});
}
async function main() {
  const lines = readline.createInterface({
    input: process.stdin,
    crlfDelay: Infinity,
  });
  let count = 0;
  for await (const line of lines) {
    if (++count > 1)
      throw new Error("Only one request per process is supported");
    await analyze(JSON.parse(line));
  }
  if (count !== 1) throw new Error("Missing analyzer request");
}
if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href)
  main().catch((error) => {
    console.error(
      `TypeScript analyzer: ${error instanceof Error ? error.message : "analysis failed"}`,
    );
    process.exitCode = 1;
  });
