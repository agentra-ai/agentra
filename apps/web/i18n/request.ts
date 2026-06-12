import { getRequestConfig } from "next-intl/server";
import { cookies } from "next/headers";
import enMessages from "../messages/en.json";
import zhMessages from "../messages/zh-CN.json";

export const locales = ["en", "zh-CN"] as const;
export type Locale = (typeof locales)[number];
export const defaultLocale: Locale = "en";

const messageBundles: Record<Locale, Record<string, unknown>> = {
  en: enMessages,
  "zh-CN": zhMessages,
};

/**
 * Verify that a parsed JSON object has no duplicate sibling keys. JSON.parse
 * uses last-wins semantics, so a duplicate top-level "inbox" would silently
 * override earlier content. We pre-parse the raw text with a streaming parser
 * that surfaces duplicates as errors.
 */
function findDuplicateKeys(
  raw: string,
  filename: string,
): string[] {
  const duplicates: string[] = [];
  let i = 0;
  let line = 1;
  let col = 0;
  let path: string[] = [];

  const isWs = (c: string) => c === " " || c === "\t" || c === "\n" || c === "\r";
  const skipWs = () => {
    while (i < raw.length && isWs(raw[i]!)) {
      if (raw[i] === "\n") {
        line += 1;
        col = 0;
      } else {
        col += 1;
      }
      i += 1;
    }
  };

  const expect = (ch: string) => {
    if (raw[i] !== ch) {
      throw new Error(
        `i18n: unexpected '${raw[i]}' at ${filename}:${line}:${col + 1} (expected '${ch}')`,
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

  const parseString = (): string => {
    expect('"');
    let out = "";
    while (i < raw.length && raw[i] !== '"') {
      if (raw[i] === "\\") {
        out += raw[i]!;
        i += 1;
        if (i < raw.length) {
          out += raw[i]!;
          i += 1;
        }
      } else {
        out += raw[i]!;
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

  const parseValue = (): unknown => {
    skipWs();
    const c = raw[i];
    if (c === "{") return parseObject();
    if (c === "[") return parseArray();
    if (c === '"') return parseString();
    if (c === "t" || c === "f") return parseBool();
    if (c === "n") return parseNull();
    return parseNumber();
  };

  const parseObject = (): Record<string, unknown> => {
    const obj: Record<string, unknown> = {};
    const seen = new Set<string>();
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
        duplicates.push(`[${path.join(".")}.${key}] at ${filename}:${line}:${col + 1}`);
      }
      seen.add(key);
      path.push(key);
      skipWs();
      expect(":");
      obj[key] = parseValue();
      path.pop();
      skipWs();
      if (raw[i] === ",") {
        i += 1;
        continue;
      }
      if (raw[i] === "}") {
        i += 1;
        return obj;
      }
      throw new Error(`i18n: unexpected '${raw[i]}' at ${filename}:${line}:${col + 1}`);
    }
    throw new Error(`i18n: unterminated object in ${filename}`);
  };

  const parseArray = (): unknown[] => {
    const arr: unknown[] = [];
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
      throw new Error(`i18n: unexpected '${raw[i]}' at ${filename}:${line}:${col + 1}`);
    }
    throw new Error(`i18n: unterminated array in ${filename}`);
  };

  const parseBool = (): boolean => {
    if (raw.slice(i, i + 4) === "true") {
      i += 4;
      return true;
    }
    if (raw.slice(i, i + 5) === "false") {
      i += 5;
      return false;
    }
    throw new Error(`i18n: bad bool at ${filename}:${line}:${col + 1}`);
  };

  const parseNull = (): null => {
    if (raw.slice(i, i + 4) === "null") {
      i += 4;
      return null;
    }
    throw new Error(`i18n: bad null at ${filename}:${line}:${col + 1}`);
  };

  const parseNumber = (): number => {
    let out = "";
    while (i < raw.length && /[0-9eE+\-.]/.test(raw[i]!)) {
      out += raw[i]!;
      i += 1;
    }
    return Number(out);
  };

  parseValue();
  return duplicates;
}

function collectKeys(value: unknown, prefix: string, out: Set<string>): void {
  if (value && typeof value === "object" && !Array.isArray(value)) {
    for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
      const path = prefix ? `${prefix}.${k}` : k;
      out.add(path);
      collectKeys(v, path, out);
    }
  }
}

let validated = false;
function validateMessageBundles(): void {
  if (validated) return;
  validated = true;

  // 1. Duplicate-key detection (raw parse). JSON.parse is last-wins, so the
  //    dev-mode guard below must run *before* a stale sibling can override
  //    new content. We pre-parse the raw source.
  const fs = require("fs") as typeof import("fs");
  const path = require("path") as typeof import("path");
  const dir = path.join(process.cwd(), "apps/web/messages");
  for (const [name, file] of [
    ["en", path.join(dir, "en.json")],
    ["zh-CN", path.join(dir, "zh-CN.json")],
  ] as const) {
    const raw = fs.readFileSync(file, "utf8");
    const duplicates = findDuplicateKeys(raw, `${name}.json`);
    if (duplicates.length > 0) {
      const message = `i18n: ${file} has duplicate keys: ${duplicates.join(", ")}. ` +
        `JSON.parse uses last-wins; the duplicate would silently override the first occurrence.`;
      if (process.env.NODE_ENV !== "production") {
        throw new Error(message);
      } else {
        // eslint-disable-next-line no-console
        console.error(message);
      }
    }
  }

  // 2. 1:1 key-set parity between locales.
  const enKeys = new Set<string>();
  const zhKeys = new Set<string>();
  collectKeys(messageBundles.en, "", enKeys);
  collectKeys(messageBundles["zh-CN"], "", zhKeys);
  const onlyEn = [...enKeys].filter((k) => !zhKeys.has(k));
  const onlyZh = [...zhKeys].filter((k) => !enKeys.has(k));
  if (onlyEn.length > 0 || onlyZh.length > 0) {
    const message = `i18n: key-set mismatch between en.json and zh-CN.json. ` +
      `Only in en: [${onlyEn.join(", ")}]. Only in zh-CN: [${onlyZh.join(", ")}].`;
    if (process.env.NODE_ENV !== "production") {
      throw new Error(message);
    } else {
      // eslint-disable-next-line no-console
      console.error(message);
    }
  }
}

export default getRequestConfig(async () => {
  validateMessageBundles();

  const cookieStore = await cookies();
  // Support both 'zh' (legacy) and 'zh-CN' formats
  let locale: Locale = defaultLocale;
  const cookieLocale = cookieStore.get("agentra-locale")?.value;

  if (cookieLocale === "zh-CN" || cookieLocale === "zh") {
    locale = "zh-CN";
  }

  return {
    locale,
    messages: messageBundles[locale],
  };
});
