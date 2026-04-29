.PHONY: backend-coverage frontend-coverage coverage

backend-coverage:
	cd backend && CGO_ENABLED=1 go test -race -coverprofile=coverage.out ./...
	@COV=$$(cd backend && go tool cover -func=coverage.out | tail -1 | awk '{print $$NF}' | tr -d '%'); \
	bash scripts/coverage-badge.sh backend $$COV coverage/backend.json

frontend-coverage:
	cd frontend && npx vitest run --coverage
	@COV=$$(cd frontend && node -e "const d=JSON.parse(require('fs').readFileSync('coverage/coverage-summary.json','utf8'));process.stdout.write(String(d.total.lines.pct))"); \
	bash scripts/coverage-badge.sh frontend $$COV coverage/frontend.json

coverage: backend-coverage frontend-coverage
