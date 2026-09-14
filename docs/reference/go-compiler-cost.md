# The Go compiler on generated output

Measured 2026-09-14 on the `volt stress` default shape: one package,
a thousand tables of a hundred and fifty columns, routed through
groups. A pinned measurement, not a maintained doc: the numbers date
from the commit that wrote them.

## What Volt emits

| file | lines |
|---|---|
| `schema.volt` (the input) | 164,035 |
| `nao_models.go` | 434,015 |
| `nao_queries.go` (7,004 functions) | 366,047 |
| `nao_dyn.go` | 270,012 |
| `volt_router.go` | 103,047 |
| `nao_selects.go` | 53,011 |
| `nao_validate.go` and the rest | the remainder to 1,328,302 lines of Go |
| `nao_schema.sql` | 159,001 |

Seventy-six megabytes on disk; about 1,300 lines of Go per table,
almost all of it per-column repetition that the design asks for: no
reflection, no callbacks, every field, scan target, parameter struct,
setter and filter spelled out as plain Go (D03, D27).

## Where the time goes

Volt's own work on this input is seconds and hundreds of megabytes:
the check runs in 0.7 s cold and an edit costs 2 ms to parse and
under 200 ms to check; generation is linear in the output (100 ms at
160 tables in the sweep, not remeasured at this size since D81).

The Go compiler's work on the output is minutes and gigabytes. A Go
package is one compilation unit whose whole intermediate form lives in
memory at once: one compile of this package was observed at about
3.5 GB resident, and the ten-package split of the same schema,
compiling three packages at a time, exceeded a sixteen-gigabyte
machine and was killed; the one-package build was never allowed to
finish here and belongs on the maintainer's machine. Nothing done to
the checker, the parser or the memos moves any of this: the bottleneck
is the size of what is emitted, measured by a compiler that is not
ours.

## The levers, by leverage

1. **Emit data where the runtime is generic.** The runtime already
   renders SQL from column descriptors (`rt.Column`, `SelectRender`);
   a per-column method that only names a column is data written as
   code. Every such method turned into a descriptor literal removes a
   function per column per table from the compiler's input.
2. **Generate only what a project uses.** `-parts` already selects the
   outputs; a project without dynamic queries or without checks should
   not pay for `nao_dyn.go` or `nao_validate.go`.
3. **Split the project into packages.** Volt cannot choose the layout
   (a library with a compiler does not dictate where its output goes),
   but the author can: packages compile in parallel and each holds a
   fraction of the intermediate form.
4. **Accept it at the sizes that occur.** A project of tens of tables
   builds in seconds; the stress shape exists to feel the limit, not
   to describe a project.

## What would tell more

The build itself, on a machine with the memory for it: `time go build
./...` and peak resident size at 100, 300 and 1,000 tables, one
package and ten, so the curve of the Go compiler against table count
sits beside the checker's in the sweep. The sweep harness has the
projects; it lacks only the machine.
