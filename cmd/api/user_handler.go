package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"godsendjoseph.dev/sandbox-api/internal/models"
	"godsendjoseph.dev/sandbox-api/internal/store"
)

const userAuthCtx contextKey = "user"
const userParamCtx contextKey = "userID"

type UpdateUserPayload struct {
	FirstName string `json:"first_name" validate:"required,max=100"`
	LastName  string `json:"last_name" validate:"required,max=100"`
}

func (app *application) getUserHandler(writer http.ResponseWriter, request *http.Request) {
	user := getUserFromCtx(request)

	if err := writeJSON(writer, http.StatusOK, "User retrieved", user); err != nil {
		app.internalServerError(writer, request, err)
		return
	}
}

func (app *application) updateUserProfileHandler(writer http.ResponseWriter, request *http.Request) {
	var payload UpdateUserPayload

	if err := readJSON(writer, request, &payload); err != nil {
		app.badRequestResponse(writer, request, err)
		return
	}

	isPayloadValid := validatePayload(writer, payload)
	if !isPayloadValid {
		return
	}

	ctx := request.Context()

	user := getUserFromCtx(request)

	user.FirstName = payload.FirstName
	user.LastName = payload.LastName

	if err := app.store.Users.UpdateUserProfile(ctx, user); err != nil {
		app.internalServerError(writer, request, err)
		return
	}

	if err := writeJSON(writer, http.StatusOK, "User updated", user); err != nil {
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

func (app *application) usersContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		log.Println("usersContextMiddleware running on path:", request.URL.Path)
		idParam := chi.URLParam(request, "userID")

		id, err := strconv.ParseInt(idParam, 10, 64)

		if err != nil {
			app.badRequestResponse(writer, request, err)
			return
		}

		ctx := request.Context()

		userID := id

		user, err := app.store.Users.GetByID(ctx, userID)

		if err != nil {
			switch {
			case errors.Is(err, store.ErrAccountNotVerified):
				app.unauthorizedErrorResponse(writer, request, err)
			case errors.Is(err, store.ErrNotFound):
				app.notFoundResponse(writer, request, err)
			default:
				app.internalServerError(writer, request, err)
			}
			return
		}

		ctx = context.WithValue(ctx, userParamCtx, user)

		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func getUserFromCtx(request *http.Request) *models.User {
	user, _ := request.Context().Value(userAuthCtx).(*models.User)
	return user
}
