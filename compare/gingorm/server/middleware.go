package server

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Piechutowski/volt/compare/gingorm/models"
)

const memberKey = "member"

// RequireMember loads the member the X-User header names into the
// context, and refuses the request with 401 when there is none.
func RequireMember(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.GetHeader("X-User"), 10, 64)
		if err != nil {
			abortWithMessage(c, http.StatusUnauthorized, "send X-User: a member id")
			return
		}
		var member models.Member
		if err := db.WithContext(c.Request.Context()).First(&member, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				abortWithMessage(c, http.StatusUnauthorized, "no such member")
				return
			}
			abortWithError(c, err)
			return
		}
		c.Set(memberKey, member)
		c.Next()
	}
}

func currentMember(c *gin.Context) models.Member {
	return c.MustGet(memberKey).(models.Member)
}
