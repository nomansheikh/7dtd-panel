import { Fragment } from "react";
import { flexRender, type useTable } from "@tanstack/react-table";
import type { Player } from "@/lib/api";
import type { features } from "@/pages/players";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

/*
The same rows in two shapes.

Nine columns do not fit a phone: the table came out about 610 pixels wide in a
390 pixel window, so two thirds of the facts were behind a sideways drag. Below
the breakpoint each player is a stacked entry instead — name first, the rest as
labelled pairs.

Both are driven by the one table instance, so sorting, filtering and the column
definitions stay in a single place and cannot drift apart.
*/
/* v9's Table type takes the feature map as well as the row type, so it is read
   off the hook rather than spelled out and kept in step by hand. */
type PlayerTableInstance = ReturnType<typeof useTable<typeof features, Player>>;

export function PlayerTable({ table }: { table: PlayerTableInstance }) {
  const rows = table.getRowModel().rows;

  return (
    <div className="region min-h-0 flex-1 overflow-auto">
      <ul className="divide-y divide-border md:hidden">
        {rows.map((row) => {
          const [name, ...facts] = row.getAllCells();
          return (
            <li key={row.id} className="space-y-2 p-4">
              <div className="text-sm">
                {flexRender(name.column.columnDef.cell, name.getContext())}
              </div>
              <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1">
                {facts.map((cell) => (
                  <Fragment key={cell.id}>
                    <dt className="stencil self-baseline">
                      {String(cell.column.columnDef.header)}
                    </dt>
                    <dd className="min-w-0 text-right text-xs">
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </dd>
                  </Fragment>
                ))}
              </dl>
            </li>
          );
        })}
      </ul>

      <Table className="hidden md:table">
        <TableHeader>
          {table.getHeaderGroups().map((group) => (
            <TableRow key={group.id}>
              {group.headers.map((header) => (
                <TableHead key={header.id}>
                  {header.isPlaceholder ? null : (
                    <button
                      type="button"
                      className="flex items-center gap-1"
                      onClick={header.column.getToggleSortingHandler()}
                    >
                      {flexRender(header.column.columnDef.header, header.getContext())}
                      {{ asc: "↑", desc: "↓" }[header.column.getIsSorted() as string] ?? null}
                    </button>
                  )}
                </TableHead>
              ))}
            </TableRow>
          ))}
        </TableHeader>
        <TableBody>
          {rows.map((row) => (
            <TableRow key={row.id}>
              {row.getAllCells().map((cell) => (
                <TableCell key={cell.id}>
                  {flexRender(cell.column.columnDef.cell, cell.getContext())}
                </TableCell>
              ))}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
