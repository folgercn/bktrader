export function ActionButton(props: {
  label: string;
  disabled?: boolean;
  variant?: "ghost";
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      className={`action-button ${props.variant === "ghost" ? "action-button-ghost" : ""}`}
      disabled={props.disabled}
      onClick={props.onClick}
    >
      {props.label}
    </button>
  );
}
