// Package middleware содержит HTTP-middleware Gin.
package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/chistotel/vpn_subscription_service/internal/model"
	"github.com/gin-gonic/gin"
)

// RequireAdmin проверяет Authorization: Bearer <adminToken>.
// При пустом adminToken все запросы отклоняются (fail-closed).
func RequireAdmin(adminToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if adminToken == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, model.ErrorResponse{Error: "unauthorized"})
			return
		}
		auth := c.GetHeader("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(auth, prefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, model.ErrorResponse{Error: "unauthorized"})
			return
		}
		token := strings.TrimPrefix(auth, prefix)
		if subtle.ConstantTimeCompare([]byte(token), []byte(adminToken)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, model.ErrorResponse{Error: "unauthorized"})
			return
		}
		c.Next()
	}
}
