package server

import (
	"errors"
	"net/http"
	"slices"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Piechutowski/volt/compare/gingorm/api"
	"github.com/Piechutowski/volt/compare/gingorm/models"
)

const publishMinBody = 100

var errNotDraft = errors.New("post is not a draft")

func (h *handlers) createPost(c *gin.Context) {
	var req api.CreatePostRequest
	if !bindJSON(c, &req) {
		return
	}
	post := models.Post{
		Slug:     req.Slug,
		Title:    req.Title,
		Body:     req.Body,
		State:    models.StateDraft,
		AuthorID: currentMember(c).ID,
	}
	names := slices.Compact(slices.Sorted(slices.Values(req.Tags)))
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		for _, name := range names {
			tag := models.Tag{Name: name}
			if err := tx.Where(&tag).FirstOrCreate(&tag).Error; err != nil {
				return err
			}
			post.Tags = append(post.Tags, tag)
		}
		return tx.Create(&post).Error
	})
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		abortWithMessage(c, http.StatusConflict, "slug: already taken")
		return
	}
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusCreated, postResponse(post))
}

func (h *handlers) showPost(c *gin.Context) {
	post, ok := h.loadPost(c)
	if !ok {
		return
	}
	if post.State != models.StatePublished && post.AuthorID != currentMember(c).ID {
		abortWithMessage(c, http.StatusNotFound, "post not found")
		return
	}
	c.JSON(http.StatusOK, postResponse(post))
}

func (h *handlers) updatePost(c *gin.Context) {
	post, ok := h.loadPost(c)
	if !ok || !authorOfDraft(c, post, "edits") {
		return
	}
	var req api.UpdatePostRequest
	if !bindJSON(c, &req) {
		return
	}
	changes := map[string]any{}
	if req.Title != nil {
		post.Title = *req.Title
		changes["title"] = post.Title
	}
	if req.Body != nil {
		post.Body = *req.Body
		changes["body"] = post.Body
	}
	if len(changes) > 0 {
		if err := h.db.WithContext(c.Request.Context()).Model(&post).Updates(changes).Error; err != nil {
			abortWithError(c, err)
			return
		}
	}
	c.JSON(http.StatusOK, postResponse(post))
}

func (h *handlers) publishPost(c *gin.Context) {
	post, ok := h.loadPost(c)
	if !ok || !authorOfDraft(c, post, "publishes") {
		return
	}
	if utf8.RuneCountInString(post.Body) < publishMinBody {
		abortWithMessage(c, http.StatusUnprocessableEntity, "body: must be at least 100 characters to publish")
		return
	}
	now := time.Now().UTC()
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&post).Where("state = ?", models.StateDraft).Updates(map[string]any{
			"state":        models.StatePublished,
			"published_at": now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errNotDraft
		}
		audit := models.Audit{PostID: post.ID, MemberID: currentMember(c).ID, Action: "publish", At: now}
		return tx.Create(&audit).Error
	})
	if errors.Is(err, errNotDraft) {
		abortWithMessage(c, http.StatusConflict, "only a draft is published")
		return
	}
	if err != nil {
		abortWithError(c, err)
		return
	}
	post.State = models.StatePublished
	post.PublishedAt = &now
	c.JSON(http.StatusOK, postResponse(post))
}

func (h *handlers) listAudit(c *gin.Context) {
	post, ok := h.loadPost(c)
	if !ok {
		return
	}
	if post.AuthorID != currentMember(c).ID {
		abortWithMessage(c, http.StatusForbidden, "only the author reads the audit")
		return
	}
	var audits []models.Audit
	if err := h.db.WithContext(c.Request.Context()).Where("post_id = ?", post.ID).Order("id").Find(&audits).Error; err != nil {
		abortWithError(c, err)
		return
	}
	entries := make([]api.AuditEntry, len(audits))
	for i, a := range audits {
		entries[i] = api.AuditEntry{Action: a.Action, MemberID: a.MemberID, At: a.At}
	}
	c.JSON(http.StatusOK, entries)
}

func (h *handlers) loadPost(c *gin.Context) (models.Post, bool) {
	var post models.Post
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		abortWithMessage(c, http.StatusNotFound, "post not found")
		return post, false
	}
	err = h.db.WithContext(c.Request.Context()).Preload("Tags").First(&post, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		abortWithMessage(c, http.StatusNotFound, "post not found")
		return post, false
	}
	if err != nil {
		abortWithError(c, err)
		return post, false
	}
	return post, true
}

func authorOfDraft(c *gin.Context, post models.Post, action string) bool {
	if post.AuthorID != currentMember(c).ID {
		abortWithMessage(c, http.StatusForbidden, "only the author "+action+" a post")
		return false
	}
	if post.State != models.StateDraft {
		abortWithMessage(c, http.StatusConflict, "only a draft is changed")
		return false
	}
	return true
}

func postResponse(p models.Post) api.Post {
	tags := make([]string, len(p.Tags))
	for i, t := range p.Tags {
		tags[i] = t.Name
	}
	slices.Sort(tags)
	return api.Post{
		ID:          p.ID,
		Slug:        p.Slug,
		Title:       p.Title,
		Body:        p.Body,
		State:       p.State,
		AuthorID:    p.AuthorID,
		Tags:        tags,
		PublishedAt: p.PublishedAt,
		CreatedAt:   p.CreatedAt,
	}
}
