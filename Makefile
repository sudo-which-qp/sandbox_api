test:
	@go test -v ./...

.PHONY: migration-create
migration-create:
	@migrate create -ext sql -dir cmd/migrate/migrations $(filter-out $@,$(MAKECMDGOALS))

.PHONY: migrate-up
migrate-up:
	@/app/bin/migrate -path /app/cmd/migrate/migrations -database "$(DATABASE_URL)" up

.PHONY: migrate-down
migrate-down:
	@/app/bin/migrate -path /app/cmd/migrate/migrations -database "$(DATABASE_URL)" down

.PHONY: migrate-force
migrate-force:
	@/app/bin/migrate -path /app/cmd/migrate/migrations -database "$(DATABASE_URL)" force

.PHONY: seed
seed:
	@go run cmd/migrate/seed/main.go