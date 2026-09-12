/**
 * A labelled control with a line of explanation under it.
 *
 * The hint is not optional decoration: every setting in this panel has a reason
 * an operator cannot infer from its name, and the space under the control is
 * where that reason goes.
 */
export function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint: string;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-1.5">
      <span className="stencil">{label}</span>
      {children}
      {hint && <p className="max-w-prose text-2xs text-bone-faint">{hint}</p>}
    </div>
  );
}
