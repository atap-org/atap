type ApiEndpointProps = {
  method: "GET" | "POST" | "DELETE" | "PUT" | "PATCH";
  path: string;
  auth?: string;
  description?: string;
};

const methodColors: Record<string, string> = {
  GET: "bg-green-500/10 text-green-600 dark:text-green-400",
  POST: "bg-blue-500/10 text-blue-600 dark:text-blue-400",
  DELETE: "bg-red-500/10 text-red-600 dark:text-red-400",
  PUT: "bg-yellow-500/10 text-yellow-600 dark:text-yellow-400",
  PATCH: "bg-orange-500/10 text-orange-600 dark:text-orange-400",
};

export function ApiEndpoint({ method, path, auth, description }: ApiEndpointProps) {
  return (
    <div className="mt-4 rounded-lg border border-border p-4">
      <div className="flex items-center gap-3">
        <span
          className={`rounded px-2 py-0.5 font-mono text-xs font-bold ${methodColors[method] || ""}`}
        >
          {method}
        </span>
        <code className="font-mono text-sm">{path}</code>
        {auth && (
          <span className="ml-auto rounded bg-muted px-2 py-0.5 text-xs text-muted-foreground">
            {auth}
          </span>
        )}
      </div>
      {description && (
        <p className="mt-2 text-sm text-muted-foreground">{description}</p>
      )}
    </div>
  );
}
