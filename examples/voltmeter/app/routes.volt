/*
  Voltmeter's routes: the API the collector and the desktop client
  speak, the printable reports, and the change stream.
*/
package app

import (
  db
)

pipeline api {
  use volt.RequestID
  use Served
}

pipeline member {
  use Vary
}

scope / [pipe: api, error_handler: Errors] {
  get  /                Home.Index     [name: root]
  any  /ping            Home.Ping
  get  /assets/:path... Assets.Serve   [name: asset]
  post /sign_in         Session.SignIn [body: SignIn]
  get  /events          volt.Events

  scope /api [name: api] {
    // The collector's writes: one sweep, the thesis counts, latency samples.
    post /uploads Uploads.Create [name: upload, body: Upload, timeout: 30]
    post /thesis  Thesis.Record  [name: thesis_record, body: ThesisUpload]
    get  /thesis  db.LineCountThesis
    get  /commits db.CommitMatching
    get  /probe   Probe.Count    [name: probe, timeout: 1]

    resources db.runs [only: (index, show, delete)]
    options /runs                     Runs.Options   [name: runs_allowed]
    get     /runs/listed              db.RunListed
    get     /runs/:id(int)/successors db.RunSuccessors
    patch   /runs/:id(int)            Runs.Annotate  [name: run_annotate, body: db.RunUpdate(id), only: (tag, note)]
    post    /runs/:id(int)/samples    Samples.Create [name: samples_record, body: SampleBatch]
    get     /runs/:run(int)/latency   db.SampleLatency
    get     /runs/:run(int)/routes    db.SamplePerRoute
    get     /runs/:run(int)/failures  db.SampleFailures

    // The run binds from the path, the baseline and sizes from the query.
    get /runs/:id(int)/compare Runs.Compare [name: run_compare] {
      run     db.RunGet(id)
      against db.RunGet(baseline)
      mine    db.MsLoadSized(id, sizes)
      theirs  db.MsLoadSized(baseline, sizes)
    }

    resources db.machines [default, except: (create, update, delete), singular: host]
    get /fleet db.MachineFleet

    resources db.tags [default]
    resources db.users [default, only: (index)]

    resources db.budgets [api, only: (index, show, create)]
    patch /budgets/:id(int) Budgets.Update [name: budget_ceiling, body: db.BudgetUpdate(id), only: (max_ms)]
    post  /budgets/reset    Budgets.Reset  [name: budgets_reset]

    scope /phases [name: phase] {
      dataset db.rows [strip: "ms_"]
    }

    scope /trend [name: trend] {
      dataset db.trend [strip: "ms_"]
    }

    scope /density [name: density] {
      dataset db.density [strip: "ms_", except: (ms_generate)]
    }

    // The run binds from the prefix, the sizes from repeated keys.
    scope /runs/:run(int)/sized [name: sized] {
      dataset db.sized [strip: "ms_", only: (ms_load, ms_generate)]
    }

    scope /per_size [name: per_size] {
      dataset db.per_size [strip: "ms_"]
    }

    get /sizes db.MsLoadSizes [name: sizes]

    // A member's own things, and the runs left for a member to tag, the
    // team's list only a member reads; the scope load answers 401 for
    // no one.
    scope /me [pipe: member, name: me] {
      load user CurrentUser

      get  /           Me.Show           [name: who]
      get  /untagged   db.RunUntagged     [name: todo]
      resources db.dashboards [only: (index, show), param: board]
      post /dashboards Dashboards.Create [name: dashboard_add, body: db.DashboardCreate, only: (name)]

      scope /dashboards/:board(int) [name: board] {
        load owned OwnedBoard

        post   /widgets          Widgets.Create [name: widget_add, body: db.WidgetCreate, only: (position, phase, tables)]
        delete /widgets/:id(int) Widgets.Delete [name: widget_remove]
      }
    }
  }

  scope /reports [name: report, error_handler: ReportErrors] {
    head /runs/:id(int) Reports.Probe
    get  /runs/:id(int) Reports.Show {
      run        db.RunGet(id)
      loading    db.MsLoadRows(id)
      checking   db.MsCheckRows(id)
      vetting    db.MsVetRows(id)
      generating db.MsGenerateRows(id)
    }
    get /runs/:id(int)/csv Reports.Export [name: export] {
      run        db.RunGet(id)
      loading    db.MsLoadRows(id)
      checking   db.MsCheckRows(id)
      vetting    db.MsVetRows(id)
      generating db.MsGenerateRows(id)
    }
  }
}
