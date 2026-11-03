package server

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

var (
	slugPattern = regexp.MustCompile(`^[a-z0-9-]{1,60}$`)
	tagPattern  = regexp.MustCompile(`^[a-z0-9-]{1,20}$`)
	registered  sync.Once
)

func registerValidators() {
	registered.Do(func() {
		v, ok := binding.Validator.Engine().(*validator.Validate)
		if !ok {
			panic("server: gin's validator is not go-playground/validator")
		}
		v.RegisterTagNameFunc(func(f reflect.StructField) string {
			name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			if name == "-" {
				return ""
			}
			return name
		})
		v.RegisterValidation("slug", func(fl validator.FieldLevel) bool {
			return slugPattern.MatchString(fl.Field().String())
		})
		v.RegisterValidation("tagname", func(fl validator.FieldLevel) bool {
			return tagPattern.MatchString(fl.Field().String())
		})
	})
}

func bindJSON(c *gin.Context, dst any) bool {
	err := c.ShouldBindJSON(dst)
	if err == nil {
		return true
	}
	var invalid validator.ValidationErrors
	if errors.As(err, &invalid) {
		messages := make([]string, len(invalid))
		for i, fe := range invalid {
			messages[i] = fieldMessage(fe)
		}
		abortWithMessage(c, http.StatusUnprocessableEntity, strings.Join(messages, "; "))
		return false
	}
	abortWithMessage(c, http.StatusBadRequest, "malformed JSON body: "+err.Error())
	return false
}

func fieldMessage(fe validator.FieldError) string {
	unit := "characters"
	if fe.Kind() == reflect.Slice {
		unit = "items"
	}
	switch fe.Tag() {
	case "required":
		return fe.Field() + ": is required"
	case "min":
		return fmt.Sprintf("%s: must be at least %s %s", fe.Field(), fe.Param(), unit)
	case "max":
		return fmt.Sprintf("%s: must be at most %s %s", fe.Field(), fe.Param(), unit)
	case "slug":
		return fe.Field() + ": must match [a-z0-9-]{1,60}"
	case "tagname":
		return fe.Field() + ": must match [a-z0-9-]{1,20}"
	}
	return fmt.Sprintf("%s: fails %s", fe.Field(), fe.Tag())
}
