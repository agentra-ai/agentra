import en from "../messages/en.json";
import zhCN from "../messages/zh-CN.json";

export const messages = {
  en,
  "zh-CN": zhCN,
} as const;

export type Message = (typeof messages)["en"];
