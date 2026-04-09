import React from 'react';

export function StatusPill(props: { children: React.ReactNode; tone: "ready" | "watch" | "blocked" | "neutral" }) {
  return <span className={`status-pill status-pill-${props.tone}`}>{props.children}</span>;
}
