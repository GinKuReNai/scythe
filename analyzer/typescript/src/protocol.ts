import type { Writable } from "node:stream";
export const protocolVersion = 1;
export interface Request {
  protocolVersion: 1;
  method: "analyze";
  root: string;
  exclude: string[];
}
export interface SymbolRecord {
  id: string;
  name: string;
  kind: "function" | "class" | "variable";
  file: string;
  start_line: number;
  end_line: number;
  exported: boolean;
  public: boolean;
  framework_entry: boolean;
  side_effects: boolean;
  dynamic_risk: boolean;
  source: string;
  source_truncated: boolean;
}
export async function emit(
  out: Writable,
  type: string,
  data: unknown,
): Promise<void> {
  const line = JSON.stringify({ protocolVersion, type, data }) + "\n";
  if (!out.write(line))
    await new Promise<void>((resolve, reject) => {
      out.once("drain", resolve);
      out.once("error", reject);
    });
}
