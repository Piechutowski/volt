package db

migration 0001__team_and_machines {
  create table users {
    id         int       [pk]
    email      string    [unique]
    name       string
    created_at time.Time [default: `CURRENT_TIMESTAMP`]
  }
  exec `INSERT INTO users (id, email, name) VALUES (1, 'ada@voltmeter.test', 'Ada'), (2, 'grace@voltmeter.test', 'Grace'), (3, 'ken@voltmeter.test', 'Ken')` [down: `DELETE FROM users WHERE id IN (1, 2, 3)`]
  create table machines {
    id     int     [pk]
    cpu    string
    cores  int
    mem_gb float64
    os     string
    arch   string
    go     string

    indexes {
      (cpu, cores, mem_gb, os, arch, go) [unique]
    }
  }
}

migration 0002__runs_and_phases {
  create table runs {
    id         int       [pk]
    commit     string
    machine_id int       [ref: machines.id]
    comment    string    [null]
    created_at time.Time [default: `CURRENT_TIMESTAMP`]
  }
  create table ms_load {
    run_id    int     [pk, ref: runs.id, delete: cascade]
    tables    int     [pk]
    wall_ms   float64
    alloc_mb  float64
    heap_mb   float64
    allocs    int
    source_kb float64
    profile   string  [null]

    indexes {
      (tables, run_id)
    }
  }
  create table ms_check {
    run_id    int     [pk, ref: runs.id, delete: cascade]
    tables    int     [pk]
    wall_ms   float64
    alloc_mb  float64
    heap_mb   float64
    allocs    int
    source_kb float64
    profile   string  [null]

    indexes {
      (tables, run_id)
    }
  }
  create table ms_vet {
    run_id    int     [pk, ref: runs.id, delete: cascade]
    tables    int     [pk]
    wall_ms   float64
    alloc_mb  float64
    heap_mb   float64
    allocs    int
    source_kb float64
    profile   string  [null]

    indexes {
      (tables, run_id)
    }
  }
  create table ms_gen {
    run_id    int     [pk, ref: runs.id, delete: cascade]
    tables    int     [pk]
    wall_ms   float64
    alloc_mb  float64
    heap_mb   float64
    allocs    int
    source_kb float64
    profile   string  [null]

    indexes {
      (tables, run_id)
    }
  }
}

migration 0003__machine_and_run_details {
  alter table machines {
    rename mem_gb to memory_gb
    add created_at time.Time [default: `CURRENT_TIMESTAMP`]
  }
  alter table runs {
    rename comment to note
    add dirty bool [default: false]
  }
  create index runs (commit)
  rename table ms_gen to ms_generate
}

migration 0004__output_size_not_profiles {
  alter table ms_load {
    add output_mb float64 [default: 0]
    drop profile string [null]
  }
  alter table ms_check {
    add output_mb float64 [default: 0]
    drop profile string [null]
  }
  alter table ms_vet {
    add output_mb float64 [default: 0]
    drop profile string [null]
  }
  alter table ms_generate {
    add output_mb float64 [default: 0]
    drop profile string [null]
  }
}

migration 0005__tags {
  create table tags {
    slug string [pk]
    name string
  }
  create index tags (`lower(name)`) [unique, name: "idx_tags_name_ci"]
  exec `INSERT INTO tags (slug, name) VALUES ('baseline', 'Baseline')` [down: `DELETE FROM tags WHERE slug = 'baseline'`]
  alter table runs {
    add tag string [null, ref: tags.slug, delete: set_null, update: cascade]
  }
}

migration 0006__run_ids_never_reused [irreversible] {
  rebuild table runs {
    id         int       [pk, autoincrement]
    commit     string    [index, from: `lower("commit")`]
    dirty      bool      [default: false]
    machine_id int       [ref: machines.id, delete: cascade]
    tag        string    [null, ref: tags.slug, delete: set_null, update: cascade]
    note       string    [null]
    created_at time.Time [default: `CURRENT_TIMESTAMP`]
  }
}

migration 0007__compact [no_transaction] {
  up {
    exec `VACUUM`
  }
  down {
  }
}

migration 0008__keystrokes_and_samples {
  create table keystrokes {
    run_id        int     [pk, ref: runs.id, delete: cascade]
    tables        int     [pk]
    ms_per_op     float64
    mb_per_op     float64
    allocs_per_op int
  }
  create table samples {
    id     int     [pk]
    run_id int     [ref: runs.id, delete: cascade]
    route  string
    status int
    ms     float64
    at     int
  }
  create index samples (route, at) [name: "idx_samples_failures", where: `status >= 400`]
}

migration 0009__thesis_counts {
  create table commits {
    sha     string    [pk]
    subject string
    author  string
    at      time.Time

    indexes {
      `lower(author)` [name: "idx_commits_author_ci"]
    }
  }
  create table line_counts {
    sha        string [pk, ref: commits.sha, delete: cascade]
    fixture    string [pk]
    volt_lines int
    hand_lines int
    gen_lines  int
  }
}

migration 0010__budgets {
  create table budgets {
    id     int     [pk]
    phase  string
    tables int
    max_ms float64
    state  string  [default: "ok"]

    indexes {
      (phase, tables) [unique]
    }
  }
  create table breaches {
    id         int       [pk]
    budget_id  int       [ref: budgets.id, delete: cascade]
    run_id     int       [ref: runs.id, delete: cascade]
    wall_ms    float64
    created_at time.Time [default: `CURRENT_TIMESTAMP`]
  }
}

migration 0011__dashboards_and_baselines {
  create table dashboards {
    id       int    [pk]
    owner_id int    [ref: users.id]
    name     string

    indexes {
      (owner_id, name) [unique]
    }
  }
  create table widgets {
    id           int    [pk]
    dashboard_id int    [ref: dashboards.id, delete: cascade]
    position     int
    phase        string
    tables       int
  }
  alter table runs {
    add baseline_id int [null, ref: runs.id, delete: set_null]
  }
}
