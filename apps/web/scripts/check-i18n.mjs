#!/usr/bin/env node
/**
 * check-i18n.mjs — Pre-build validator for the apps/web i18n message bundles.
 *
 * Why this is a prebuild script and not a runtime guard:
 *   - The Next.js middleware (which is the consumer of i18n/request.ts) runs
 *     in the Edge runtime, where process / fs / path are unavailable. A
 *     runtime validator there breaks `next build` with "A Node.js API is
 *     used" errors.
 *   - Running in plain Node as part of `prebuild` / `predev` / `test` (and
 *     CI) catches the same class of bugs at build time without any runtime
 *     cost or Edge-runtime coupling.
 *
 * What it checks:
 *   1. Each locale file parses without duplicate sibling keys
 *      (JSON.parse uses last-wins, so a duplicate silently overrides the
 *      first occurrence — exactly the f6ad441 inbox bug).
 *   2. The set of leaf keys in en.json matches the set of leaf keys in
 *      zh-CN.json (1:1 parity).
 *
 * Exits 0 on success, 1 on failure with a diagnostic for each problem.
 */
import fs from "node:fs";
import path from "node:path";
import process from "node:process";

const messagesDir = path.resolve(
  path.dirname(new URL(import.meta.url).pathname),
  "../messages",
);

const files = ["en.json", "zh-CN.json"];

/**
 * Stream-parse a JSON document. Track current path (e.g. "inbox.types") and
 * return the list of duplicate sibling keys, each annotated with the file
 * and line:col where the duplicate was seen.
 */
function findDuplicateKeys(raw, filename) {
  const duplicates = [];
  let i = 0;
  let line = 1;
  let col = 0;
  const pathStack = [];

  const isWs = (c) => c === " " || c === "\t" || c === "\n" || c === "\r";
  const skipWs = () => {
    while (i < raw.length && isWs(raw[i])) {
      if (raw[i] === "\n") {
        line += 1;
        col = 0;
      } else {
        col += 1;
      }
      i += 1;
    }
  };

  const expect = (ch) => {
    if (raw[i] !== ch) {
      throw new Error(
        `unexpected '${raw[i]}' at ${filename}:${line}:${col + 1} (expected '${ch}')`,
      );
    }
    if (ch === "\n") {
      line += 1;
      col = 0;
    } else {
      col += 1;
    }
    i += 1;
  };

  const parseString = () => {
    expect('"');
    let out = "";
    while (i < raw.length && raw[i] !== '"') {
      if (raw[i] === "\\") {
        out += raw[i];
        i += 1;
        if (i < raw.length) {
          out += raw[i];
          i += 1;
        }
      } else {
        out += raw[i];
        if (raw[i] === "\n") {
          line += 1;
          col = 0;
        } else {
          col += 1;
        }
        i += 1;
      }
    }
    expect('"');
    return out;
  };

  const parseValue = () => {
    skipWs();
    const c = raw[i];
    if (c === "{") return parseObject();
    if (c === "[") return parseArray();
    if (c === '"') return parseString();
    if (c === "t" || c === "f") return parseBool();
    if (c === "n") return parseNull();
    return parseNumber();
  };

  const parseObject = () => {
    const obj = {};
    const seen = new Set();
    expect("{");
    skipWs();
    if (raw[i] === "}") {
      i += 1;
      return obj;
    }
    while (i < raw.length) {
      skipWs();
      const key = parseString();
      if (seen.has(key)) {
        const dotted = pathStack.length ? `${pathStack.join(".")}.${key}` : key;
        duplicates.push(`'${dotted}' at ${filename}:${line}:${col + 1}`);
      }
      seen.add(key);
      pathStack.push(key);
      skipWs();
      expect(":");
      obj[key] = parseValue();
      pathStack.pop();
      skipWs();
      if (raw[i] === ",") {
        i += 1;
        continue;
      }
      if (raw[i] === "}") {
        i += 1;
        return obj;
      }
      throw new Error(`unexpected '${raw[i]}' at ${filename}:${line}:${col + 1}`);
    }
    throw new Error(`unterminated object in ${filename}`);
  };

  const parseArray = () => {
    const arr = [];
    expect("[");
    skipWs();
    if (raw[i] === "]") {
      i += 1;
      return arr;
    }
    while (i < raw.length) {
      arr.push(parseValue());
      skipWs();
      if (raw[i] === ",") {
        i += 1;
        continue;
      }
      if (raw[i] === "]") {
        i += 1;
        return arr;
      }
      throw new Error(`unexpected '${raw[i]}' at ${filename}:${line}:${col + 1}`);
    }
    throw new Error(`unterminated array in ${filename}`);
  };

  const parseBool = () => {
    if (raw.slice(i, i + 4) === "true") {
      i += 4;
      return true;
    }
    if (raw.slice(i, i + 5) === "false") {
      i += 5;
      return false;
    }
    throw new Error(`bad bool at ${filename}:${line}:${col + 1}`);
  };

  const parseNull = () => {
    if (raw.slice(i, i + 4) === "null") {
      i += 4;
      return null;
    }
    throw new Error(`bad null at ${filename}:${line}:${col + 1}`);
  };

  const parseNumber = () => {
    let out = "";
    while (i < raw.length && /[0-9eE+\-.]/.test(raw[i])) {
      out += raw[i];
      i += 1;
    }
    return Number(out);
  };

  parseValue();
  return duplicates;
}

function collectKeys(value, prefix, out) {
  if (value && typeof value === "object" && !Array.isArray(value)) {
    for (const [k, v] of Object.entries(value)) {
      const path = prefix ? `${prefix}.${k}` : k;
      out.add(path);
      collectKeys(v, path, out);
    }
  }
}

const errors = [];

// 1. Duplicate-key detection
for (const file of files) {
  const full = path.join(messagesDir, file);
  const raw = fs.readFileSync(full, "utf8");
  const duplicates = findDuplicateKeys(raw, file);
  if (duplicates.length > 0) {
    errors.push(`${file} has duplicate keys: ${duplicates.join(", ")}`);
  }
}

// 2. 1:1 key-set parity between en and zh-CN
const enRaw = JSON.parse(fs.readFileSync(path.join(messagesDir, "en.json"), "utf8"));
const zhRaw = JSON.parse(fs.readFileSync(path.join(messagesDir, "zh-CN.json"), "utf8"));
const enKeys = new Set();
const zhKeys = new Set();
collectKeys(enRaw, "", enKeys);
collectKeys(zhRaw, "", zhKeys);
const onlyEn = [...enKeys].filter((k) => !zhKeys.has(k));
const onlyZh = [...zhKeys].filter((k) => !enKeys.has(k));
if (onlyEn.length > 0 || onlyZh.length > 0) {
  errors.push(
    `key-set mismatch — only in en.json: [${onlyEn.join(", ")}]; only in zh-CN.json: [${onlyZh.join(", ")}]`,
  );
}

if (errors.length > 0) {
  console.error(`\n❌ i18n check failed:\n  - ${errors.join("\n  - ")}\n`);
  process.exit(1);
}

console.log(`✅ i18n check passed (${files.length} files, ${enKeys.size} keys each).`);
