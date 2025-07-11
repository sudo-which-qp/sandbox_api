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
		route.Route("/users", func(route chi.Router) {
			route.With(app.AuthTokenMiddleware).Get("/profile", app.getUserHandler)
		})

		// Public routes
		route.Route("/auth", func(route chi.Router) {
			route.Post("/register", app.registerUserHandler)
			route.Post("/login", app.loginUserHandler)
		})
	})
}
