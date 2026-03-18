import { notFound } from "next/navigation";
import type { Metadata } from "next";
import Link from "next/link";
import { getBlogPost, getAllBlogSlugs } from "@/lib/blog";

type Props = {
  params: Promise<{ slug: string }>;
};

export async function generateStaticParams() {
  return getAllBlogSlugs().map((slug) => ({ slug }));
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { slug } = await params;
  const post = await getBlogPost(slug);
  if (!post) return {};
  return {
    title: post.frontmatter.title,
    description: post.frontmatter.description,
  };
}

export default async function BlogPost({ params }: Props) {
  const { slug } = await params;
  const post = await getBlogPost(slug);
  if (!post) return notFound();

  return (
    <div className="mx-auto max-w-2xl px-4 py-12 md:py-16">
      <Link
        href="/blog"
        className="text-sm text-muted-foreground hover:text-foreground"
      >
        &larr; Back to blog
      </Link>
      <article className="mt-6">
        <time className="text-sm text-muted-foreground">
          {post.frontmatter.date}
        </time>
        <h1 className="mt-2 text-3xl font-bold tracking-tight">
          {post.frontmatter.title}
        </h1>
        <div className="prose dark:prose-invert mt-8 max-w-none">
          {post.content}
        </div>
      </article>
    </div>
  );
}
