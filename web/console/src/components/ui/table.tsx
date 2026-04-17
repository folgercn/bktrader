import * as React from "react"

import { cn } from "../../lib/utils"

function extractTextContent(node: React.ReactNode): string {
  if (typeof node === "string" || typeof node === "number") {
    return String(node)
  }

  if (Array.isArray(node)) {
    return node.map(extractTextContent).join(" ").trim()
  }

  if (React.isValidElement(node)) {
    return extractTextContent(node.props.children)
  }

  return ""
}

function isPrimitiveCellContent(node: React.ReactNode): node is string | number {
  return typeof node === "string" || typeof node === "number"
}

function Table({
  className,
  tone = "default",
  ...props
}: React.ComponentProps<"table"> & {
  tone?: "default" | "bento"
}) {
  return (
    <div
      data-slot="table-container"
      data-tone={tone}
      className="group/table-container relative w-full overflow-x-auto"
    >
      <table
        data-slot="table"
        data-tone={tone}
        className={cn(
          "w-full caption-bottom text-sm data-[tone=bento]:text-[var(--bk-text-primary)]",
          className
        )}
        {...props}
      />
    </div>
  )
}

function TableHeader({ className, ...props }: React.ComponentProps<"thead">) {
  return (
    <thead
      data-slot="table-header"
      className={cn("[&_tr]:border-b", className)}
      {...props}
    />
  )
}

function TableBody({ className, ...props }: React.ComponentProps<"tbody">) {
  return (
    <tbody
      data-slot="table-body"
      className={cn("[&_tr:last-child]:border-0", className)}
      {...props}
    />
  )
}

function TableFooter({ className, ...props }: React.ComponentProps<"tfoot">) {
  return (
    <tfoot
      data-slot="table-footer"
      className={cn(
        "border-t bg-muted/50 font-medium [&>tr]:last:border-b-0 group-data-[tone=bento]/table-container:border-[var(--bk-border)] group-data-[tone=bento]/table-container:bg-[var(--bk-surface-muted)]",
        className
      )}
      {...props}
    />
  )
}

function TableRow({ className, ...props }: React.ComponentProps<"tr">) {
  return (
    <tr
      data-slot="table-row"
      className={cn(
        "border-b transition-colors hover:bg-muted/50 has-aria-expanded:bg-muted/50 data-[state=selected]:bg-muted group-data-[tone=bento]/table-container:border-[var(--bk-border-soft)] group-data-[tone=bento]/table-container:hover:bg-[var(--bk-surface-muted)]",
        className
      )}
      {...props}
    />
  )
}

function TableHead({ className, ...props }: React.ComponentProps<"th">) {
  const title = props.title ?? extractTextContent(props.children)

  return (
    <th
      data-slot="table-head"
      title={title || undefined}
      className={cn(
        "h-10 px-2 text-left align-middle font-medium whitespace-nowrap text-foreground [&:has([role=checkbox])]:pr-0 group-data-[tone=bento]/table-container:text-[var(--bk-text-muted)]",
        className
      )}
      {...props}
    />
  )
}

function TableCell({ className, ...props }: React.ComponentProps<"td">) {
  const title = props.title ?? extractTextContent(props.children)
  const children =
    props.children == null ? null : isPrimitiveCellContent(props.children) ? (
      <span className="block max-w-full overflow-hidden text-ellipsis whitespace-nowrap" title={title || undefined}>
        {props.children}
      </span>
    ) : (
      props.children
    )

  return (
    <td
      data-slot="table-cell"
      title={title || undefined}
      className={cn(
        "p-2 align-middle whitespace-nowrap [&:has([role=checkbox])]:pr-0",
        className
      )}
      {...props}
    >
      {children}
    </td>
  )
}

function TableCaption({
  className,
  ...props
}: React.ComponentProps<"caption">) {
  return (
    <caption
      data-slot="table-caption"
      className={cn("mt-4 text-sm text-muted-foreground", className)}
      {...props}
    />
  )
}

export {
  Table,
  TableHeader,
  TableBody,
  TableFooter,
  TableHead,
  TableRow,
  TableCell,
  TableCaption,
}
