import * as React from "react"

import { cn } from "../../lib/utils"

function Card({
  className,
  size = "default",
  tone = "default",
  ...props
}: React.ComponentProps<"div"> & {
  size?: "default" | "sm"
  tone?: "default" | "bento" | "subtle"
}) {
  return (
    <div
      data-slot="card"
      data-size={size}
      data-tone={tone}
      className={cn(
        "group/card flex flex-col gap-4 overflow-hidden rounded-xl py-4 text-sm has-data-[slot=card-footer]:pb-0 has-[>img:first-child]:pt-0 data-[size=sm]:gap-3 data-[size=sm]:py-3 data-[size=sm]:has-data-[slot=card-footer]:pb-0 *:[img:first-child]:rounded-t-xl *:[img:last-child]:rounded-b-xl",
        "data-[tone=default]:bg-card data-[tone=default]:text-card-foreground data-[tone=default]:ring-1 data-[tone=default]:ring-foreground/10",
        "data-[tone=bento]:bg-[var(--bk-surface)] data-[tone=bento]:text-[var(--bk-text-primary)] data-[tone=bento]:shadow-[var(--bk-shadow-card)] data-[tone=bento]:ring-1 data-[tone=bento]:ring-[var(--bk-border-strong)]",
        "data-[tone=subtle]:bg-[var(--bk-surface-muted)] data-[tone=subtle]:text-[var(--bk-text-primary)] data-[tone=subtle]:ring-1 data-[tone=subtle]:ring-[var(--bk-border)]",
        className
      )}
      {...props}
    />
  )
}

function CardHeader({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="card-header"
      className={cn(
        "group/card-header @container/card-header grid auto-rows-min items-start gap-1 rounded-t-xl px-4 group-data-[size=sm]/card:px-3 has-data-[slot=card-action]:grid-cols-[1fr_auto] has-data-[slot=card-description]:grid-rows-[auto_auto] [.border-b]:pb-4 group-data-[size=sm]/card:[.border-b]:pb-3",
        className
      )}
      {...props}
    />
  )
}

function CardTitle({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="card-title"
      className={cn(
        "font-heading text-base leading-snug font-medium group-data-[size=sm]/card:text-sm",
        className
      )}
      {...props}
    />
  )
}

function CardDescription({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="card-description"
      className={cn("text-sm text-muted-foreground", className)}
      {...props}
    />
  )
}

function CardAction({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="card-action"
      className={cn(
        "col-start-2 row-span-2 row-start-1 self-start justify-self-end",
        className
      )}
      {...props}
    />
  )
}

function CardContent({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="card-content"
      className={cn("px-4 group-data-[size=sm]/card:px-3", className)}
      {...props}
    />
  )
}

function CardFooter({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="card-footer"
      className={cn(
        "flex items-center rounded-b-xl border-t bg-muted/50 p-4 group-data-[size=sm]/card:p-3 group-data-[tone=bento]/card:border-[var(--bk-border)] group-data-[tone=bento]/card:bg-[var(--bk-surface-muted)] group-data-[tone=subtle]/card:border-[var(--bk-border-soft)] group-data-[tone=subtle]/card:bg-white/30",
        className
      )}
      {...props}
    />
  )
}

export {
  Card,
  CardHeader,
  CardFooter,
  CardTitle,
  CardAction,
  CardDescription,
  CardContent,
}
