package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"godsendjoseph.dev/sandbox-api/internal/models"
)

func (app *application) AuthTokenMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		// read the auth header
		authHeader := request.Header.Get("Authorization")
		if authHeader == "" {
			app.unauthorizedErrorResponse(writer, request, fmt.Errorf("missing auth header"))
			return
		}

		// parse it -> get the base64 encoded username and password
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			app.unauthorizedErrorResponse(writer, request, fmt.Errorf("invalid auth header"))
			return
		}

		token := parts[1]

		jwtToken, err := app.authenticator.ValidateToken(token)
		if err != nil {
			app.unauthorizedErrorResponse(writer, request, err)
			return
		}

		claims := jwtToken.Claims.(jwt.MapClaims)

		userID, err := strconv.ParseInt(fmt.Sprintf("%.f", claims["sub"]), 10, 64)
		if err != nil {
			app.unauthorizedErrorResponse(writer, request, err)
			return
		}

		ctx := request.Context()

		user, err := app.getUser(ctx, userID)
		if err != nil {
			app.unauthorizedErrorResponse(writer, request, err)
			return
		}

		ctx = context.WithValue(ctx, userCtx, user)

		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func (app *application) BasicAuthMiddleware() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			// read the auth header
			authHeader := request.Header.Get("Authorization")
			if authHeader == "" {
				app.unauthorizedBasicErrorResponse(writer, request, fmt.Errorf("missing auth header"))
				return
			}
			// parse it -> get the base64 encoded username and password
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Basic" {
				app.unauthorizedBasicErrorResponse(writer, request, fmt.Errorf("invalid auth header"))
				return
			}
			// decode it
			decoded, err := base64.StdEncoding.DecodeString(parts[1])
			if err != nil {
				app.unauthorizedBasicErrorResponse(writer, request, err)
				return
			}

			// check the credentials

			username := app.config.auth.basic.username
			password := app.config.auth.basic.password

			credentials := strings.SplitN(string(decoded), ":", 2)
			if len(credentials) != 2 || credentials[0] != username || credentials[1] != password {
				app.unauthorizedBasicErrorResponse(writer, request, fmt.Errorf("invalid credentials"))
				return
			}

			next.ServeHTTP(writer, request)
		})
	}
}

func (app *application) checkRolePrecedence(ctx context.Context, user *models.User, roleName string) (bool, error) {
	role, err := app.store.Roles.GetByName(ctx, roleName)

	if err != nil {
		return false, err
	}

	return user.Role.Level >= role.Level, nil
}

func (app *application) getUser(ctx context.Context, userID int64) (*models.User, error) {
	if !app.config.redisCfg.enabled {
		return app.store.Users.GetByID(ctx, userID)
	}

	app.logger.Infow("cache hit", "key", "user", "userID", userID)
	user, err := app.cacheStorage.Users.Get(ctx, userID)

	if err != nil {
		app.logger.Infow("error coming from cache", "key", "user", "userID", userID)
		return nil, err
	}

	if user == nil {
		app.logger.Infow("fetching from db", "userID", userID)
		user, err := app.store.Users.GetByID(ctx, userID)
		if err != nil {
			return nil, err
		}

		if err := app.cacheStorage.Users.Set(ctx, user); err != nil {
			return nil, err
		}

		return user, nil
	}

	return user, nil
}

func (app *application) RateLimiterMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if app.config.rateLimiter.Enabled {
			if allow, retryAfter := app.rateLimiter.Allow(request.RemoteAddr); !allow {
				app.rateLimitExceededResponse(writer, request, retryAfter.String())
				return
			}
		}
		next.ServeHTTP(writer, request)
	})
}
