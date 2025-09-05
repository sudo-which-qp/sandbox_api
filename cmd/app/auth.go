package main

import (
	"crypto/rand"
	"errors"
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

type ResendOTPPayload struct {
	Email string `json:"email" validate:"required,email,max=255"`
}

type VerifyEmailPayload struct {
	Email   string `json:"email" validate:"required,email,max=255"`
	OtpCode string `json:"otp_code" validate:"required,max=6"`
}

type ResetPasswordPayload struct {
	Email       string `json:"email" validate:"required,email,max=255"`
	OtpCode     string `json:"otp_code" validate:"required,max=6"`
	NewPassword string `json:"new_password" validate:"required,min=8,max=100"`
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
		OtpExp:    otpCodeExpiring.Format(time.RFC3339),
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

	err = app.sendOTP(user, "Finish up your Registration", otpCode, otpCodeExpiring, mailer.UserWelcomeTemplate)

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
	user, err := app.store.Users.GetByEmail(request.Context(), payload.Email, true)
	if err != nil {
		switch err {
		case store.ErrNotFound:
			app.unauthorizedErrorResponse(writer, request, err)
		case store.ErrAccountNotVerified:
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

	var data = map[string]any{
		"user":  user,
		"token": token,
	}

	// send back the token
	if err := writeJSON(writer, http.StatusOK, "User authenticated", data); err != nil {
		app.internalServerError(writer, request, err)
		return
	}
}

func (app *application) verifyEmailHandler(writer http.ResponseWriter, request *http.Request) {
	var payload VerifyEmailPayload

	if err := readJSON(writer, request, &payload); err != nil {
		app.badRequestResponse(writer, request, err)
		return
	}

	isPayloadValid := validatePayload(writer, payload)
	if !isPayloadValid {
		return
	}

	ctx := request.Context()
	user, err := app.store.Users.GetByEmail(ctx, payload.Email, false)

	if err != nil {
		switch err {
		case store.ErrNotFound:
			app.unauthorizedErrorResponse(writer, request, err)
		default:
			app.internalServerError(writer, request, err)
		}
		return
	}

	if user.OtpCode != payload.OtpCode {
		app.unauthorizedErrorResponse(writer, request, errors.New("invalid otp code"))
		return
	}

	otpExp, err := time.Parse(time.RFC3339, user.OtpExp)
	if err != nil {
		app.internalServerError(writer, request, fmt.Errorf("invalid OTP expiration format: %w", err))
		return
	}

	if time.Now().After(otpExp) {
		app.unauthorizedErrorResponse(writer, request, errors.New("OTP code has expired"))
		return
	}

	err = app.store.Users.VerifyEmail(ctx, user.ID)
	if err != nil {
		app.internalServerError(writer, request, err)
		return
	}

	writeJSON(writer, http.StatusOK, "Email verified", user.OtpCode)
}

func (app *application) forgotPasswordHandler(writer http.ResponseWriter, request *http.Request) {
	var payload ResendOTPPayload

	if err := readJSON(writer, request, &payload); err != nil {
		app.badRequestResponse(writer, request, err)
		return
	}

	isPayloadValid := validatePayload(writer, payload)
	if !isPayloadValid {
		return
	}

	user, err := app.store.Users.GetByEmail(request.Context(), payload.Email, false)

	if err != nil {
		switch err {
		case store.ErrNotFound:
			app.unauthorizedErrorResponse(writer, request, err)
		default:
			app.internalServerError(writer, request, err)
		}
		return
	}

	otpCode, err := generateOTP()
	if err != nil {
		app.internalServerError(writer, request, err)
		return
	}
	otpCodeExpiring := time.Now().Add(5 * time.Minute)

	err = app.sendOTP(user, "OTP Code", otpCode, otpCodeExpiring, mailer.UserWelcomeTemplate)

	if err != nil {
		app.logger.Errorw("error sending welcome email", "error", err)
		app.internalServerError(writer, request, err)
		return
	}

	err = app.store.Users.UpdateOTPCode(request.Context(), user, otpCode, otpCodeExpiring.Format(time.RFC3339))

	if err != nil {
		app.internalServerError(writer, request, err)
		return
	}

	if err := writeJSON(writer, http.StatusOK, "Email sent for password reset", nil); err != nil {
		app.internalServerError(writer, request, err)
		return
	}
}

func (app *application) resetPasswordHandler(writer http.ResponseWriter, request *http.Request) {
	var payload ResetPasswordPayload

	if err := readJSON(writer, request, &payload); err != nil {
		app.badRequestResponse(writer, request, err)
		return
	}

	isPayloadValid := validatePayload(writer, payload)
	if !isPayloadValid {
		return
	}

	user, err := app.store.Users.GetByEmail(request.Context(), payload.Email, false)

	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			app.unauthorizedErrorResponse(writer, request, err)
		default:
			app.internalServerError(writer, request, err)
		}
		return
	}

	if user.OtpCode != payload.OtpCode {
		app.unauthorizedErrorResponse(writer, request, errors.New("invalid otp code"))
		return
	}

	otpExp, err := time.Parse(time.RFC3339, user.OtpExp)
	if err != nil {
		app.internalServerError(writer, request, fmt.Errorf("invalid OTP expiration format: %w", err))
		return
	}

	if time.Now().After(otpExp) {
		app.unauthorizedErrorResponse(writer, request, errors.New("OTP code has expired"))
		return
	}

	err = user.Password.Set(payload.NewPassword)
	if err != nil {
		app.internalServerError(writer, request, err)
		return
	}

	err = app.store.Users.ResetPassword(request.Context(), user)

	if err != nil {
		app.internalServerError(writer, request, err)
		return
	}

	if err := writeJSON(writer, http.StatusOK, "You have successfully reset your password", nil); err != nil {
		app.internalServerError(writer, request, err)
		return
	}
}

func (app *application) resendOTPHandler(writer http.ResponseWriter, request *http.Request) {
	var payload ResendOTPPayload

	if err := readJSON(writer, request, &payload); err != nil {
		app.badRequestResponse(writer, request, err)
		return
	}

	isPayloadValid := validatePayload(writer, payload)
	if !isPayloadValid {
		return
	}

	user, err := app.store.Users.GetByEmail(request.Context(), payload.Email, false)

	if err != nil {
		switch err {
		case store.ErrNotFound:
			app.unauthorizedErrorResponse(writer, request, err)
		default:
			app.internalServerError(writer, request, err)
		}
		return
	}

	otpCode, err := generateOTP()
	if err != nil {
		app.internalServerError(writer, request, err)
		return
	}
	otpCodeExpiring := time.Now().Add(5 * time.Minute)

	err = app.sendOTP(user, "OTP Code", otpCode, otpCodeExpiring, mailer.UserWelcomeTemplate)

	if err != nil {
		app.logger.Errorw("error sending welcome email", "error", err)
		app.internalServerError(writer, request, err)
		return
	}

	err = app.store.Users.UpdateOTPCode(request.Context(), user, otpCode, otpCodeExpiring.Format(time.RFC3339))

	if err != nil {
		app.internalServerError(writer, request, err)
		return
	}

	if err := writeJSON(writer, http.StatusOK, "OTP sent", nil); err != nil {
		app.internalServerError(writer, request, err)
		return
	}

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

func (app *application) sendOTP(user *models.User, subject string, otpCode string, otpCodeExpiring time.Time, emailTemplate string) error {
	isProdEnv := app.config.env == "production"

	vars := struct {
		Username string
		OtpCode  string
		OTPExp   string
		Subject  string
	}{
		Username: user.Username,
		OtpCode:  otpCode,
		OTPExp:   otpCodeExpiring.String(),
		Subject:  subject,
	}

	return app.mailer.SendWithOptions(
		emailTemplate,
		user.Username,
		user.Email,
		subject,
		vars,
		mailer.AsyncInMemory,
		!isProdEnv,
	)
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
