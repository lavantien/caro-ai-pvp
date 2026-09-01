.PHONY: backend-coverage frontend-coverage coverage

backend-coverage:
	cd backend && dotnet test /p:CollectCoverage=true /p:CoverletOutputFormat=cobertura /p:CoverletOutput=coverage/
	@COV=$$(node -e "const fs=require('fs');const dirs=fs.readdirSync('backend/tests',{withFileTypes:true}).filter(d=>d.isDirectory()).map(d=>'backend/tests/'+d.name+'/coverage/coverage.cobertura.xml').filter(f=>fs.existsSync(f));let valid=0,covered=0;for(const f of dirs){const s=fs.readFileSync(f,'utf8');const a=n=>{const m=s.match(new RegExp(n+'=\"([0-9.]+)\"'));return m?+m[1]:null};const lv=a('lines-valid'),lr=a('line-rate');if(lv&&lr!==null){valid+=lv;covered+=lr*lv;}}process.stdout.write(String(valid?Math.round(100*covered/valid):0))"); \
	bash scripts/coverage-badge.sh backend $$COV coverage/backend.json

frontend-coverage:
	cd frontend && npx vitest run --coverage
	@COV=$$(cd frontend && node -e "const d=JSON.parse(require('fs').readFileSync('coverage/coverage-summary.json','utf8'));process.stdout.write(String(d.total.lines.pct))"); \
	bash scripts/coverage-badge.sh frontend $$COV coverage/frontend.json

coverage: backend-coverage frontend-coverage
