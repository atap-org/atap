export type NavItem = {
  title: string;
  href: string;
};

export type NavSection = {
  title: string;
  links: NavItem[];
};

export const navigation: NavSection[] = [
  {
    title: "Getting Started",
    links: [
      { title: "Introduction", href: "/docs/getting-started" },
      { title: "Quickstart", href: "/docs/quickstart" },
    ],
  },
  {
    title: "Concepts",
    links: [
      { title: "Entities & DIDs", href: "/docs/concepts/entities" },
      { title: "Approvals", href: "/docs/concepts/approvals" },
      { title: "Credentials", href: "/docs/concepts/credentials" },
      { title: "DIDComm Messaging", href: "/docs/concepts/didcomm" },
      { title: "Templates", href: "/docs/concepts/templates" },
      { title: "Server Trust", href: "/docs/concepts/server-trust" },
    ],
  },
  {
    title: "API Reference",
    links: [
      { title: "Overview", href: "/docs/api/overview" },
      { title: "Entities", href: "/docs/api/entities" },
      { title: "OAuth & DPoP", href: "/docs/api/oauth" },
      { title: "Credentials", href: "/docs/api/credentials" },
      { title: "Revocations", href: "/docs/api/revocations" },
      { title: "DIDComm", href: "/docs/api/didcomm" },
      { title: "Discovery", href: "/docs/api/discovery" },
    ],
  },
  {
    title: "SDKs",
    links: [
      { title: "Overview", href: "/docs/sdks/overview" },
      { title: "Go", href: "/docs/sdks/go" },
      { title: "JavaScript", href: "/docs/sdks/javascript" },
      { title: "Python", href: "/docs/sdks/python" },
    ],
  },
];
