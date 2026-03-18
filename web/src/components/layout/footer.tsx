import Link from "next/link";
import { URLS } from "@/lib/constants";

export function Footer() {
  return (
    <footer className="border-t border-border">
      <div className="mx-auto flex max-w-6xl flex-col gap-8 px-4 py-12 md:flex-row md:justify-between">
        <div>
          <p className="text-sm font-semibold">ATAP</p>
          <p className="mt-1 text-sm text-muted-foreground">
            Agent Trust and Authority Protocol
          </p>
        </div>

        <div className="flex gap-16">
          <div>
            <p className="text-sm font-semibold">Protocol</p>
            <ul className="mt-2 space-y-1.5">
              <li>
                <Link
                  href={URLS.SPEC}
                  className="text-sm text-muted-foreground hover:text-foreground"
                >
                  Specification
                </Link>
              </li>
              <li>
                <Link
                  href={URLS.DOCS}
                  className="text-sm text-muted-foreground hover:text-foreground"
                >
                  Documentation
                </Link>
              </li>
              <li>
                <Link
                  href="/blog"
                  className="text-sm text-muted-foreground hover:text-foreground"
                >
                  Blog
                </Link>
              </li>
            </ul>
          </div>
          <div>
            <p className="text-sm font-semibold">Resources</p>
            <ul className="mt-2 space-y-1.5">
              <li>
                <a
                  href={URLS.GITHUB}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-sm text-muted-foreground hover:text-foreground"
                >
                  GitHub
                </a>
              </li>
              <li>
                <a
                  href={URLS.API_SANDBOX}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-sm text-muted-foreground hover:text-foreground"
                >
                  Sandbox API
                </a>
              </li>
            </ul>
          </div>
        </div>
      </div>
      <div className="border-t border-border">
        <div className="mx-auto max-w-6xl px-4 py-4">
          <p className="text-xs text-muted-foreground">
            &copy; {new Date().getFullYear()} ATAP Contributors. Apache License
            2.0.
          </p>
        </div>
      </div>
    </footer>
  );
}
