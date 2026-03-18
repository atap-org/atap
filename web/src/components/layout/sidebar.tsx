"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { navigation } from "@/lib/navigation";

export function Sidebar() {
  const pathname = usePathname();

  return (
    <nav className="space-y-6">
      {navigation.map((section) => (
        <div key={section.title}>
          <p className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            {section.title}
          </p>
          <ul className="mt-2 space-y-0.5">
            {section.links.map((link) => (
              <li key={link.href}>
                <Link
                  href={link.href}
                  className={`block rounded-md px-2.5 py-1.5 text-sm transition-colors ${
                    pathname === link.href
                      ? "bg-accent/10 font-medium text-accent"
                      : "text-muted-foreground hover:text-foreground"
                  }`}
                >
                  {link.title}
                </Link>
              </li>
            ))}
          </ul>
        </div>
      ))}
    </nav>
  );
}
