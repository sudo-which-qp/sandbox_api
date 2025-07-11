package main

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"godsendjoseph.dev/sandbox-api/internal/models"
	"godsendjoseph.dev/sandbox-api/internal/store"
)

const userCtx contextKey = "user"

func (app *application) getUserHandler(writer http.ResponseWriter, request *http.Request) {
	user := getUserFromCtx(request)

	if err := writeJSON(writer, http.StatusOK, "User retrieved", user); err != nil {
		app.internalServerError(writer, request, err)
		return
	}
}

func (app *application) getUserByIDHandler(writer http.ResponseWriter, request *http.Request) {
	idParam := chi.URLParam(request, "userID")

	id, err := strconv.ParseInt(idParam, 10, 64)

	if err != nil {
		app.badRequestResponse(writer, request, err)
		return
	}

	ctx := request.Context()

	user, err := app.getUser(ctx, id)

	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			app.notFoundResponse(writer, request, err)
		default:
			app.internalServerError(writer, request, err)
		}
		return
	}

	if err := writeJSON(writer, http.StatusOK, "User retrieved", user); err != nil {
		app.internalServerError(writer, request, err)
		return
	}
}

func (app *application) activateUserHandler(writer http.ResponseWriter, request *http.Request) {
	token := chi.URLParam(request, "token")

	ctx := request.Context()

	if err := app.store.Users.ActivateUser(ctx, token); err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			app.notFoundResponse(writer, request, err)
		default:
			app.internalServerError(writer, request, err)
		}
		return
	}

	if err := writeJSON(writer, http.StatusOK, "user activated", nil); err != nil {
		app.internalServerError(writer, request, err)
		return
	}
}

func (app *application) usersContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		idParam := chi.URLParam(request, "userID")

		id, err := strconv.ParseInt(idParam, 10, 64)

		if err != nil {
			app.badRequestResponse(writer, request, err)
			return
		}

		ctx := request.Context()

		userID := int64(id)

		user, err := app.store.Users.GetByID(ctx, userID)

		if err != nil {
			switch {
			case errors.Is(err, store.ErrNotFound):
				app.notFoundResponse(writer, request, err)
			default:
				app.internalServerError(writer, request, err)
			}
			return
		}

		ctx = context.WithValue(ctx, userCtx, user)

		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func getUserFromCtx(request *http.Request) *models.User {
	user, _ := request.Context().Value(userCtx).(*models.User)
	return user
}
