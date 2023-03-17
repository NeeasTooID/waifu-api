package middlewares

import (
	"fmt"
	"github.com/Waifu-im/waifu-api/api/utils"
	"github.com/labstack/echo/v4"
	"net/http"
	"strings"
)

func defaultSkipper(_ *echo.Context) (bool, error) {
	return false, nil
}

// TokenVerification check that the user secret (that is what is used to refresh a token) is the same as in db.
func TokenVerification(globals utils.Globals) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims := GetUserClaims(c)
			if claims == nil {
				return next(c)
			}
			isValid, err := globals.Database.IsValidCredentials(claims.UserId, claims.UserSecret)
			if err != nil {
				return err
			}
			if isValid {
				return next(c)
			}
			return &echo.HTTPError{
				Code:     http.StatusUnauthorized,
				Message:  fmt.Sprintf("Your token has expired, please check your current token at %v", globals.Config.WebSiteUrl+"/dashboard/"),
				Internal: err,
			}
		}
	}
}

// PermissionsVerification it will check if the user as the provided permission, there is a skipper to skip if not needed
func PermissionsVerification(globals utils.Globals, permissions []string, skipper func(c *echo.Context) (bool, error)) echo.MiddlewareFunc {
	if skipper == nil {
		skipper = defaultSkipper
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			skip, err := skipper(&c)
			if err != nil {
				return err
			}
			if skip {
				return next(c)
			}
			var targetUserId int64 = 0
			targetUserIdInterface := c.Get("target_user_id")
			if targetUserIdInterface == nil {
				targetUserId = 0
			} else {
				targetUserId = targetUserIdInterface.(int64)
			}
			claims := GetUserClaims(c)
			missing, err := globals.Database.GetMissingPermissions(claims.UserId, targetUserId, permissions)

			if err != nil {
				return err
			}
			if len(missing) > 0 {
				return &echo.HTTPError{
					Code:     http.StatusForbidden,
					Message:  fmt.Sprintf("You are missing the following permissions : %v", strings.Join(missing, ", ")),
					Internal: err,
				}
			}
			return next(c)
		}
	}
}
