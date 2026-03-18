import { notFound, redirect } from "next/navigation";
import type { Metadata } from "next";
import { getDoc, getAllDocSlugs } from "@/lib/mdx";

type Props = {
  params: Promise<{ slug?: string[] }>;
};

export async function generateStaticParams() {
  const slugs = getAllDocSlugs();
  // Include the base /docs route (no slug) for the redirect
  return [{ slug: undefined }, ...slugs.map((slug) => ({ slug }))];
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { slug } = await params;
  if (!slug) return { title: "Documentation" };
  const doc = await getDoc(slug);
  if (!doc) return {};
  return {
    title: doc.frontmatter.title,
    description: doc.frontmatter.description,
  };
}

export default async function DocPage({ params }: Props) {
  const { slug } = await params;
  if (!slug) redirect("/docs/getting-started");

  const doc = await getDoc(slug);
  if (!doc) return notFound();

  return (
    <div>
      <h1 className="text-3xl font-bold tracking-tight">
        {doc.frontmatter.title}
      </h1>
      {doc.frontmatter.description && (
        <p className="mt-2 text-lg text-muted-foreground">
          {doc.frontmatter.description}
        </p>
      )}
      <div className="mt-8">{doc.content}</div>
    </div>
  );
}
