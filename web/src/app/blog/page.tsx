import type { Metadata } from "next";
import Link from "next/link";
import { getAllBlogPosts } from "@/lib/blog";

export const metadata: Metadata = {
  title: "Blog",
  description: "ATAP protocol updates and announcements.",
};

export default async function BlogIndex() {
  const posts = await getAllBlogPosts();

  return (
    <div className="mx-auto max-w-2xl px-4 py-12 md:py-16">
      <h1 className="text-3xl font-bold tracking-tight">Blog</h1>
      <p className="mt-2 text-muted-foreground">
        Protocol updates and announcements.
      </p>

      <div className="mt-10 space-y-8">
        {posts.map((post) => (
          <article key={post.slug}>
            <Link href={`/blog/${post.slug}`} className="group block">
              <time className="text-sm text-muted-foreground">{post.date}</time>
              <h2 className="mt-1 text-xl font-semibold group-hover:text-accent">
                {post.title}
              </h2>
              <p className="mt-1 text-muted-foreground">{post.description}</p>
            </Link>
          </article>
        ))}
      </div>
    </div>
  );
}
