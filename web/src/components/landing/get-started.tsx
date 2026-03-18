import { Button } from "@/components/ui/button";
import { URLS } from "@/lib/constants";

export function GetStarted() {
  return (
    <section className="border-t border-border">
      <div className="mx-auto max-w-6xl px-4 py-16 text-center md:py-20">
        <h2 className="text-2xl font-bold tracking-tight md:text-3xl">
          Start implementing ATAP
        </h2>
        <p className="mx-auto mt-2 max-w-lg text-muted-foreground">
          Register an entity on the sandbox, send your first approval, and
          verify the signatures — all in a few minutes.
        </p>
        <div className="mt-8 flex flex-wrap justify-center gap-3">
          <Button href={URLS.DOCS}>Read the Docs</Button>
          <Button href={URLS.SPEC} variant="secondary">
            Protocol Spec
          </Button>
          <Button href={URLS.GITHUB} variant="secondary" external>
            GitHub
          </Button>
        </div>
        <p className="mt-6 text-sm text-muted-foreground">
          Sandbox API:{" "}
          <code className="rounded bg-muted px-1.5 py-0.5 text-xs font-mono">
            {URLS.API_SANDBOX}
          </code>
        </p>
      </div>
    </section>
  );
}
