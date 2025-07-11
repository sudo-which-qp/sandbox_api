package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"godsendjoseph.dev/sandbox-api/internal/mailer"
	"godsendjoseph.dev/sandbox-api/internal/models"
	"godsendjoseph.dev/sandbox-api/internal/store"
)

type RegisterUserPayload struct {
	Username string `json:"username" validate:"required,max=100"`
	Email    string `json:"email" validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,min=8,max=100"`
}

type CreateUserTokenPayload struct {
	Email    string `json:"email" validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,min=8,max=100"`
}

func (app *application) registerUserHandler(writer http.ResponseWriter, request *http.Request) {
	log.Printf("Frontend URL: %s", app.config.frontendURL)
	var payload RegisterUserPayload

	if err := readJSON(writer, request, &payload); err != nil {
		app.badRequestResponse(writer, request, err)
		return
	}

	if err := Validate.Struct(payload); err != nil {
		app.badRequestResponse(writer, request, err)
		return
	}

	user := &models.User{
		Username: payload.Username,
		Email:    payload.Email,
		Role: models.Role{
			Name: "user",
		},
	}

	// hash the user password
	if err := user.Password.Set(payload.Password); err != nil {
		app.internalServerError(writer, request, err)
		return
	}

	ctx := request.Context()

	plainToken := uuid.New().String() // using Google UUID package

	hash := sha256.Sum256([]byte(plainToken))
	hashToken := hex.EncodeToString(hash[:])

	// store the user
	err := app.store.Users.CreateAndInvite(ctx, user, hashToken, app.config.mail.exp)
	if err != nil {
		switch err {
		case store.ErrDuplicateEmail:
			app.badRequestResponse(writer, request, err)
		case store.ErrDuplicateUsername:
			app.badRequestResponse(writer, request, err)
		default:
			app.internalServerError(writer, request, err)
		}
		return
	}

	var data = map[string]any{
		"user":        user,
		"plain_token": plainToken,
	}

	isProdEnv := app.config.env == "production"
	activationURL := fmt.Sprintf("%s/confirm/%s", app.config.frontendURL, plainToken)

	vars := struct {
		Username      string
		ActivationURL string
	}{
		Username:      user.Username,
		ActivationURL: activationURL,
	}

	// send the user an email
	// there is an option for using Go Routine to send email
	err = app.mailer.Send(
		mailer.UserWelcomeTemplate,
		user.Username, user.Email,
		"Finish up your Registration",
		vars,
		!isProdEnv,
	)

	if err != nil {
		app.logger.Errorw("error sending welcome email", "error", err)

		// rollback user creation if email fails (SAGA Pattern)
		if err := app.store.Users.Delete(ctx, user.ID); err != nil {
			app.logger.Errorw("error deleting user", "error", err)
		}
		app.internalServerError(writer, request, err)
		return
	}

	if err := writeJSON(writer, http.StatusOK, "User created", data); err != nil {
		app.internalServerError(writer, request, err)
		return
	}

}

func (app *application) loginUserHandler(writer http.ResponseWriter, request *http.Request) {
	// parse the json payload
	var payload CreateUserTokenPayload

	if err := readJSON(writer, request, &payload); err != nil {
		app.badRequestResponse(writer, request, err)
		return
	}

	if err := Validate.Struct(payload); err != nil {
		app.badRequestResponse(writer, request, err)
		return
	}

	// fetch the user (check if the user exists) from the payload
	user, err := app.store.Users.GetByEmail(request.Context(), payload.Email)
	if err != nil {
		switch err {
		case store.ErrNotFound:
			app.unauthorizedErrorResponse(writer, request, err)
		default:
			app.internalServerError(writer, request, err)
		}
		return
	}

	// compare the password
	err = user.Password.Compare(payload.Password)
	if err != nil {
		app.unauthorizedPwdErrorResponse(writer, request, err)
		return
	}

	// generate the token -> add claims -> sign the token
	claims := jwt.MapClaims{
		"sub": user.ID,
		"exp": time.Now().Add(app.config.auth.token.exp).Unix(),
		"iat": time.Now().Unix(),
		"nbf": time.Now().Unix(),
		"iss": app.config.auth.token.issuer,
		"aud": app.config.auth.token.audience,
	}
	token, err := app.authenticator.GenerateToken(claims)
	if err != nil {
		app.internalServerError(writer, request, err)
		return
	}

	// send back the token
	if err := writeJSON(writer, http.StatusOK, "token created", token); err != nil {
		app.internalServerError(writer, request, err)
		return
	}
}

// ==================== Private Methods ===================== //
