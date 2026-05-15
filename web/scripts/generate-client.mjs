import fs from "node:fs";
import path from "node:path";

const rootDir = path.resolve(path.dirname(new URL(import.meta.url).pathname), "..");
const repoDir = path.resolve(rootDir, "..");
const specPath = path.join(repoDir, "openapi", "openapi.json");
const outputPath = path.join(rootDir, "src", "generated", "client.ts");

const spec = JSON.parse(fs.readFileSync(specPath, "utf8"));
const schemas = spec.components?.schemas ?? {};

function schemaNameFromRef(ref) {
  return ref.split("/").at(-1);
}

function toTs(schema) {
  if (!schema) {
    return "unknown";
  }
  if (schema.$ref) {
    return schemaNameFromRef(schema.$ref);
  }
  if (schema.enum) {
    return schema.enum.map((value) => JSON.stringify(value)).join(" | ");
  }
  if (schema.oneOf) {
    return schema.oneOf.map(toTs).join(" | ");
  }
  if (schema.anyOf) {
    return schema.anyOf.map(toTs).join(" | ");
  }
  if (schema.type === "array") {
    return `${toTs(schema.items)}[]`;
  }
  if (schema.type === "object" || schema.properties || schema.additionalProperties) {
    if (!schema.properties && schema.additionalProperties) {
      return `Record<string, ${schema.additionalProperties === true ? "unknown" : toTs(schema.additionalProperties)}>`;
    }
    const required = new Set(schema.required ?? []);
    const fields = Object.entries(schema.properties ?? {}).map(([name, value]) => {
      const optional = required.has(name) ? "" : "?";
      return `${JSON.stringify(name)}${optional}: ${toTs(value)};`;
    });
    if (schema.additionalProperties) {
      fields.push(`[key: string]: ${schema.additionalProperties === true ? "unknown" : toTs(schema.additionalProperties)};`);
    }
    return `{ ${fields.join(" ")} }`;
  }
  switch (schema.type) {
    case "integer":
    case "number":
      return "number";
    case "boolean":
      return "boolean";
    case "string":
      return "string";
    default:
      return "unknown";
  }
}

function emitSchema(name, schema) {
  return `export interface ${name} ${toTs(schema)}`;
}

function operationResponseSchema(operation) {
  return operation.responses?.["200"]?.content?.["application/json"]?.schema ?? { type: "unknown" };
}

function buildParamInterface(operationId, params) {
  if (params.length === 0) {
    return "";
  }
  const lines = params.map((param) => {
    const optional = param.required ? "" : "?";
    return `  ${JSON.stringify(param.name)}${optional}: ${toTs(param.schema)};`;
  });
  return `export interface ${operationId}Params {\n${lines.join("\n")}\n}\n`;
}

function pathExpression(route, params) {
  let expression = "`" + route.replace(/{([^}]+)}/g, (_, key) => `\${encodeURIComponent(String(params[${JSON.stringify(key)}]))}`) + "`";
  return expression;
}

function queryObject(params) {
  const queryParams = params.filter((param) => param.in === "query");
  if (queryParams.length === 0) {
    return "undefined";
  }
  return `{ ${queryParams.map((param) => `${JSON.stringify(param.name)}: params[${JSON.stringify(param.name)}]`).join(", ")} }`;
}

function buildOperation(route, operation) {
  const operationId = operation.operationId;
  const params = operation.parameters ?? [];
  const responseType = toTs(operationResponseSchema(operation));
  const hasParams = params.length > 0;
  const allOptional = params.every((param) => !param.required);
  const paramType = hasParams ? `${operationId}Params` : "void";
  const paramArg = !hasParams ? "" : allOptional ? `params: ${paramType} = {}` : `params: ${paramType}`;
  const pathExpr = hasParams ? pathExpression(route, params) : JSON.stringify(route);
  const queryExpr = hasParams ? queryObject(params) : "undefined";

  return `  async ${operationId}(${[paramArg, "init?: RequestInit"].filter(Boolean).join(", ")}): Promise<${responseType}> {
    return request<${responseType}>(config.fetchImpl ?? globalThis.fetch.bind(globalThis), baseUrl, ${pathExpr}, ${queryExpr}, init);
  },`;
}

const operations = [];
const interfaces = [];
for (const [route, methods] of Object.entries(spec.paths)) {
  for (const [method, operation] of Object.entries(methods)) {
    if (method !== "get") {
      continue;
    }
    const params = operation.parameters ?? [];
    if (params.length > 0) {
      interfaces.push(buildParamInterface(operation.operationId, params));
    }
    operations.push(buildOperation(route, operation));
  }
}

const output = `/* eslint-disable */
// This file is generated from openapi/openapi.json. Do not edit by hand.

${Object.entries(schemas).map(([name, schema]) => emitSchema(name, schema)).join("\n\n")}

${interfaces.join("\n")}
type QueryParams = Record<string, string | number | boolean | undefined>;

export interface ClientConfig {
  baseUrl?: string;
  fetchImpl?: typeof fetch;
}

async function request<T>(
  fetchImpl: typeof fetch,
  baseUrl: string,
  route: string,
  query: QueryParams | undefined,
  init?: RequestInit
): Promise<T> {
  const url = new URL(route, baseUrl.endsWith("/") ? baseUrl : baseUrl + "/");
  if (query) {
    for (const [key, value] of Object.entries(query)) {
      if (value !== undefined && value !== "") {
        url.searchParams.set(key, String(value));
      }
    }
  }
  const response = await fetchImpl(url.toString(), init);
  const payload = await response.json();
  if (!response.ok) {
    const message = payload?.error?.message ?? response.statusText;
    throw new Error(message);
  }
  return payload as T;
}

export function createClient(config: ClientConfig = {}) {
  const baseUrl = config.baseUrl ?? (import.meta.env.VITE_API_BASE_URL || "http://localhost:8080");
  return {
${operations.join("\n")}
  };
}

export const apiClient = createClient();
`;

fs.mkdirSync(path.dirname(outputPath), { recursive: true });
fs.writeFileSync(outputPath, output);
