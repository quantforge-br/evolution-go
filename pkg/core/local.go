// Modified for private self-hosting: remote licensing and telemetry are removed.
package core

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// LicenseRoutes preserves the bundled Manager's API contract without contacting
// a licensing service. "active" is a compatibility status, not a legal grant.
// These read-only endpoints never return credentials or hardware identifiers.
func LicenseRoutes(engine *gin.Engine) {
	localStatus := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":            "active",
			"mode":              "local",
			"telemetry_enabled": false,
			"message":           "Local self-hosted build; remote registration is disabled.",
		})
	}

	license := engine.Group("/license")
	license.GET("/status", localStatus)
	license.GET("/register", localStatus)
	license.GET("/activate", localStatus)
}
