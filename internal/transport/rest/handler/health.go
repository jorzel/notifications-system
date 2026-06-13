package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// HealthCheck reports service liveness. Dependency checks (queue
// connection) are added once those dependencies exist.
func HealthCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "healthy"})
}
