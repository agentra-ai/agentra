import type { MetadataRoute } from "next";
import { getSiteUrl } from "@/shared/env";

export default function sitemap(): MetadataRoute.Sitemap {
  const baseUrl = getSiteUrl();

  if (!baseUrl) {
    return [];
  }

  return [
    {
      url: baseUrl,
      lastModified: new Date("2026-04-01"),
      changeFrequency: "weekly",
      priority: 1.0,
    },
    {
      url: `${baseUrl}/about`,
      lastModified: new Date("2026-04-01"),
      changeFrequency: "monthly",
      priority: 0.7,
    },
    {
      url: `${baseUrl}/changelog`,
      lastModified: new Date("2026-04-01"),
      changeFrequency: "weekly",
      priority: 0.6,
    },
  ];
}
