import fs from "fs";
import path from "path";
import { compileMDX } from "next-mdx-remote/rsc";
import remarkGfm from "remark-gfm";
import rehypeSlug from "rehype-slug";
import rehypePrettyCode from "rehype-pretty-code";

const SPEC_PATHS = [
  path.join(process.cwd(), "..", "spec", "ATAP-SPEC-v1.0-rc1.md"),
  path.join(process.cwd(), "spec", "ATAP-SPEC-v1.0-rc1.md"),
];

export async function getSpec() {
  const specPath = SPEC_PATHS.find((p) => fs.existsSync(p));
  if (!specPath) {
    return { content: null, headings: [] };
  }
  const source = fs.readFileSync(specPath, "utf-8");

  // Extract headings for table of contents
  const headings: { id: string; text: string; level: number }[] = [];
  const headingRegex = /^(#{2,3})\s+(.+)$/gm;
  let match;
  while ((match = headingRegex.exec(source)) !== null) {
    const level = match[1].length;
    const text = match[2].trim();
    const id = text
      .toLowerCase()
      .replace(/[^\w\s-]/g, "")
      .replace(/\s+/g, "-");
    headings.push({ id, text, level });
  }

  const { content } = await compileMDX({
    source,
    options: {
      mdxOptions: {
        remarkPlugins: [remarkGfm],
        rehypePlugins: [
          rehypeSlug,
          [
            rehypePrettyCode,
            {
              theme: {
                dark: "github-dark",
                light: "github-light",
              },
              keepBackground: false,
            },
          ],
        ],
      },
    },
  });

  return { content, headings };
}
