package db

group phases {
  ms_load
  ms_check
  ms_vet
  ms_generate
}

group front = phases \ (ms_generate)

pred of_run  { run_id = :run }
pred at_size { tables in :sizes }
pred heavy   { alloc_mb > :mb }

select rows   for phases where of_run [order: (tables asc)]
select sized  for phases where of_run and at_size [order: (tables asc)]
select trend  for phases where tables = :tables [order: (run_id asc)]
select brief  (run_id, tables, wall_ms) for phases where of_run [order: (tables asc)]
select lean   (* \ (output_mb, source_kb)) for front where of_run and not heavy [order: (tables asc)]
select per_size (tables, count() as n, avg(alloc_mb) as mean_alloc, min(alloc_mb) as least_alloc, max(heap_mb) as peak_heap, sum(allocs) as total_allocs) for phases where at_size [group: (tables), order: (tables asc)]
select sizes (tables, count() as runs) for ms_load [group: (tables), order: (tables asc)]
select density (run_id, tables, allocs / tables as allocs_per_table, alloc_mb * 1024 / source_kb as alloc_per_source) for phases where of_run [order: (tables asc)]

select listed (id, commit, dirty, note, created_at, machine.cpu, machine.os, tag.name) for runs join machine join tag [order: (id desc)]
select untagged for runs where tag is null [order: (id asc)]
select of_commit for runs where commit = :commit [order: (id asc)]
select successors (id, commit, followers.id, followers.commit) for runs join followers where id = :id [order: (followers.id asc)]

select fleet (id, cpu, count(runs.id) as run_count) for machines join runs [group: (id, cpu), order: (id asc)]
select identified for machines where cpu = :cpu and cores = :cores and memory_gb = :memory and os = :os and arch = :arch and go = :toolchain

select latency (count() as n, avg(ms) as mean, min(ms) as best, max(ms) as worst) for samples where run_id = :run
select per_route (route, count() as n, avg(ms) as mean, max(ms) as worst) for samples where of_run [group: (route), order: (route asc)]
select failures for samples where of_run and status >= 400 [order: (id asc)]

select thesis (sha, fixture, volt_lines, hand_lines, gen_lines, commit.subject, commit.at) for line_counts join commit where fixture = :fixture [order: (commit.at asc)]
select matching for commits where subject like :pattern [order: (at desc)]

select by_state for budgets where state in :states [order: (id asc)]
select over for budgets where phase = :phase and tables = :tables and max_ms < :ms

select boards for dashboards with widgets where owner_id = :owner [order: (name asc)]
select board  for dashboards with widgets where id = :id

// The phases past the checker, small runs of them, and the forms of a
// predicate the selects above do not use.
group after_check = (ms_vet) + ms_generate

pred up_to     { tables <= :most }
pred run_up_to { of_run and up_to }

select smallest      for after_check where run_up_to [order: (tables asc)]
select slow          for samples where of_run and (status >= 400 or ms > :ms) [order: (id asc)]
select server_errors for samples where of_run and status in (500, 502, 503, 504) [order: (id asc)]
select others        for runs where machine_id = :machine and id != :id [order: (id desc)]
select annotated     for runs where note is not null [order: (id asc)]

// The forms no select above uses: a qualified column of a join in the
// predicate, two columns compared, a negated membership, a literal of
// each kind, a date/time bound, arithmetic with a parameter and both
// additive operators, min and max over text and time, and a with
// select that projects.
select on_system (id, commit, machine.os) for runs join machine where machine.os = :os [order: (id asc)]
select outgrown  for line_counts where gen_lines > hand_lines and not fixture in ("examples/voltmeter") [order: (sha asc)]
select uncommitted for runs where dirty = true [order: (id asc)]
select added     for commits where subject like "Add%" and at >= :since [order: (at asc)]
select quick     for samples where ms < 2.5 and status = 200 and route != "/runs/:id" [order: (id asc)]
select churn     (run_id, tables, (alloc_mb - heap_mb) * :scale as garbage, alloc_mb + output_mb as moved) for ms_generate where of_run [order: (tables asc)]
select span      (min(at) as first, max(at) as last, min(author) as first_author, max(subject) as last_subject) for commits
select names     (id, name) for dashboards with widgets where owner_id = :owner [order: (name asc)]
