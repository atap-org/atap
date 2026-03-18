export function Standards() {
  const standards = [
    { layer: "Identity", standard: "W3C DIDs (did:web)", role: "Entity addressing and key discovery" },
    { layer: "Claims", standard: "W3C Verifiable Credentials 2.0", role: "Verified properties and relationships" },
    { layer: "Messaging", standard: "DIDComm v2.1", role: "Encrypted entity-to-entity communication" },
    { layer: "Authorization", standard: "OAuth 2.1 + DPoP", role: "API authentication and token management" },
    { layer: "Signatures", standard: "JWS + JCS", role: "Approval signatures (Ed25519)" },
    { layer: "Templates", standard: "Adaptive Cards", role: "Approval rendering on devices" },
  ];

  return (
    <section className="border-t border-border">
      <div className="mx-auto max-w-6xl px-4 py-16 md:py-20">
        <h2 className="text-2xl font-bold tracking-tight md:text-3xl">
          Built on standards
        </h2>
        <p className="mt-2 max-w-2xl text-muted-foreground">
          ATAP composes established, formally analyzed standards. Its sole
          contribution is the multi-signature approval model — everything else
          is composition.
        </p>

        <div className="mt-8 overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border text-left">
                <th className="pb-3 pr-4 font-medium text-muted-foreground">Layer</th>
                <th className="pb-3 pr-4 font-medium text-muted-foreground">Standard</th>
                <th className="pb-3 font-medium text-muted-foreground">Role</th>
              </tr>
            </thead>
            <tbody>
              {standards.map((s) => (
                <tr key={s.layer} className="border-b border-border">
                  <td className="py-3 pr-4 font-medium">{s.layer}</td>
                  <td className="py-3 pr-4 font-mono text-xs">{s.standard}</td>
                  <td className="py-3 text-muted-foreground">{s.role}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </section>
  );
}
