# Determine if we're in a production environment
ifeq ($(PROD),true)
    # Production: use the pre-built binary
    APP_CMD=./main
else
    # Development: build a temporary binary and run it
    APP_CMD=go build -o /tmp/dev-main ./cmd/app/ && /tmp/dev-main
endif

test:
	@go test -v ./...

.PHONY: migration-create
migration-create:
	@migrate create -ext sql -dir cmd/migrate/migrations $(filter-out $@,$(MAKECMDGOALS))

.PHONY: migrate-up
migrate-up:
	@$(APP_CMD) up

.PHONY: migrate-down
migrate-down:
	@$(APP_CMD) down

.PHONY: migrate-force
migrate-force:
	@$(APP_CMD) $(version) force

.PHONY: seed
seed:
	@$(APP_CMD) seed

.PHONY: clean
clean:
	@rm -f /tmp/dev-main