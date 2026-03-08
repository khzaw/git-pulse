# git-pulse

`git-pulse` is a Go TUI dashboard for repository activity. It focuses on dense, fast local analytics with optional GitHub pull request cycle metrics when the repo has a GitHub remote.

## Current capabilities

- Commit velocity trends across `7d`, `30d`, `90d`, `1y`, and `all`
- Author activity, new contributors, and bus-factor style concentration
- File and directory hotspots by touch frequency and churn
- Branch freshness and release cadence from local branches and tags
- Optional GitHub PR cycle, review, throughput, and aging summaries
- Snapshot export as JSON, Markdown, or CSV for CI/reporting

## Install

```bash
git clone <your-remote> git-pulse
cd git-pulse
make build
./bin/git-pulse
```

## Usage

Launch the dashboard in the current repository:

```bash
./bin/git-pulse --repo .
```

Generate non-interactive output:

```bash
./bin/git-pulse --repo . --json
./bin/git-pulse --repo . --markdown
./bin/git-pulse --repo . --csv
./bin/git-pulse --repo . --ci
```

If the repository points at GitHub, `git-pulse` will attempt to fetch pull request metrics. Set `GITHUB_TOKEN` for higher API limits.

## Keybindings

- `tab` / `shift+tab`: cycle panel focus
- `1`-`6`: jump directly to a panel
- `t`: cycle time window
- `r`: refresh metrics
- `q` / `ctrl+c`: quit

## Config

Example config:

```yaml
repo_path: .
theme: tokyo-night
refresh_seconds: 60
default_window: 30d
```

Use it with:

```bash
./bin/git-pulse --config .git-pulse.yml
```

## Development

```bash
make fmt
make test
make test-race
make build
```

## Architecture

- `internal/git`: local repository scanning via `go-git`
- `internal/aggregator`: windowed metric aggregation
- `internal/remote`: remote detection plus GitHub PR/review summarization
- `internal/dashboard`: shared loader for TUI and export modes
- `internal/tui`: Bubble Tea dashboard and rendering helpers
- `pkg/export`: JSON, Markdown, and CSV snapshot output

## License

MIT. See [LICENSE](./LICENSE).
