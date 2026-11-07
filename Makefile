.PHONY: up down test smoke integration-test

up:
	docker compose up -d --wait --wait-timeout 120

down:
	docker compose down

test:
	go test ./...

smoke:
	sh scripts/aws-smoke.sh

integration-test:
	go test -tags=integration -count=1 -timeout=120s -v ./tests/integration/...