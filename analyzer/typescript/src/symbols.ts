import ts from "typescript";
import type { LoadedProject } from "./program.js";
import type { SymbolRecord } from "./protocol.js";

// Only expressions with clearly inert evaluation are treated as pure.
export function pure(node: ts.Node | undefined): boolean {
  if (!node) return true;
  if (
    ts.isLiteralExpression(node) ||
    node.kind === ts.SyntaxKind.TrueKeyword ||
    node.kind === ts.SyntaxKind.FalseKeyword ||
    node.kind === ts.SyntaxKind.NullKeyword ||
    ts.isArrowFunction(node) ||
    ts.isFunctionExpression(node)
  )
    return true;
  if (
    ts.isParenthesizedExpression(node) ||
    ts.isAsExpression(node) ||
    ts.isSatisfiesExpression(node) ||
    ts.isNonNullExpression(node)
  )
    return pure(node.expression);
  if (ts.isArrayLiteralExpression(node))
    return node.elements.every((e) => pure(e));
  if (ts.isObjectLiteralExpression(node))
    return node.properties.every(
      (p) =>
        ts.isPropertyAssignment(p) &&
        !ts.isComputedPropertyName(p.name) &&
        pure(p.initializer),
    );
  return false;
}
export function frameworkFile(
  project: LoadedProject,
  file: ts.SourceFile,
): boolean {
  const name = project.relative(file.fileName);
  if (project.framework === "nextjs")
    return (
      (/(^|\/)(app|pages)\//.test(name) &&
        (/\/(page|layout|route|loading|error|global-error|not-found|template|default|head|middleware)\.[cm]?tsx?$/.test(
          name,
        ) ||
          /(^|\/)pages\//.test(name))) ||
      /(^|\/)(middleware|instrumentation)\.[cm]?tsx?$/.test(name)
    );
  if (project.framework === "vite")
    return /(^|\/)(main|index|vite\.config)\.[cm]?tsx?$/.test(name);
  return false;
}
export function discover(project: LoadedProject) {
  const records: SymbolRecord[] = [];
  const nodes = new Map<ts.Node, string>();
  const symbols = new Map<ts.Symbol, string>();
  const declarationGroups = new Map<ts.Symbol, string[]>();
  const exportedByFile = new Map<ts.SourceFile, Set<ts.Symbol>>();
  for (const file of project.files) {
    const module = project.checker.getSymbolAtLocation(file);
    const exported = new Set<ts.Symbol>();
    for (const symbol of module
      ? project.checker.getExportsOfModule(module)
      : [])
      exported.add(
        symbol.flags & ts.SymbolFlags.Alias
          ? project.checker.getAliasedSymbol(symbol)
          : symbol,
      );
    exportedByFile.set(file, exported);
  }
  const allExported = new Set(
    [...exportedByFile.values()].flatMap((set) => [...set]),
  );
  const publicSymbols = new Set<ts.Symbol>();
  for (const [file, exports] of exportedByFile)
    if (project.publicFiles.has(file))
      for (const symbol of exports) publicSymbols.add(symbol);
  for (const file of project.files) {
    const visit = (node: ts.Node) => {
      let name: ts.Node | undefined;
      let kind: SymbolRecord["kind"] | undefined;
      if (ts.isFunctionDeclaration(node)) {
        name = node.name;
        kind = "function";
      }
      if (ts.isClassDeclaration(node)) {
        name = node.name;
        kind = "class";
      }
      if (ts.isVariableDeclaration(node)) {
        name = node.name;
        kind = "variable";
      }
      if (kind) {
        const symbol = name
          ? project.checker.getSymbolAtLocation(name)
          : undefined;
        const pos = node.getStart(file);
        const id = `${project.relative(file.fileName)}:${pos}:${kind}`;
        const exported =
          (!!symbol && allExported.has(symbol)) ||
          !!ts
            .getModifiers(node as ts.HasModifiers)
            ?.some((m) => m.kind === ts.SyntaxKind.ExportKeyword);
        const classRisk =
          ts.isClassDeclaration(node) &&
          (!!node.heritageClauses?.length ||
            node.members.some(
              (m) =>
                ts.isClassStaticBlockDeclaration(m) ||
                !!ts.getDecorators(m as ts.HasDecorators)?.length ||
                ("name" in m &&
                  !!m.name &&
                  ts.isComputedPropertyName(m.name)) ||
                (ts.isPropertyDeclaration(m) &&
                  (!pure(m.initializer) || ts.isComputedPropertyName(m.name))),
            ));
        const decoratorRisk = !!ts.getDecorators(node as ts.HasDecorators)
          ?.length;
        const variableRisk =
          ts.isVariableDeclaration(node) &&
          (!ts.isIdentifier(node.name) || !pure(node.initializer));
        const source = node.getText(file);
        records.push({
          id,
          name: name?.getText(file) ?? "default",
          kind,
          file: project.relative(file.fileName),
          start_line: file.getLineAndCharacterOfPosition(pos).line + 1,
          end_line: file.getLineAndCharacterOfPosition(node.end).line + 1,
          exported,
          public:
            (!!symbol && publicSymbols.has(symbol)) ||
            (exported && project.unmappedPublic),
          framework_entry: frameworkFile(project, file) && exported,
          side_effects: !!(classRisk || decoratorRisk || variableRisk),
          dynamic_risk: false,
          source: source.slice(0, 2048),
          source_truncated: source.length > 2048,
        });
        nodes.set(node, id);
        if (symbol) {
          symbols.set(symbol, id);
          const group = declarationGroups.get(symbol) ?? [];
          group.push(id);
          declarationGroups.set(symbol, group);
        }
      }
      ts.forEachChild(node, visit);
    };
    visit(file);
  }
  return {
    records,
    nodes,
    symbols,
    byID: new Map(records.map((record) => [record.id, record])),
    declarationGroups,
  };
}
export type Discovered = ReturnType<typeof discover>;
