.PHONY: up down test smoke

up:
	docker compose up -d --wait --wait-timeout 120

down:
	docker compose down

test:
	go test ./...

smoke:
	sh scripts/aws-smoke.sh