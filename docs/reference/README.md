# Reference

Pinned-subject research: consulted, not maintained. Each document's
header records the exact upstream source, ref, commit and fetch date it
was derived from; the vendored documentation corpora themselves were
removed from the repository (reproducible from that provenance).

- [`comparison.md`](comparison.md) — the synthesis: every capability
  matrix leads with the Go std column; closes with table stakes,
  differentiators, gaps, and the design tensions Volt must resolve.
- [`gostd.md`](gostd.md) — the baseline: what Go std, the toolchain and
  `golang.org/x` already give (`STD`/`TOOL`/`X`/`NO`); the `NO` rows
  mark candidate Volt surface.
- [`rails.md`](rails.md) · [`laravel.md`](laravel.md) ·
  [`phoenix.md`](phoenix.md) · [`django.md`](django.md) — per-framework
  feature inventories on a shared P1–P21 skeleton.
- [`orm-matrix.md`](orm-matrix.md) — every Rails Active Record
  capability with nao's verdict; the exhaustiveness audit behind the
  data-layer roadmap.

Measurements, pinned on the day they were taken:

- [`perf-sweep-2026-09-14.md`](perf-sweep-2026-09-14.md): every phase
  of the compiler timed and profiled from 10 to 160 tables, with the
  charts in [`perf-sweep/`](perf-sweep/).
- [`go-compiler-cost.md`](go-compiler-cost.md): what the thousand-table
  project generates and what the Go compiler does with it, the
  bottleneck that is not Volt's, and the levers that are.
