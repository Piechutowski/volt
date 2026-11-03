package app

import (
	"net/http"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/examples/voltmeter/db"
)

// Dashboards are a member's own arrangements of charts (spec
// §scope-loads): every action receives the member CurrentUser loaded.
type Dashboards struct{}

// Index is the member's dashboards with their widgets.
func (Dashboards) Index(r volt.Request, user db.User) volt.Response {
	return volt.Then(db.DashboardBoards(user.ID), func(bs []db.DashboardWithWidgets) volt.Response {
		return volt.Render(bs)
	})
}

// Show is one of the member's dashboards; another member's is a 404, as
// if it did not exist.
func (Dashboards) Show(r volt.Request, user db.User, board int) volt.Response {
	return volt.Then(db.DashboardBoards(user.ID), func(bs []db.DashboardWithWidgets) volt.Response {
		for _, b := range bs {
			if b.ID == board {
				return volt.Render(b)
			}
		}
		return volt.Reject(http.StatusNotFound, "no such dashboard")
	})
}

// Create adds a dashboard owned by the member from a changeset body
// narrowed to its name (spec §request-bodies); a second board of one
// name is the unique index's 409.
func (Dashboards) Create(r volt.Request, user db.User, body db.DashboardChangeset) volt.Response {
	if !body.Valid() {
		return volt.Reject(http.StatusUnprocessableEntity, "check the fields").WithDetails(body.Errors())
	}
	return negotiated(r, func() volt.Response {
		return volt.Then(db.DashboardCreate(body.OwnerIDSet(user.ID)), func(d db.Dashboard) volt.Response {
			return volt.Created(d)
		})
	})
}

// Widgets are the charts on a dashboard the board scope loaded.
type Widgets struct{}

// Create adds a widget to the member's board.
func (Widgets) Create(r volt.Request, user db.User, owned db.Dashboard, board int, body db.WidgetChangeset) volt.Response {
	if owned.OwnerID != user.ID {
		return volt.Reject(http.StatusNotFound, "no such dashboard")
	}
	if !body.Valid() {
		return volt.Reject(http.StatusUnprocessableEntity, "check the fields").WithDetails(body.Errors())
	}
	return negotiated(r, func() volt.Response {
		return volt.Then(db.WidgetCreate(body.DashboardIDSet(owned.ID)), func(w db.Widget) volt.Response {
			return volt.Created(w)
		})
	})
}

// Delete removes a widget of the member's board; a widget of another
// board is a 404.
func (Widgets) Delete(r volt.Request, user db.User, owned db.Dashboard, board int, id int) volt.Response {
	if owned.OwnerID != user.ID {
		return volt.Reject(http.StatusNotFound, "no such dashboard")
	}
	return volt.Then(db.WidgetDeleteWhere(db.WidgetID.Eq(id), db.WidgetDashboardID.Eq(owned.ID)), func(n int64) volt.Response {
		if n == 0 {
			return volt.Reject(http.StatusNotFound, "no such widget")
		}
		return volt.NoContent()
	})
}
