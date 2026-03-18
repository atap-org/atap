import { Button } from "@/components/ui/button";
import { URLS } from "@/lib/constants";

export function Hero() {
  return (
    <section className="mx-auto max-w-6xl px-4 pb-16 pt-20 md:pt-28">
      <div className="max-w-3xl">
        <p className="text-sm font-medium text-accent">Open Protocol &middot; v1.0-rc1</p>
        <h1 className="mt-3 text-4xl font-bold tracking-tight md:text-5xl lg:text-6xl">
          Verifiable trust for AI&nbsp;agents
        </h1>
        <p className="mt-4 text-lg text-muted-foreground md:text-xl">
          ATAP defines multi-signature approvals for AI agent ecosystems.
          When an agent acts on behalf of a human, every party signs —
          producing a portable, cryptographic proof of consent that anyone
          can verify offline.
        </p>
        <div className="mt-8 flex flex-wrap gap-3">
          <Button href={URLS.DOCS}>Get Started</Button>
          <Button href={URLS.SPEC} variant="secondary">
            Read the Spec
          </Button>
          <Button href={URLS.GITHUB} variant="secondary" external>
            GitHub
          </Button>
        </div>
      </div>
    </section>
  );
}
