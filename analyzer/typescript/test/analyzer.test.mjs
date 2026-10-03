import test from "node:test";
import assert from "node:assert/strict";
import path from "node:path";
import fs from "node:fs";
import os from "node:os";
import { Writable } from "node:stream";
import { analyze } from "../dist/index.js";
import { glob } from "../dist/program.js";
const fixture = (name) => path.resolve("../../testdata", name);
async function run(root, exclude = []) {
  let output = "";
  const stream = new Writable({
    write(chunk, _encoding, done) {
      output += chunk;
      done();
    },
  });
  await analyze(
    { protocolVersion: 1, method: "analyze", root, exclude },
    stream,
  );
  const records = output.trim().split("\n").map(JSON.parse);
  assert.ok(records.every((r) => r.protocolVersion === 1));
  assert.equal(records.at(-1).type, "done");
  const symbols = records.filter((r) => r.type === "symbol").map((r) => r.data);
  const edges = records.filter((r) => r.type === "edge").map((r) => r.data);
  const roots = records.filter((r) => r.type === "root").map((r) => r.data.id);
  const reached = new Set();
  const stack = [...roots];
  while (stack.length) {
    const id = stack.pop();
    if (reached.has(id)) continue;
    reached.add(id);
    for (const e of edges) if (e.from === id) stack.push(e.to);
  }
  return {
    symbols,
    edges,
    reached,
    project: records[0].data,
    byName: (name) => symbols.find((s) => s.name === name),
  };
}
test("dead subgraphs are unreachable; module calls remain live", async () => {
  const r = await run(fixture("basic"));
  assert.equal(r.project.complete, true);
  assert.equal(r.symbols.length, 6);
  for (const name of [
    "deadFunction",
    "deadA",
    "deadB",
    "unusedLiteral",
    "UnusedClass",
  ])
    assert.ok(!r.reached.has(r.byName(name).id), name);
  assert.ok(r.reached.has(r.byName("liveFunction").id));
  assert.ok(
    r.edges.some(
      (e) => e.from === r.byName("deadA").id && e.to === r.byName("deadB").id,
    ),
  );
});
test("package reexports preserve public implementation and its helpers", async () => {
  const r = await run(fixture("exports"));
  assert.ok(r.reached.has(r.byName("publicFunction").id));
  assert.equal(r.byName("publicFunction").public, true);
  assert.ok(r.reached.has(r.byName("helper").id));
  assert.ok(!r.reached.has(r.byName("internalExport").id));
  assert.equal(r.byName("internalExport").exported, true);
});
test("initializer calls and computed class keys preserve runtime effects", async () => {
  const r = await run(fixture("side-effects"));
  for (const name of ["unused", "RiskyClass"])
    assert.equal(r.byName(name).side_effects, true);
  for (const name of ["registerPlugin", "computedKey"])
    assert.ok(r.reached.has(r.byName(name).id));
});
test("literal dynamic imports connect exported targets; string names flag reflection", async () => {
  const r = await run(fixture("dynamic-import"));
  assert.ok(r.reached.has(r.byName("plugin").id));
  assert.ok(!r.reached.has(r.byName("deadPluginHelper").id));
  assert.equal(r.byName("registeredPlugin").dynamic_risk, true);
});
test("Next.js conventions root handlers and default pages", async () => {
  const r = await run(fixture("nextjs"));
  assert.equal(r.project.framework, "nextjs");
  for (const name of ["GET", "helper", "Page"])
    assert.ok(r.reached.has(r.byName(name).id));
  assert.ok(!r.reached.has(r.byName("deadHandler").id));
});
test("glob exclusions match both root and nested generated files", () => {
  for (const name of ["a.generated.ts", "src/a.generated.ts"])
    assert.ok(glob("**/*.generated.ts", name));
  assert.ok(!glob("**/*.generated.ts", "src/a.ts"));
});
test("unresolved dynamic imports, excluded files and global scripts reduce trust", async () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "scythe-analyzer-"));
  try {
    fs.writeFileSync(
      path.join(root, "tsconfig.json"),
      JSON.stringify({
        compilerOptions: { target: "ES2022", module: "ESNext" },
        include: ["*.ts"],
      }),
    );
    fs.writeFileSync(
      path.join(root, "index.ts"),
      'export {}; const target = "./other"; import(target); function dead() {}',
    );
    fs.writeFileSync(
      path.join(root, "other.ts"),
      "export function external() {}",
    );
    const r = await run(root, ["other.ts"]);
    assert.equal(r.project.dynamic_risk, true);
    assert.equal(r.project.complete, false);
    fs.writeFileSync(
      path.join(root, "index.ts"),
      "function globalFunction() {}",
    );
    assert.equal((await run(root)).project.complete, false);
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
});
test("invalid protocol and malformed tsconfig fail usefully", async () => {
  await assert.rejects(
    analyze({
      protocolVersion: 2,
      method: "analyze",
      root: fixture("basic"),
      exclude: [],
    }),
    /protocol/,
  );
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "scythe-config-"));
  try {
    fs.writeFileSync(path.join(root, "tsconfig.json"), "{bad");
    await assert.rejects(run(root), /expected|Property|Unknown/);
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
});

test("all overload declarations remain reachable together", async () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "scythe-overloads-"));
  try {
    fs.writeFileSync(
      path.join(root, "tsconfig.json"),
      JSON.stringify({
        compilerOptions: {
          target: "ES2022",
          module: "ESNext",
          strict: true,
          types: [],
        },
        include: ["*.ts"],
      }),
    );
    fs.writeFileSync(
      path.join(root, "index.ts"),
      `export {}; function overloaded(x:string):string; function overloaded(x:number):number; function overloaded(x:string|number){return x;} overloaded(1);`,
    );
    const r = await run(root);
    assert.equal(r.project.complete, true);
    assert.equal(r.symbols.length, 3);
    assert.ok(r.symbols.every((s) => r.reached.has(s.id)));
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
});
