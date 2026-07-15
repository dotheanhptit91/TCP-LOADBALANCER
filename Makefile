.PHONY: build test run down logs redis-state

build:
	go build ./...

test:
	go test ./...

run:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f

redis-state:
	docker compose exec redis sh -c 'for key in $$(redis-cli --scan --pattern "lb:*"); do type=$$(redis-cli TYPE "$$key"); echo "[$$key] ($$type)"; if [ "$$type" = hash ]; then redis-cli HGETALL "$$key"; else redis-cli SMEMBERS "$$key"; fi; done'
