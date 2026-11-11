# Web Dashboard

| Page | What it shows (all data from the backend) |
|---|---|
| Overview | Project count, analysis runs, backward breaking changes, blocked releases, latest test pass rate, per-project status |
| Projects | Create and list projects |
| Project > Versions | Upload OpenAPI files; explore paths, operations and schemas of a version |
| Project > Compatibility | Run an analysis between two versions; open the diff viewer (filter by classification and direction, explanations and remediation) |
| Project > Contracts | Register consumer contracts (JSON); verify against a version |
| Project > Testing | Generate and run tests on a mock; run details |
| Project > Policies | Create policies (admins); evaluate the release gate with contracts and a test run; gate history |
| Project > Analytics | Score trend, breaking changes per version, classification distribution, test pass-rate trend, contract trend, gate history, most-changed operations |
| Settings | Session information and sign-out |

Every page has loading, error and empty states; forms validate input and ask for confirmation before heavy actions. Mock/replay and workflow tools are available in the CLI only.
