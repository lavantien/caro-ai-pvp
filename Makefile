.PHONY: backend-coverage frontend-coverage coverage

backend-coverage:
	cd backend && dotnet test -p:CollectCoverage=true -p:CoverletOutputFormat=cobertura -p:CoverletOutput=coverage/
	@COV=$$(node scripts/backend-coverage.mjs); \
	bash scripts/coverage-badge.sh backend $$COV coverage/backend.json

frontend-coverage:
	cd frontend && npx vitest run --coverage
	@COV=$$(cd frontend && node -e "const d=JSON.parse(require('fs').readFileSync('coverage/coverage-summary.json','utf8'));process.stdout.write(String(d.total.lines.pct))"); \
	bash scripts/coverage-badge.sh frontend $$COV coverage/frontend.json

coverage: backend-coverage frontend-coverage
