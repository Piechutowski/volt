package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Piechutowski/volt/compare/gingorm/api"
	"github.com/Piechutowski/volt/compare/gingorm/models"
)

func (h *handlers) stats(c *gin.Context) {
	member := currentMember(c)
	db := h.db.WithContext(c.Request.Context())
	var drafts, published, received int64
	if err := db.Model(&models.Post{}).Where("author_id = ? AND state = ?", member.ID, models.StateDraft).Count(&drafts).Error; err != nil {
		abortWithError(c, err)
		return
	}
	if err := db.Model(&models.Post{}).Where("author_id = ? AND state = ?", member.ID, models.StatePublished).Count(&published).Error; err != nil {
		abortWithError(c, err)
		return
	}
	err := db.Model(&models.Comment{}).
		Joins("JOIN posts ON posts.id = comments.post_id").
		Where("posts.author_id = ?", member.ID).
		Count(&received).Error
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, api.Stats{Drafts: int(drafts), Published: int(published), CommentsReceived: int(received)})
}
