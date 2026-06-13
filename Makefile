.PHONY: up down logs ps build test tidy vet api smoke tls

up:
	docker compose up -d --build

down:
	docker compose down

down-data:
	docker compose down -v

logs:
	docker compose logs -f

ps:
	docker compose ps

build:
	cd control-plane && go build ./...

vet:
	cd control-plane && go vet ./...

test:
	cd control-plane && go test ./...

tidy:
	cd control-plane && go mod tidy

# Run the API locally (needs a reachable PostgreSQL via DATABASE_URL).
api:
	cd control-plane && DATABASE_URL=$${DATABASE_URL:-postgres://freeswitch:freeswitch@localhost:5432/freeswitch_control?sslmode=disable} go run ./cmd/api

# Generate dev TLS material (CA + server + client certs) into deploy/tls/.
tls:
	bash hack/gen-tls.sh

# Quick end-to-end smoke against a running stack (HTTPS + mTLS).
smoke:
	curl -s --cacert deploy/tls/ca.crt https://localhost:8080/healthz; echo
	curl -s --cacert deploy/tls/ca.crt https://localhost:8080/readyz; echo
	curl -s --cacert deploy/tls/ca.crt --cert deploy/tls/client.crt --key deploy/tls/client.key \
		-u freeswitch:$${XML_PASSWORD:?set XML_PASSWORD} \
		-X POST https://localhost:8080/xml/directory --data 'user=2001&domain=192.168.48.143'; echo
