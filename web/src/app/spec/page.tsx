import type { Metadata } from "next";
import { getSpec } from "@/lib/spec";
import { SpecToc } from "./toc";

export const metadata: Metadata = {
  title: "Specification",
  description: "ATAP Protocol Specification v1.0-rc1",
};

export default async function SpecPage() {
  const { content, headings } = await getSpec();

  if (!content) {
    return (
      <div className="mx-auto max-w-6xl px-4 py-8 md:py-12">
        <p className="text-muted-foreground">
          Specification not available. See the{" "}
          <a href="https://github.com/8upio/atap/blob/main/spec/ATAP-SPEC-v1.0-rc1.md" className="underline">
            source on GitHub
          </a>.
        </p>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-6xl px-4 py-8 md:py-12">
      <div className="flex gap-12">
        <article className="prose prose-sm dark:prose-invert min-w-0 max-w-none flex-1">
          {content}
        </article>
        <aside className="hidden w-56 shrink-0 lg:block">
          <div className="sticky top-20">
            <SpecToc headings={headings} />
          </div>
        </aside>
      </div>
    </div>
  );
}
