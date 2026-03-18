/**
 * Post-build script: generates .md files in out/ for every content page.
 * Nginx serves these when Accept: text/markdown is requested.
 */
import fs from "fs";
import path from "path";

const OUT = path.resolve("out");
const CONTENT_DOCS = path.resolve("src/content/docs");
const CONTENT_BLOG = path.resolve("src/content/blog");
const SPEC_PATHS = [
  path.resolve("..", "spec", "ATAP-SPEC-v1.0-rc1.md"),
  path.resolve("spec", "ATAP-SPEC-v1.0-rc1.md"),
];

function ensureDir(dir) {
  fs.mkdirSync(dir, { recursive: true });
}

/** Strip frontmatter from MDX, return { frontmatter, body } */
function parseMdx(content) {
  const match = content.match(/^---\n([\s\S]*?)\n---\n([\s\S]*)$/);
  if (!match) return { frontmatter: {}, body: content };
  const fm = {};
  for (const line of match[1].split("\n")) {
    const idx = line.indexOf(":");
    if (idx > 0) {
      const key = line.slice(0, idx).trim();
      const val = line.slice(idx + 1).trim().replace(/^["']|["']$/g, "");
      fm[key] = val;
    }
  }
  return { frontmatter: fm, body: match[2] };
}

/** Strip JSX import lines and component tags, keep markdown */
function mdxToMarkdown(body) {
  return body
    .replace(/^import\s+.*$/gm, "")
    .replace(/<Callout[^>]*>([\s\S]*?)<\/Callout>/g, "> $1")
    .replace(/<ApiEndpoint[^/]*\/>/g, "")
    .replace(/<[A-Z][^>]*>/g, "")
    .replace(/<\/[A-Z][^>]*>/g, "")
    .replace(/\n{3,}/g, "\n\n")
    .trim();
}

// --- Docs ---
function walkDir(dir, prefix = []) {
  const results = [];
  if (!fs.existsSync(dir)) return results;
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    if (entry.isDirectory()) {
      results.push(...walkDir(path.join(dir, entry.name), [...prefix, entry.name]));
    } else if (entry.name.endsWith(".mdx")) {
      results.push({
        slug: [...prefix, entry.name.replace(/\.mdx$/, "")],
        filePath: path.join(dir, entry.name),
      });
    }
  }
  return results;
}

let count = 0;

for (const { slug, filePath } of walkDir(CONTENT_DOCS)) {
  const raw = fs.readFileSync(filePath, "utf-8");
  const { frontmatter, body } = parseMdx(raw);
  const title = frontmatter.title || slug[slug.length - 1];
  const md = `# ${title}\n\n${mdxToMarkdown(body)}\n`;
  const outPath = path.join(OUT, "docs", ...slug) + ".md";
  ensureDir(path.dirname(outPath));
  fs.writeFileSync(outPath, md);
  count++;
}

// --- Blog ---
for (const entry of walkDir(CONTENT_BLOG)) {
  const raw = fs.readFileSync(entry.filePath, "utf-8");
  const { frontmatter, body } = parseMdx(raw);
  const title = frontmatter.title || entry.slug[entry.slug.length - 1];
  const date = frontmatter.date || "";
  const md = `# ${title}\n\n${date ? `*${date}*\n\n` : ""}${mdxToMarkdown(body)}\n`;
  const outPath = path.join(OUT, "blog", ...entry.slug) + ".md";
  ensureDir(path.dirname(outPath));
  fs.writeFileSync(outPath, md);
  count++;
}

// --- Spec ---
const specFile = SPEC_PATHS.find((p) => fs.existsSync(p));
if (specFile) {
  const outPath = path.join(OUT, "spec", "index.md");
  ensureDir(path.dirname(outPath));
  fs.copyFileSync(specFile, outPath);
  count++;
}

// --- Homepage ---
const homepage = `# ATAP — Agent Trust and Authority Protocol

Open protocol for verifiable multi-party authorization in AI agent ecosystems.

ATAP defines multi-signature approvals for AI agent ecosystems. When an agent acts on behalf of a human, every party signs — producing a portable, cryptographic proof of consent that anyone can verify offline.

## How approvals work

A three-party approval produces a self-contained document with three independent signatures — verifiable by anyone, without calling back to any server.

1. **Agent (from) — Requester signs**: The agent creates an approval document describing the action and signs it with its Ed25519 key.
2. **System (via) — System co-signs**: The mediating system validates the request, checks business rules, and adds its signature — or rejects.
3. **Human (to) — Approver signs**: The human reviews the approval on their device, authenticates with biometrics, and signs to approve or decline.

## Four entity types

Every ATAP entity gets a \`did:web\` DID. The entity type determines lifecycle and routing behavior.

| Type | Description | Example DID |
|------|-------------|-------------|
| **agent** | Ephemeral software actor performing tasks on behalf of other entities | \`did:web:example.com:agent:travel-booker\` |
| **machine** | Persistent application or service with a long-running identity | \`did:web:airline.com:machine:booking\` |
| **human** | Natural person. ID derived from public key — no PII in the identifier | \`did:web:example.com:human:x7k9m2w4p3n8\` |
| **org** | Legal entity. Signals routed to delegates via the ATAP server | \`did:web:corp.com:org:engineering\` |

## Links

- [Documentation](https://atap.dev/docs/getting-started)
- [Specification](https://atap.dev/spec)
- [GitHub](https://github.com/atap-dev/atap)
`;
ensureDir(path.join(OUT));
fs.writeFileSync(path.join(OUT, "index.md"), homepage);
count++;

console.log(`Generated ${count} markdown files in out/`);
