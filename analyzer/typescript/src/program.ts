import ts from "typescript";
import fs from "node:fs";
import path from "node:path";
import type { Request } from "./protocol.js";

export function glob(pattern: string, file: string): boolean {
  const escaped = pattern.replace(/[.+^${}()|[\]\\]/g, "\\$&");
  const regex = escaped
    .replace(/\*\*\//g, "\u0000")
    .replace(/\*\*/g, "\u0001")
    .replace(/\*/g, "[^/]*")
    .replace(/\?/g, "[^/]")
    .replace(/\u0000/g, "(?:.*/)?")
    .replace(/\u0001/g, ".*");
  return new RegExp(`^${regex}$`).test(file);
}
export function loadProject(request: Request) {
  const root = fs.realpathSync(request.root);
  if (!fs.statSync(root).isDirectory())
    throw new Error("Analysis path must be a project directory");
  const configPath = ts.findConfigFile(root, ts.sys.fileExists);
  if (!configPath)
    throw new Error(
      "tsconfig.json not found; specify a TypeScript project directory",
    );
  const raw = ts.readConfigFile(configPath, ts.sys.readFile);
  if (raw.error)
    throw new Error(
      ts.flattenDiagnosticMessageText(raw.error.messageText, "\n"),
    );
  const config = ts.parseJsonConfigFileContent(
    raw.config,
    ts.sys,
    path.dirname(configPath),
  );
  if (config.errors.length)
    throw new Error(
      config.errors
        .map((d) => ts.flattenDiagnosticMessageText(d.messageText, "\n"))
        .join("\n"),
    );
  const program = ts.createProgram(config.fileNames, config.options);
  const checker = program.getTypeChecker();
  const relative = (file: string) =>
    path.relative(root, file).split(path.sep).join("/");
  const ownedFiles = program
    .getSourceFiles()
    .filter(
      (f) =>
        !f.isDeclarationFile &&
        !program.isSourceFileFromExternalLibrary(f) &&
        !relative(f.fileName).startsWith("../"),
    );
  const allFiles = ownedFiles.filter((f) => /\.[cm]?tsx?$/.test(f.fileName));
  const files = allFiles.filter(
    (f) =>
      !request.exclude.some((g) => glob(g, relative(f.fileName))) &&
      !/(@generated|auto-generated|automatically generated)/i.test(
        f.text.slice(0, 300),
      ),
  );
  const warnings: string[] = [];
  if (ownedFiles.length !== allFiles.length)
    warnings.push(
      "JavaScript source is not analyzed; runtime reachability may be incomplete.",
    );
  if (
    program
      .getSourceFiles()
      .some(
        (f) =>
          !f.isDeclarationFile &&
          !program.isSourceFileFromExternalLibrary(f) &&
          relative(f.fileName).startsWith("../"),
      )
  )
    warnings.push(
      "Source files outside the project directory are not analyzed.",
    );
  const hasNamespace = (node: ts.Node): boolean =>
    ts.isModuleDeclaration(node) || !!ts.forEachChild(node, hasNamespace);
  if (files.some(hasNamespace))
    warnings.push(
      "Namespace execution and public surfaces require manual review.",
    );
  if (files.length !== allFiles.length)
    warnings.push(
      "Runtime source files were excluded; reachability may be incomplete.",
    );
  if (config.projectReferences?.length)
    warnings.push(
      "Project references are not traversed; scan each referenced project separately.",
    );
  if (files.some((f) => !ts.isExternalModule(f)))
    warnings.push(
      "Global script declarations may be consumed outside this project.",
    );
  const diagnostics = ts.getPreEmitDiagnostics(program);
  if (diagnostics.length)
    warnings.push(
      `TypeScript reported ${diagnostics.length} diagnostic(s); reachability may be incomplete.`,
    );
  let pkg: Record<string, unknown> = {};
  const packagePath = path.join(root, "package.json");
  if (fs.existsSync(packagePath))
    pkg = JSON.parse(fs.readFileSync(packagePath, "utf8"));
  const deps = {
    ...(pkg.dependencies as object),
    ...(pkg.devDependencies as object),
  };
  const names = fs.readdirSync(root);
  const framework =
    "next" in deps || names.some((n) => /^next\.config\./.test(n))
      ? "nextjs"
      : "vite" in deps || names.some((n) => /^vite\.config\./.test(n))
        ? "vite"
        : Object.keys(pkg).length
          ? "node"
          : "unknown";
  const publicPaths = new Set<string>();
  const executablePaths = new Set<string>();
  function collect(value: unknown, target: Set<string>) {
    if (typeof value === "string") target.add(value.replace(/^\.\//, ""));
    else if (value && typeof value === "object")
      Object.values(value).forEach((v) => collect(v, target));
  }
  collect(pkg.exports, publicPaths);
  collect(pkg.main, publicPaths);
  collect(pkg.module, publicPaths);
  collect(pkg.types, publicPaths);
  collect(pkg.bin, executablePaths);
  // Built outputs do not establish a reliable source mapping. Preserve exports if unmapped.
  function matches(file: ts.SourceFile, targets: Set<string>) {
    const rel = relative(file.fileName);
    return [...targets].some(
      (t) =>
        t === rel ||
        t.replace(/\.[cm]?js$/, "") === rel.replace(/\.[cm]?tsx?$/, "") ||
        (t.includes("*") && glob(t, rel)),
    );
  }
  const unmappedPublic = [...publicPaths].some(
    (t) => !files.some((f) => matches(f, new Set([t]))),
  );
  const unmappedExecutable = [...executablePaths].some(
    (t) => !files.some((f) => matches(f, new Set([t]))),
  );
  if (unmappedExecutable)
    warnings.push(
      "Executable package entries could not be mapped to TypeScript source.",
    );
  if (unmappedPublic)
    warnings.push(
      "Public package outputs could not be mapped to source; exported symbols are preserved.",
    );
  const publicFiles = new Set(files.filter((f) => matches(f, publicPaths)));
  const executableFiles = new Set(
    files.filter((f) => matches(f, executablePaths)),
  );
  const discoveredFiles = names
    .filter(
      (name) =>
        [
          "package.json",
          "tsconfig.json",
          "pnpm-lock.yaml",
          "yarn.lock",
          "package-lock.json",
        ].includes(name) || /^(next|vite)\.config\./.test(name),
    )
    .sort();
  const sourceRoots = [
    ...new Set(
      files.map((file) => relative(path.dirname(file.fileName)) || "."),
    ),
  ].sort();
  return {
    discoveredFiles,
    sourceRoots,
    root,
    configPath,
    program,
    checker,
    files,
    relative,
    warnings,
    framework,
    publicFiles,
    executableFiles,
    unmappedPublic,
  };
}
export type LoadedProject = ReturnType<typeof loadProject>;
