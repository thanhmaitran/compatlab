# Developer Setup

```powershell
git clone <repo> compatlab; cd compatlab
./scripts/check-env.ps1
go mod tidy                       # creates go.sum
./scripts/dev.ps1 fmt             # gofmt (run once after pulling generated code)
./scripts/dev.ps1 check           # gofmt check + vet + tests
cd apps/web; npm install; npm test
```
Windows note: `go test -race` needs a C compiler (gcc); CI runs the race detector on Linux. Set `COMPATLAB_TEST_DATABASE_URL` to run the PostgreSQL store contract test.
