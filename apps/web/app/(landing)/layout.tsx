import { getSiteUrl } from "@/shared/env";

const siteUrl = getSiteUrl();

const jsonLd = {
  "@context": "https://schema.org",
  "@graph": [
    {
      "@type": "Organization",
      name: "Agentra",
      url: siteUrl,
    },
    {
      "@type": "SoftwareApplication",
      name: "Agentra",
      applicationCategory: "ProjectManagement",
      operatingSystem: "Web",
      description:
        "AI-native task management platform that turns coding agents into real teammates.",
      offers: {
        "@type": "Offer",
        price: "0",
        priceCurrency: "USD",
      },
    },
  ],
};

export default function LandingLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }}
      />
      <div className="h-full overflow-x-hidden overflow-y-auto bg-white">
        {children}
      </div>
    </>
  );
}
