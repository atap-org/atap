import { Sidebar } from "@/components/layout/sidebar";

export default function DocsLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="mx-auto max-w-6xl px-4 py-8 md:py-12">
      <div className="flex gap-12">
        <aside className="hidden w-56 shrink-0 md:block">
          <div className="sticky top-20">
            <Sidebar />
          </div>
        </aside>
        <article className="min-w-0 flex-1">{children}</article>
      </div>
    </div>
  );
}
