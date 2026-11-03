// Package server is the blog's HTTP API on Echo: routes, handlers, the
// X-User middleware, request validation and the error handler that
// shapes every refusal as a JSON object with a message.
package server

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/Piechutowski/volt/compare/echosqlx/store"
)

// New builds the Echo instance that serves the blog from st.
func New(st *store.Store) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Validator = newValidator()
	e.HTTPErrorHandler = errorHandler

	h := &handler{store: st}
	e.Use(middleware.Recover())
	e.Use(h.requireMember)

	e.POST("/posts", h.createPost)
	e.GET("/posts", h.feed)
	e.GET("/posts/:id", h.getPost)
	e.PATCH("/posts/:id", h.editPost)
	e.POST("/posts/:id/publish", h.publishPost)
	e.GET("/posts/:id/audit", h.audit)
	e.POST("/posts/:id/comments", h.createComment)
	e.GET("/posts/:id/comments", h.listComments)
	e.DELETE("/posts/:id/comments/:cid", h.deleteComment)
	e.GET("/me/stats", h.stats)
	return e
}

type handler struct {
	store *store.Store
}

const memberKey = "member"

func (h *handler) requireMember(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		id, err := strconv.Atoi(c.Request().Header.Get("X-User"))
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "send X-User: a member id")
		}
		m, err := h.store.Member(c.Request().Context(), id)
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusUnauthorized, "no such member")
		}
		if err != nil {
			return err
		}
		c.Set(memberKey, m)
		return next(c)
	}
}

func currentMember(c echo.Context) store.Member {
	return c.Get(memberKey).(store.Member)
}

func pathID(c echo.Context, name string) (int, error) {
	id, err := strconv.Atoi(c.Param(name))
	if err != nil {
		return 0, echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	return id, nil
}

type requestValidator struct {
	validate *validator.Validate
}

var (
	slugPattern = regexp.MustCompile(`^[a-z0-9-]{1,60}$`)
	tagPattern  = regexp.MustCompile(`^[a-z0-9-]{1,20}$`)
)

func newValidator() *requestValidator {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		return name
	})
	v.RegisterValidation("slug", func(fl validator.FieldLevel) bool {
		return slugPattern.MatchString(fl.Field().String())
	})
	v.RegisterValidation("tag", func(fl validator.FieldLevel) bool {
		return tagPattern.MatchString(fl.Field().String())
	})
	return &requestValidator{validate: v}
}

func (v *requestValidator) Validate(i any) error {
	return v.validate.Struct(i)
}

func fieldError(field, problem string) *echo.HTTPError {
	return echo.NewHTTPError(http.StatusUnprocessableEntity, map[string]any{
		"message": field + ": " + problem,
		"fields":  map[string]string{field: problem},
	})
}

func describe(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "min":
		if fe.Param() == "1" {
			return "must not be empty"
		}
		return "needs at least " + fe.Param() + " characters"
	case "max":
		if fe.Kind() == reflect.Slice {
			return "takes at most " + fe.Param()
		}
		return "takes at most " + fe.Param() + " characters"
	case "slug":
		return "must match [a-z0-9-]{1,60}"
	case "tag":
		return fmt.Sprintf("%q must match [a-z0-9-]{1,20}", fe.Value())
	}
	return "is invalid"
}

func errorHandler(err error, c echo.Context) {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		fields := make(map[string]string, len(ve))
		messages := make([]string, 0, len(ve))
		for _, fe := range ve {
			name, _, _ := strings.Cut(fe.Field(), "[")
			fields[name] = describe(fe)
			messages = append(messages, name+": "+describe(fe))
		}
		err = echo.NewHTTPError(http.StatusUnprocessableEntity, map[string]any{
			"message": strings.Join(messages, "; "),
			"fields":  fields,
		})
	}
	var he *echo.HTTPError
	if !errors.As(err, &he) {
		c.Logger().Error(err)
	}
	c.Echo().DefaultHTTPErrorHandler(err, c)
}
