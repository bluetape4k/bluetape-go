package echoadapter_test

import (
	"net/http"

	"github.com/bluetape4k/bluetape-go/web"
	"github.com/labstack/echo/v4"
)

var (
	_ echo.HandlerFunc = func(echo.Context) error { return nil }
	_ echo.HandlerFunc = echo.WrapHandler(http.NotFoundHandler())
	_ web.ProblemError = conformanceProblemError{}
)
