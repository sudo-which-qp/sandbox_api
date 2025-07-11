package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"godsendjoseph.dev/sandbox-api/internal/mailer"
	"godsendjoseph.dev/sandbox-api/internal/models"
	"godsendjoseph.dev/sandbox-api/internal/store"
)

type RegisterUserPayload struct {
	FirstName string `json:"first_name" validate:"required,max=100"`
	LastName  string `json:"last_name" validate:"required,max=100"`
	Username  string `json:"username" validate:"required,max=100"`
	Email     string `json:"email" validate:"required,email,max=255"`
	Password  string `json:"password" validate:"required,min=8,max=100"`
}

type LoginUserPayload struct {
	Email    string `json:"email" validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,min=8,max=100"`
}

func (app *application) registerUserHandler(writer http.ResponseWriter, request *http.Request) {
	var payload RegisterUserPayload

	if err := readJSON(writer, request, &payload); err != nil {
		app.badRequestResponse(writer, request, err)
		return
	}

	isPayloadValid := validatePayload(writer, payload)
	if !isPayloadValid {
		return
	}

	otpCode, err := generateOTP()
	if err != nil {
		app.internalServerError(writer, request, err)
		return
	}

	otpCodeExpiring := time.Now().Add(5 * time.Minute)

	user := &models.User{
		FirstName: payload.FirstName,
		LastName:  payload.LastName,
		Username:  payload.Username,
		Email:     payload.Email,
		OtpCode:   otpCode,
		OtpExp:    otpCodeExpiring.String(),
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
	// store the user
	err = app.store.Users.CreateUserTx(ctx, user)
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

	isProdEnv := app.config.env == "production"

	vars := struct {
		Username string
		OtpCode  string
		OTPExp   string
	}{
		Username: user.Username,
		OtpCode:  otpCode,
		OTPExp:   otpCodeExpiring.String(),
	}

	// send the user an email
	// there is an option for using Go Routine to send email
	err = app.mailer.SendWithOptions(
		mailer.UserWelcomeTemplate,
		user.Username, user.Email,
		"Finish up your Registration",
		vars,
		mailer.AsyncInMemory,
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
	// generate the token -> add claims -> sign the token
	token, err := app.generateJWTToken(user)
	if err != nil {
		app.internalServerError(writer, request, err)
		return
	}

	var data = map[string]any{
		"user":  user,
		"token": token,
	}

	if err := writeJSON(writer, http.StatusOK, "User created", data); err != nil {
		app.internalServerError(writer, request, err)
		return
	}
}

func (app *application) loginUserHandler(writer http.ResponseWriter, request *http.Request) {
	// parse the json payload
	var payload LoginUserPayload

	if err := readJSON(writer, request, &payload); err != nil {
		app.badRequestResponse(writer, request, err)
		return
	}

	isPayloadValid := validatePayload(writer, payload)
	if !isPayloadValid {
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
	token, err := app.generateJWTToken(user)
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

func (app *application) verifyEmailHandler(writer http.ResponseWriter, request *http.Request) {

}

func (app *application) forgotPasswordHandler(writer http.ResponseWriter, request *http.Request) {

}

func (app *application) resetPasswordHandler(writer http.ResponseWriter, request *http.Request) {

}

func (app *application) veriftOtpHandler(writer http.ResponseWriter, request *http.Request) {

}

// ==================== Private Methods ===================== //
func generateOTP() (string, error) {
	const digits = 6
	max := big.NewInt(1000000) // 10^6 = 1,000,000

	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}

	// Format with leading zeros (e.g., 000123)
	otp := fmt.Sprintf("%06d", n.Int64())
	return otp, nil
}

func (app *application) generateJWTToken(user *models.User) (string, error) {
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
		app.logger.Errorw("error generating JWT token", "error", err)
		return "", err
	}
	return token, nil
}
