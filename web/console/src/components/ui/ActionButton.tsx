export function ActionButton(props: {
  label: string;
  disabled?: boolean;
  variant?: "ghost" | "danger";
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      className={`action-button ${props.variant === "ghost" ? "action-button-ghost" : ""} ${props.variant === "danger" ? "action-button-danger" : ""}`}
      disabled={props.disabled}
      onClick={props.onClick}
    >
      {props.label}
    </button>
  );
}
