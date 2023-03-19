package routes

import (
	"github.com/Waifu-im/waifu-api/api/controllers/image"
	"github.com/Waifu-im/waifu-api/api/middlewares"
	"github.com/Waifu-im/waifu-api/api/utils"
	"github.com/labstack/echo/v4"
)

func AddImageRouter(globals utils.Globals, app *echo.Echo) error {
	controller := image.Controller{Globals: globals}
	//using a group for all the commons middleware would have been better but there is some issue with groups.
	//404 instead of 405 when using wrong method, parent group middlewares not triggered on children etc...

	app.GET(
		"/search",
		controller.RouteSelector(false),
		middlewares.TokenVerification(
			globals,
			func(c echo.Context) (bool, error) {
				var full bool
				_ = echo.QueryParamsBinder(c).Bool("full", &full)
				// when new release will be out this middleware will be on the whole group with that condition in addition
				// c.Request().URL.Path == "/search"
				return !full, nil
			}),
		middlewares.PermissionsVerification(globals, []string{"admin"}, middlewares.BoolParamsSkipper("full", "", true)),
	)

	app.GET(
		"/fav",
		controller.RouteSelector(true),
		middlewares.TokenVerification(globals, nil),
		middlewares.PermissionsVerification(globals, []string{"view_favorites"}, middlewares.Int64ParamsSkipper("user_id", "target_user_id", true)),
	)
	return nil
}
