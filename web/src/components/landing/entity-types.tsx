export function EntityTypes() {
  const entities = [
    {
      type: "agent",
      label: "Agent",
      description: "Ephemeral software actor performing tasks on behalf of other entities.",
      example: "did:web:example.com:agent:travel-booker",
    },
    {
      type: "machine",
      label: "Machine",
      description: "Persistent application or service with a long-running identity.",
      example: "did:web:airline.com:machine:booking",
    },
    {
      type: "human",
      label: "Human",
      description: "Natural person. ID derived from public key — no PII in the identifier.",
      example: "did:web:example.com:human:x7k9m2w4p3n8",
    },
    {
      type: "org",
      label: "Organization",
      description: "Legal entity. Signals routed to delegates via the ATAP server.",
      example: "did:web:corp.com:org:engineering",
    },
  ];

  return (
    <section className="border-t border-border bg-muted/50">
      <div className="mx-auto max-w-6xl px-4 py-16 md:py-20">
        <h2 className="text-2xl font-bold tracking-tight md:text-3xl">
          Four entity types
        </h2>
        <p className="mt-2 max-w-2xl text-muted-foreground">
          Every ATAP entity gets a <code className="rounded bg-muted px-1.5 py-0.5 text-xs font-mono">did:web</code> DID.
          The entity type determines lifecycle and routing behavior.
        </p>

        <div className="mt-8 grid gap-4 md:grid-cols-2">
          {entities.map((e) => (
            <div
              key={e.type}
              className="rounded-lg border border-border bg-background p-5"
            >
              <div className="flex items-center gap-2">
                <code className="rounded bg-accent/10 px-2 py-0.5 text-sm font-semibold text-accent">
                  {e.type}
                </code>
                <span className="font-medium">{e.label}</span>
              </div>
              <p className="mt-2 text-sm text-muted-foreground">{e.description}</p>
              <code className="mt-3 block truncate text-xs text-muted-foreground">
                {e.example}
              </code>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
