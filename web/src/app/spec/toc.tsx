"use client";

type Heading = {
  id: string;
  text: string;
  level: number;
};

export function SpecToc({ headings }: { headings: Heading[] }) {
  return (
    <nav>
      <p className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
        On this page
      </p>
      <ul className="mt-3 space-y-1">
        {headings.map((h) => (
          <li key={h.id}>
            <a
              href={`#${h.id}`}
              className={`block text-xs leading-relaxed text-muted-foreground transition-colors hover:text-foreground ${
                h.level === 3 ? "pl-3" : ""
              }`}
            >
              {h.text}
            </a>
          </li>
        ))}
      </ul>
    </nav>
  );
}
