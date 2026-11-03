// Package server is the blog's Gin router: the member middleware, the
// handlers that enforce the blog's rules and the validators the request
// bodies name.
package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type handlers struct {
	db *gorm.DB
}

// New returns the blog's router over db.
func New(db *gorm.DB) *gin.Engine {
	registerValidators()
	h := &handlers{db: db}

	r := gin.New()
	r.Use(gin.Recovery(), RequireMember(db))
	r.NoRoute(func(c *gin.Context) { abortWithMessage(c, http.StatusNotFound, "not found") })

	r.POST("/posts", h.createPost)
	r.GET("/posts", h.feed)
	r.GET("/posts/:id", h.showPost)
	r.PATCH("/posts/:id", h.updatePost)
	r.POST("/posts/:id/publish", h.publishPost)
	r.GET("/posts/:id/audit", h.listAudit)
	r.POST("/posts/:id/comments", h.createComment)
	r.GET("/posts/:id/comments", h.listComments)
	r.DELETE("/posts/:id/comments/:cid", h.deleteComment)
	r.GET("/me/stats", h.stats)
	return r
}

func abortWithMessage(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, gin.H{"message": message})
}

func abortWithError(c *gin.Context, err error) {
	_ = c.Error(err)
	abortWithMessage(c, http.StatusInternalServerError, "internal error")
}
