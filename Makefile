## confirm with y, deny with N
.PHONY: confirm
confirm:
	@echo 'Are you sure? [y/N] ' && read ans && [ $${ans:-N} = y ]

## db/migrations/up: apply database migrations
.PHONY: db/migrations/up
db/migrations/up:
	@echo 'Running up migration..'
	migrate -path=./migrations -database ${FORUM_DSN} up

## db/migrations/down: revert database migrations
.PHONY: db/migrations/down
db/migrations/down:
	@echo 'Running down migration..'
	migrate -path=./migrations -database ${FORUM_DSN} down

## db/migrations/new name=$1: create a new database migration
.PHONY: db/migrations/new
db/migrations/new: confirm
	@echo 'Creating migration files for ${name}..'
	migrate create -seq -ext=.sql -dir=./migrations ${name}

## otel/example: run OpenTelemetry dice example with exported traces and metrics
.PHONY: otel/example
otel/example: 
	@echo 'Running OpenTelemetry dice example.'
	OTEL_RESOURCE_ATTRIBUTES="service.name=dice,service.version=0.1.0" go run ./internal/scratch/otel
