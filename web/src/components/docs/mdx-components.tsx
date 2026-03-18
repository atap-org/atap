import type { MDXComponents } from "mdx/types";
import { Callout } from "./callout";
import { ApiEndpoint } from "./api-endpoint";

export const mdxComponents: MDXComponents = {
  h1: (props) => (
    <h1 className="mt-8 text-3xl font-bold tracking-tight" {...props} />
  ),
  h2: (props) => (
    <h2
      className="mt-10 scroll-mt-20 border-b border-border pb-2 text-2xl font-semibold tracking-tight"
      {...props}
    />
  ),
  h3: (props) => (
    <h3 className="mt-8 scroll-mt-20 text-xl font-semibold tracking-tight" {...props} />
  ),
  h4: (props) => (
    <h4 className="mt-6 scroll-mt-20 text-lg font-semibold" {...props} />
  ),
  p: (props) => <p className="mt-4 leading-7" {...props} />,
  ul: (props) => <ul className="mt-4 list-disc pl-6 leading-7" {...props} />,
  ol: (props) => <ol className="mt-4 list-decimal pl-6 leading-7" {...props} />,
  li: (props) => <li className="mt-1" {...props} />,
  a: (props) => (
    <a
      className="text-accent underline underline-offset-4 hover:opacity-80"
      {...props}
    />
  ),
  table: (props) => (
    <div className="mt-4 overflow-x-auto">
      <table className="w-full text-sm" {...props} />
    </div>
  ),
  thead: (props) => <thead className="border-b border-border" {...props} />,
  th: (props) => (
    <th
      className="px-3 py-2 text-left font-medium text-muted-foreground"
      {...props}
    />
  ),
  td: (props) => (
    <td className="border-b border-border px-3 py-2" {...props} />
  ),
  code: (props) => {
    // Inline code (not inside pre)
    const isInline = typeof props.children === "string";
    if (isInline) {
      return (
        <code
          className="rounded bg-muted px-1.5 py-0.5 font-mono text-sm"
          {...props}
        />
      );
    }
    return <code {...props} />;
  },
  blockquote: (props) => (
    <blockquote
      className="mt-4 border-l-4 border-accent/50 pl-4 italic text-muted-foreground"
      {...props}
    />
  ),
  hr: () => <hr className="my-8 border-border" />,
  Callout,
  ApiEndpoint,
};
