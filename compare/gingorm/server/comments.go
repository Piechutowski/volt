package server

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Piechutowski/volt/compare/gingorm/api"
	"github.com/Piechutowski/volt/compare/gingorm/models"
)

const (
	commentsPerWindow = 3
	commentWindow     = time.Hour
)

var errTooManyComments = errors.New("too many comments")

func (h *handlers) createComment(c *gin.Context) {
	post, ok := h.loadPost(c)
	if !ok {
		return
	}
	if post.State != models.StatePublished {
		abortWithMessage(c, http.StatusConflict, "only a published post takes comments")
		return
	}
	var req api.CreateCommentRequest
	if !bindJSON(c, &req) {
		return
	}
	comment := models.Comment{PostID: post.ID, MemberID: currentMember(c).ID, Body: req.Body}
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var recent int64
		since := time.Now().UTC().Add(-commentWindow)
		err := tx.Model(&models.Comment{}).
			Where("post_id = ? AND member_id = ? AND created_at > ?", comment.PostID, comment.MemberID, since).
			Count(&recent).Error
		if err != nil {
			return err
		}
		if recent >= commentsPerWindow {
			return errTooManyComments
		}
		return tx.Create(&comment).Error
	})
	if errors.Is(err, errTooManyComments) {
		abortWithMessage(c, http.StatusTooManyRequests, "at most 3 comments on a post in an hour")
		return
	}
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusCreated, commentResponse(comment))
}

func (h *handlers) listComments(c *gin.Context) {
	post, ok := h.loadPost(c)
	if !ok {
		return
	}
	if post.State != models.StatePublished && post.AuthorID != currentMember(c).ID {
		abortWithMessage(c, http.StatusNotFound, "post not found")
		return
	}
	var comments []models.Comment
	if err := h.db.WithContext(c.Request.Context()).Where("post_id = ?", post.ID).Order("id").Find(&comments).Error; err != nil {
		abortWithError(c, err)
		return
	}
	out := make([]api.Comment, len(comments))
	for i, cm := range comments {
		out[i] = commentResponse(cm)
	}
	c.JSON(http.StatusOK, out)
}

func (h *handlers) deleteComment(c *gin.Context) {
	postID, err1 := strconv.ParseUint(c.Param("id"), 10, 64)
	commentID, err2 := strconv.ParseUint(c.Param("cid"), 10, 64)
	if err1 != nil || err2 != nil {
		abortWithMessage(c, http.StatusNotFound, "comment not found")
		return
	}
	db := h.db.WithContext(c.Request.Context())
	var comment models.Comment
	err := db.Preload("Post").Where("post_id = ?", postID).First(&comment, commentID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		abortWithMessage(c, http.StatusNotFound, "comment not found")
		return
	}
	if err != nil {
		abortWithError(c, err)
		return
	}
	member := currentMember(c)
	if member.ID != comment.MemberID && member.ID != comment.Post.AuthorID {
		abortWithMessage(c, http.StatusForbidden, "only the comment's or the post's author deletes it")
		return
	}
	if err := db.Delete(&comment).Error; err != nil {
		abortWithError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func commentResponse(cm models.Comment) api.Comment {
	return api.Comment{
		ID:        cm.ID,
		PostID:    cm.PostID,
		MemberID:  cm.MemberID,
		Body:      cm.Body,
		CreatedAt: cm.CreatedAt,
	}
}
