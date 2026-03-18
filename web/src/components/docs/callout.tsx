type CalloutProps = {
  type?: "info" | "warning" | "danger";
  children: React.ReactNode;
};

const styles = {
  info: "border-accent/50 bg-accent/5",
  warning: "border-yellow-500/50 bg-yellow-500/5",
  danger: "border-red-500/50 bg-red-500/5",
};

const labels = {
  info: "Note",
  warning: "Warning",
  danger: "Danger",
};

export function Callout({ type = "info", children }: CalloutProps) {
  return (
    <div className={`mt-4 rounded-lg border-l-4 p-4 ${styles[type]}`}>
      <p className="text-sm font-semibold">{labels[type]}</p>
      <div className="mt-1 text-sm">{children}</div>
    </div>
  );
}
