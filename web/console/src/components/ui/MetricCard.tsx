export function MetricCard(props: { label: string; value: string; tone?: "accent" }) {
  return (
    <article className={`metric-card flex flex-col justify-center ${props.tone === "accent" ? "metric-card-accent" : ""}`}>
      <p className="opacity-70">{props.label}</p>
      <strong title={props.value}>{props.value}</strong>
    </article>
  );
}
