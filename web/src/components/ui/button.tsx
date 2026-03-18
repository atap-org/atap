import Link from "next/link";

type ButtonProps = {
  href: string;
  variant?: "primary" | "secondary";
  external?: boolean;
  children: React.ReactNode;
};

export function Button({
  href,
  variant = "primary",
  external,
  children,
}: ButtonProps) {
  const className =
    variant === "primary"
      ? "inline-flex items-center rounded-lg bg-accent px-4 py-2 text-sm font-medium text-accent-foreground transition-opacity hover:opacity-90"
      : "inline-flex items-center rounded-lg border border-border px-4 py-2 text-sm font-medium text-foreground transition-colors hover:bg-muted";

  if (external) {
    return (
      <a href={href} target="_blank" rel="noopener noreferrer" className={className}>
        {children}
      </a>
    );
  }

  return (
    <Link href={href} className={className}>
      {children}
    </Link>
  );
}
