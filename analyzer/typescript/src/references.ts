import ts from "typescript";
import type { LoadedProject } from "./program.js";
import type { Discovered } from "./symbols.js";
import { frameworkFile } from "./symbols.js";
export function references(project: LoadedProject, discovered: Discovered) {
  const edges: { from: string; to: string; file: string; line: number }[] = [];

  // All declarations of an overloaded compiler symbol share its reachability.
  for (const group of discovered.declarationGroups.values()) {
    const canonical = group.at(-1)!;
    for (const id of group.slice(0, -1)) {
      const symbol = discovered.byID.get(id)!;
      edges.push({
        from: canonical,
        to: id,
        file: symbol.file,
        line: symbol.start_line,
      });
    }
  }
  const roots = new Set<string>();
  let dynamicRisk = false;
  const modules = new Map(
    project.files.map((f) => [
      f.fileName,
      `module:${project.relative(f.fileName)}`,
    ]),
  );
  const add = (from: string, to: string, node: ts.Node, file: ts.SourceFile) =>
    edges.push({
      from,
      to,
      file: project.relative(file.fileName),
      line: file.getLineAndCharacterOfPosition(node.getStart(file)).line + 1,
    });
  function resolve(symbol: ts.Symbol | undefined): string | undefined {
    if (!symbol) return;
    if (symbol.flags & ts.SymbolFlags.Alias)
      symbol = project.checker.getAliasedSymbol(symbol);
    return (
      discovered.symbols.get(symbol) ??
      symbol.declarations?.map((d) => discovered.nodes.get(d)).find(Boolean)
    );
  }
  for (const record of discovered.records)
    if (record.public || record.framework_entry) roots.add(record.id);
  for (const file of project.files) {
    const module = modules.get(file.fileName)!;
    // All configured modules may be executed by tooling. This conservative root
    // assumption does not make uncalled function bodies reachable.
    roots.add(module);
    const moduleSymbol = project.checker.getSymbolAtLocation(file);
    if (
      project.publicFiles.has(file) ||
      frameworkFile(project, file) ||
      project.executableFiles.has(file)
    ) {
      for (const symbol of moduleSymbol
        ? project.checker.getExportsOfModule(moduleSymbol)
        : []) {
        const id = resolve(symbol);
        if (id) roots.add(id);
      }
    }
    const visit = (node: ts.Node, owner: string) => {
      if (
        ts.isImportDeclaration(node) ||
        ts.isExportDeclaration(node) ||
        ts.isImportEqualsDeclaration(node)
      )
        return;
      const id = discovered.nodes.get(node);
      if (id) {
        const record = discovered.byID.get(id)!;
        if (record.side_effects) add(owner, id, node, file);
        owner = id;
      }
      if (ts.isCallExpression(node)) {
        if (node.expression.kind === ts.SyntaxKind.ImportKeyword) {
          const argument = node.arguments[0];
          if (argument && ts.isStringLiteralLike(argument)) {
            const symbol = project.checker.getSymbolAtLocation(argument);
            if (symbol)
              for (const exported of project.checker.getExportsOfModule(
                symbol,
              )) {
                const target = resolve(exported);
                if (target) add(owner, target, node, file);
              }
            else dynamicRisk = true;
          } else dynamicRisk = true;
        }
        if (
          ts.isIdentifier(node.expression) &&
          ["eval", "require", "Function"].includes(node.expression.text)
        )
          dynamicRisk = true;
      }
      if (ts.isElementAccessExpression(node)) dynamicRisk = true;
      if (ts.isIdentifier(node)) {
        const parent = node.parent;
        const declarationName =
          (ts.isDeclarationStatement(parent) ||
            ts.isVariableDeclaration(parent) ||
            ts.isParameter(parent) ||
            ts.isPropertyDeclaration(parent) ||
            ts.isMethodDeclaration(parent)) &&
          "name" in parent &&
          parent.name === node;
        if (!declarationName) {
          const target = resolve(project.checker.getSymbolAtLocation(node));
          if (target) add(owner, target, node, file);
        }
      }
      ts.forEachChild(node, (child) => visit(child, owner));
    };
    visit(file, module);
  }
  // String-based symbol names may indicate registries or reflective lookup.
  const names = new Set(discovered.records.map((r) => r.name));
  const stringNames = new Set<string>();
  for (const file of project.files) {
    const visit = (n: ts.Node) => {
      if (ts.isStringLiteralLike(n) && names.has(n.text))
        stringNames.add(n.text);
      ts.forEachChild(n, visit);
    };
    visit(file);
  }
  for (const record of discovered.records)
    record.dynamic_risk = stringNames.has(record.name);
  return { edges, roots: [...roots].sort(), dynamicRisk };
}
