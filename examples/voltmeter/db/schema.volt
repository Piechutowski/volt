package db

type units.Millis float64 from "github.com/Piechutowski/volt/examples/voltmeter/units"
type units.Commit string  from "github.com/Piechutowski/volt/examples/voltmeter/units"
type Phase       string
type Health string

table_partial stamped {
  created_at time.Time [default: `CURRENT_TIMESTAMP`, note: "When the row was stored, by the database clock"]
}

table_partial measured {
  run_id    int          [pk]
  tables    int          [pk, note: "Tables in the generated stress project"]
  wall_ms   units.Millis [note: "Best wall time over the repetitions"]
  alloc_mb  float64      [note: "Bytes allocated by one run, in MiB"]
  heap_mb   float64      [note: "Heap in use right after the run, in MiB"]
  allocs    int          [note: "Allocations by one run"]
  output_mb float64      [default: 0, note: "Bytes emitted, generate only, in MiB"]
  source_kb float64      [note: "Size of the project's schema.volt, in KiB"]

  indexes {
    (tables, run_id)
  }

  validators {
    MillisMeasured(wall_ms)
  }
}

table users [note: "A team member who reads and annotates runs."] {
  id    int    [pk]
  email string [required, unique, note: "Sign-in name, one per member"]
  name  string [required]
  ~stamped

  validators {
    EmailShape(email)
  }

  records (id, email, name) {
    1, "ada@voltmeter.test", "Ada"
    2, "grace@voltmeter.test", "Grace"
    3, "ken@voltmeter.test", "Ken"
  }
}

table machines [note: "The machine a run measured on, as perfsweep.Info reports it."] {
  id        int     [pk]
  cpu       string  [note: "The processor's model name"]
  cores     int     [note: "Logical CPUs the Go runtime sees"]
  memory_gb float64
  os        string  [note: "The system and its version, kernel included"]
  arch      string
  go        string  [note: "runtime.Version() of the measuring binary"]
  ~stamped

  indexes {
    (cpu, cores, memory_gb, os, arch, go) [unique]
  }
}

table tags [note: "A label a run can carry, such as a release or an experiment."] {
  slug string [pk]
  name string [required]

  indexes {
    `lower(name)` [unique, name: "idx_tags_name_ci"]
  }

  records (slug, name) {
    "baseline", "Baseline"
  }
}

table runs [note: """
  One collection run: the sweep, the keystroke benchmark and the
  latency samples of one commit on one machine. Its id is never reused,
  since exported reports cite it.
"""] {
  id          int          [pk, autoincrement]
  commit      units.Commit [index, note: "The measured commit, lower-case hex"]
  dirty       bool         [default: false, note: "The tree had uncommitted changes"]
  machine_id  int
  tag         string       [null]
  baseline_id int          [null, note: "The run this one is compared against"]
  note        string       [null]
  ~stamped
}

ref: runs.machine_id > machines.id [delete: cascade]
ref: runs.tag > tags.slug [delete: set_null, update: cascade]
ref: runs.baseline_id > runs.id [delete: set_null, reverse: followers]

table ms_load [note: "The load phase of the sweep at one size."] {
  ~measured
}

table ms_check [note: "The check phase of the sweep at one size."] {
  ~measured
}

table ms_vet [note: "The vet phase of the sweep at one size."] {
  ~measured
}

table ms_generate [note: "The generate phase of the sweep at one size."] {
  ~measured
}

ref: ms_load.run_id > runs.id [delete: cascade]
ref: ms_check.run_id > runs.id [delete: cascade]
ref: ms_vet.run_id > runs.id [delete: cascade]
ref: ms_generate.run_id > runs.id [delete: cascade]

table keystrokes [note: "The language server's keystroke benchmark at one size."] {
  run_id        int     [pk]
  tables        int     [pk]
  ms_per_op     float64
  mb_per_op     float64
  allocs_per_op int
}

ref: keystrokes.run_id > runs.id [delete: cascade]

table commits [note: "A commit of Volt's history whose lines were counted."] {
  sha     string    [pk]
  subject string
  author  string
  at      time.Time [note: "The author date"]

  indexes {
    `lower(author)` [name: "idx_commits_author_ci"]
  }
}

table line_counts [note: "The thesis numbers of one fixture at one commit: what Volt writes against what a person writes."] {
  sha        string [pk]
  fixture    string [pk, note: "The fixture's directory, such as itest/blog"]
  volt_lines int    [note: "Lines of .volt files"]
  hand_lines int    [note: "Lines of hand-written Go"]
  gen_lines  int    [note: "Lines of Go carrying the generated marker"]
}

ref: line_counts.sha > commits.sha [delete: cascade, as: commit]

table samples [note: "One timed call of a Voltmeter route by the collector."] {
  id     int     [pk]
  run_id int
  route  string
  status int
  ms     float64
  at     int     [note: "Unix milliseconds when the call started"]

  indexes {
    (route, at) [name: "idx_samples_failures", where: `status >= 400`]
  }
}

ref: samples.run_id > runs.id [delete: cascade]

table budgets [note: "A ceiling on one phase's wall time at one size."] {
  id     int     [pk]
  phase  Phase
  tables int
  max_ms float64
  state  Health  [default: "ok", note: "breached once a run went over"]

  indexes {
    (phase, tables) [unique]
  }

  validators {
    PhaseKnown(phase)
    HealthKnown(state)
    BudgetSane(tables, max_ms)
  }
}

table breaches [note: "A run that went over a budget."] {
  id        int     [pk]
  budget_id int
  run_id    int
  wall_ms   float64
  ~stamped
}

ref: breaches.budget_id > budgets.id [delete: cascade]
ref: breaches.run_id > runs.id [delete: cascade]

table dashboards [note: "A member's arrangement of charts."] {
  id       int    [pk]
  owner_id int    [ref: users.id]
  name     string [required]

  indexes {
    (owner_id, name) [unique]
  }
}

table widgets [note: "One chart on a dashboard."] {
  id           int    [pk]
  dashboard_id int
  position     int
  phase        Phase
  tables       int

  validators {
    PhaseKnown(phase)
  }
}

ref: widgets.dashboard_id > dashboards.id [delete: cascade]
