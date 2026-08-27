# Framework research

Groundwork for Volt: exhaustive feature inventories of four reference
full-stack frameworks — **Laravel 13.x**, **Rails 8.1.3**, **Phoenix
1.8.9** (with Ecto, LiveView, Plug), **Django 6.1.x** — plus the **Go
standard library** baseline (go1.26.5) and a cross-framework synthesis.
Before Volt commits to a feature, this answers "what does a
batteries-included framework actually contain?"

The inventories were distilled from a local corpus of each framework's
official documentation sources (fetched and pinned 2026-07-14). The
corpus and its fetch scripts have been removed from the repository —
they were ~50MB of other people's docs; each inventory's header records
the exact upstream repo, ref, and commit it was derived from, so the
corpus is reproducible from that provenance if ever needed.

## `features/`

One document per stack, all following the same skeleton (P1 routing …
P21 admin, shared ID prefixes like `ROUTE-`, `ORM-`, `AUTH-`):
problem-oriented sections, one terse row per feature with a stable ID, a
tier marker, and notes.

- [`gostd.md`](features/gostd.md) — **the baseline**: what Go std, the
  toolchain and `golang.org/x` already give (tiers `STD`/`TOOL`/`X`/
  `NO`). The `NO` rows are the point: they mark candidate Volt surface.
  Its closing section, "The gap Volt fills", is the short version.
- [`laravel.md`](features/laravel.md), [`rails.md`](features/rails.md),
  [`phoenix.md`](features/phoenix.md), [`django.md`](features/django.md)
  — per-framework inventories (tiers `CORE`/`OPT`/`ECO`/`DIY`).
- [`comparison.md`](features/comparison.md) — the synthesis: every
  capability matrix leads with the Go std column, so each row reads
  "what Go gives for free vs how the four frameworks answer it"; closes
  with table stakes, differentiators, gaps, and the design tensions
  Volt must resolve.

Known blind spot: the inventories reflect the framework *guides*, not
the generated per-module API references (api.rubyonrails.org, hexdocs
module pages) — capabilities documented only in doc comments appear
only where the guides name them.
