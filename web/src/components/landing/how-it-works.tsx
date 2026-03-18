export function HowItWorks() {
  return (
    <section className="border-t border-border bg-muted/50">
      <div className="mx-auto max-w-6xl px-4 py-16 md:py-20">
        <h2 className="text-2xl font-bold tracking-tight md:text-3xl">
          How approvals work
        </h2>
        <p className="mt-2 max-w-2xl text-muted-foreground">
          A three-party approval produces a self-contained document with three
          independent signatures — verifiable by anyone, without calling back to
          any server.
        </p>

        {/* Flow diagram */}
        <div className="mt-12 grid gap-0 md:grid-cols-3">
          <Step
            number={1}
            role="Agent"
            roleType="from"
            label="Requester signs"
            description="The agent creates an approval document describing the action and signs it with its Ed25519 key."
          />
          <Step
            number={2}
            role="System"
            roleType="via"
            label="System co-signs"
            description="The mediating system validates the request, checks business rules, and adds its signature — or rejects."
          />
          <Step
            number={3}
            role="Human"
            roleType="to"
            label="Approver signs"
            description="The human reviews the approval on their device, authenticates with biometrics, and signs to approve or decline."
          />
        </div>

        {/* Result */}
        <div className="mt-10 rounded-lg border border-border bg-background p-6">
          <p className="text-sm font-medium text-muted-foreground">Result</p>
          <p className="mt-1 font-medium">
            A portable document with three JWS signatures — verifiable offline by
            anyone holding the document and access to DID resolution.
          </p>
          <pre className="mt-4 overflow-x-auto rounded-md bg-muted p-4 font-mono text-xs text-muted-foreground">
{`{
  "atap_approval": "1",
  "id": "apr_01JQXYZ...",
  "from": "did:web:example.com:agent:travel-booker",
  "to":   "did:web:example.com:human:x7k9m2w4p3n8",
  "via":  "did:web:airline.com:machine:booking",
  "subject": {
    "type": "com.airline.booking",
    "label": "Flight LH-1234 — €489.00",
    "reversible": false,
    "payload": { ... }
  },
  "signatures": {
    "from": "<JWS>",
    "via":  "<JWS>"
  }
}`}
          </pre>
        </div>
      </div>
    </section>
  );
}

function Step({
  number,
  role,
  roleType,
  label,
  description,
}: {
  number: number;
  role: string;
  roleType: string;
  label: string;
  description: string;
}) {
  return (
    <div className="relative flex flex-col border-l border-border py-6 pl-8 md:border-l-0 md:border-t md:pl-0 md:pt-8">
      {/* Connector dot */}
      <div className="absolute -left-1.5 top-6 h-3 w-3 rounded-full border-2 border-accent bg-background md:-top-1.5 md:left-0" />

      <div className="flex items-center gap-2">
        <span className="flex h-6 w-6 items-center justify-center rounded-full bg-accent text-xs font-bold text-accent-foreground">
          {number}
        </span>
        <span className="text-sm font-medium">{role}</span>
        <code className="rounded bg-muted px-1.5 py-0.5 text-xs text-muted-foreground">
          {roleType}
        </code>
      </div>
      <p className="mt-2 font-medium">{label}</p>
      <p className="mt-1 text-sm text-muted-foreground">{description}</p>
    </div>
  );
}
