import fs from "fs";
import path from "path";
import { compileMDX } from "next-mdx-remote/rsc";
import remarkGfm from "remark-gfm";
import rehypeSlug from "rehype-slug";
import rehypePrettyCode from "rehype-pretty-code";

const BLOG_DIR = path.join(process.cwd(), "src/content/blog");

export type BlogMeta = {
  title: string;
  description: string;
  date: string;
};

export async function getBlogPost(slug: string) {
  const filePath = path.join(BLOG_DIR, `${slug}.mdx`);
  if (!fs.existsSync(filePath)) return null;

  const source = fs.readFileSync(filePath, "utf-8");
  const { content, frontmatter } = await compileMDX<BlogMeta>({
    source,
    options: {
      parseFrontmatter: true,
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

  return { content, frontmatter };
}

export function getAllBlogSlugs(): string[] {
  if (!fs.existsSync(BLOG_DIR)) return [];
  return fs
    .readdirSync(BLOG_DIR)
    .filter((f) => f.endsWith(".mdx"))
    .map((f) => f.replace(/\.mdx$/, ""));
}

export async function getAllBlogPosts() {
  const slugs = getAllBlogSlugs();
  const posts = await Promise.all(
    slugs.map(async (slug) => {
      const post = await getBlogPost(slug);
      return post ? { slug, ...post.frontmatter } : null;
    })
  );
  return posts
    .filter((p): p is NonNullable<typeof p> => p !== null)
    .sort((a, b) => new Date(b.date).getTime() - new Date(a.date).getTime());
}
