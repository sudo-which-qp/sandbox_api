test:
	@go test -v ./...

.PHONY: migration-create-dev
migration-create:
	@migrate create -ext sql -dir cmd/migrate/migrations $(filter-out $@,$(MAKECMDGOALS))

.PHONY: migration-create-prod
migration-create:
	@migrate create -ext sql -dir cmd/migrate/migrations $(filter-out $@,$(MAKECMDGOALS))

.PHONY: migrate-up
migrate-up:
	@go run cmd/api/*.go up

.PHONY: migrate-down
migrate-down:
	@go run cmd/api/*.go down

.PHONY: migrate-force
migrate-force:
	@go run cmd/api/*.go $(version) force

.PHONY: seed
seed:
	@go run cmd/migrate/seed/main.go