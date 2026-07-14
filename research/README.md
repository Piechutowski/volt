# Framework research

Groundwork for Volt: the complete documentation of four reference
full-stack frameworks — **Laravel**, **Rails**, **Phoenix** (with Ecto,
LiveView, Plug), and **Django** — as a local markdown corpus, plus
exhaustive feature inventories distilled from it. Before Volt commits to
a feature set, this answers "what does a batteries-included framework
actually contain?" the same way
[not-an-orm/docs/features.md](https://github.com/Piechutowski/not-an-orm/blob/main/docs/features.md)
answers it for the data layer.

## Layout

```
research/
├── scripts/     fetching + conversion tooling (re-runnable)
├── corpus/      the docs themselves, one directory per framework
└── features/    distilled feature inventories + cross-framework matrix
```

## The corpus

All four frameworks publish their documentation *source* on GitHub — the
same files their doc sites are rendered from — so nothing is scraped:
the scripts pull the sources directly and every corpus directory carries
a `MANIFEST` with the exact repo, ref, commit, and fetch date.

| Corpus | Source | Native format |
|---|---|---|
| `corpus/laravel/` | `laravel/docs` (branch per release line) | Markdown |
| `corpus/rails/` | `rails/rails` → `guides/source/` | Markdown |
| `corpus/phoenix/{phoenix,liveview,ecto,plug}/` | framework + ecosystem repos → `guides/` | Markdown |
| `corpus/django/` | `django/django` → `docs/` | Sphinx RST → converted |

Django is the one conversion: its docs are reStructuredText (`.txt`),
converted file-by-file with `pandoc -f rst -t gfm`. Sphinx-specific
roles (`:setting:`, `:ref:`, …) degrade to plain text; anything pandoc
cannot convert is copied verbatim and listed in
`corpus/django/CONVERSION_FAILURES` (currently: none).

Phoenix is fetched together with the libraries a real Phoenix app is
inseparable from — Ecto (data), LiveView (real-time UI), Plug (HTTP) —
because Phoenix core is deliberately thin and comparing it bare against
batteries-included frameworks would mislead.

## Refreshing / re-pinning

```sh
cd research/scripts
./fetch_all.sh                    # everything at latest stable
LARAVEL_REF=12.x ./fetch_laravel.sh   # or pin any ref per framework
RAILS_REF=main   ./fetch_rails.sh     # edge guides
```

Requirements: `git`, `pandoc` (Django only). Version resolution is
automatic — highest stable branch/tag via `git ls-remote` — and each
script accepts an env override (`LARAVEL_REF`, `RAILS_REF`,
`PHOENIX_REF`, `LIVEVIEW_REF`, `ECTO_REF`, `PLUG_REF`, `DJANGO_REF`).

### Known blind spot: generated API references

The guides above are the feature-bearing docs, but each ecosystem also
has a generated API reference (api.rubyonrails.org, api.laravel.com,
hexdocs module pages) whose text exists only as comments in the source
code — RDoc / PHPDoc / `@moduledoc` — rendered to HTML at release
time, with no doc files to fetch. The corpus and the inventories built
from it therefore reflect what the guides document. If that layer is
ever needed: for the Elixir libraries the clean path is extracting
`@moduledoc`/`@doc` strings (they are markdown) from a source clone;
for Rails/Laravel it would mean scraping the rendered HTML.

## Feature inventories (`features/`)

One document per framework — `laravel.md`, `rails.md`, `phoenix.md`,
`django.md` — plus `comparison.md`, a cross-framework matrix. All four
follow the same skeleton (P1 routing … P21 admin, shared ID prefixes
like `ROUTE-`, `ORM-`, `AUTH-`), in the style of not-an-orm's
`features.md`: problem-oriented sections, one terse row per feature with
a stable ID, a tier marker (`CORE` / `OPT` / `ECO` / `DIY`), and source
file references back into the corpus. The shared skeleton is what makes
the comparison matrix line up row-for-row.
