package main

import (
	"github.com/go-chi/chi/v5"
	"net/http"
)

func (app *application) registerRoutes(router *chi.Mux) {
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/v1/health", http.StatusSeeOther)
	})

	router.Route("/v1", func(route chi.Router) {
		route.Get("/health", app.healthCheckHandler)
		route.Post("/bulk-emails", app.sendBulkEmails)

		// users
		route.Route("/user", func(route chi.Router) {
			route.Use(app.AuthTokenMiddleware)
			route.Get("/profile", app.getUserHandler)
			route.Post("/update-profile", app.updateUserProfileHandler)

			route.Route("/{userID}", func(route chi.Router) {
				route.Use(app.usersContextMiddleware)
				route.Get("/fetch-user", app.getUserByIDHandler)
			})
		})

		// Public routes
		route.Route("/auth", func(route chi.Router) {
			route.Post("/register", app.registerUserHandler)
			route.Post("/login", app.loginUserHandler)
			route.Post("/verify-email", app.verifyEmailHandler)
			route.Post("/forgot-password", app.forgotPasswordHandler)
			route.Post("/reset-password", app.resetPasswordHandler)
			route.Post("/resend-otp", app.resendOTPHandler)
		})
	})
}
