# aws-tui

Fork of [bporter816/aws-tui](https://github.com/bporter816/aws-tui): a k9s-style
terminal UI for AWS, written in Go with tview/tcell and AWS SDK v2.

## Build & test

There is no Go toolchain on the host; everything runs in Docker.

```sh
# tests + vet
docker run --rm -v "$PWD":/src -w /src -e GOFLAGS=-buildvcs=false golang:latest \
  sh -c 'go vet ./... && go test ./...'

# binary for the host (macOS arm64); ./aws-tui is gitignored
docker run --rm -v "$(pwd)":/app -w /app -e GOOS=darwin -e GOARCH=arm64 golang:latest go build -o aws-tui .
```

After a change, run tests and rebuild `./aws-tui` so it can be tried right away.
Run `gofmt` only on files you touch — some upstream files are not gofmt-clean.

### Driving the real app

`hack/tui.sh` runs `./aws-tui` in a detached tmux session against
`hack/fake_aws.py` (a local endpoint answering every call with two EC2
instances) with fake credentials, so no real AWS account is touched:

```sh
hack/tui.sh start                 # extra env can follow, e.g. AWS_PROFILE=p1
hack/tui.sh keys Down Down Down Down Down Down Down Enter Down Down Enter  # EC2 > Instances
hack/tui.sh keys / d b            # prints the screen after the keys
hack/tui.sh stop
```

Unit tests can't catch focus/key routing bugs reliably; check those here.
`internal/app_test.go` also runs the app on a `tcell.SimulationScreen`.

## Layout

- `internal/app.go` — `Application`: layout (header / pages / footer), global
  key handling (`handleKey`), navigation stack, profile/region switching
  (`rebuild`), AWS config loading (`loadConfig`).
- `internal/table_view.go` — generic `TableView[T]` that almost every list
  view (~90, e.g. `ec2_instances.go`) is built on. Implements sorting
  (`Sortable`) and filtering (`Filterable`), see `internal/types.go`.
- `internal/ui/` — thin wrappers over tview widgets (`Table`, `Tree`, `Text`).
- `internal/registry.go` — `"Service.View"` → constructor, used by `rebuild`
  to reopen the current list view after switching profile/region.
- `internal/repo/` — AWS API calls; `internal/model/` — model types.
- `internal/config_picker.go` — popup list picker (region, profile, sort column).

Views not built on `TableView` (Tags, S3 objects, detail/policy pages) don't
get sort/filter.

## Features added in this fork

- `S` — sort by a column (picker). Same column again reverses; `(default
  order)` resets. Arrow in the header. Numeric / natural / case-insensitive
  compare, empty values always last (`utils.CompareValues`). Selection stays
  on the same item; sort survives refresh.
- `/` — filter rows (case-insensitive substring in any column) as you type,
  shown in the footer. Enter keeps it, Esc clears it; Esc in the table clears
  an active filter before going back.
- Pickers (`Ctrl+G` region, `Ctrl+P` profile, `S`) narrow the list as you type
  (prefix matches first), Backspace edits.
- Region defaults to `us-east-1`. Profile/region resolution follows the AWS
  CLI: picker choice > `AWS_REGION`/`AWS_DEFAULT_REGION` > profile region >
  `us-east-1`; `AWS_PROFILE`/`AWS_DEFAULT_PROFILE`, `AWS_CONFIG_FILE`,
  `AWS_SHARED_CREDENTIALS_FILE` are honored (the SDK does most of it;
  `readAWSProfiles` and `envProfile` cover the rest).
- `ui.Table.SetData` drops stale rows when the data shrinks.

## Decisions & gotchas

- Never write to `~/.aws/*` — those belong to the AWS CLI. The app has no
  config file of its own, so the last used region is not persisted. If that
  is wanted, add an app-owned file (e.g. `~/.config/aws-tui/`), keyed by
  account ID.
- tview `Pages`: once all pages are removed it focuses its own box behind the
  application's back and never blurs it; the root `Flex` then routes all keys
  to it (the filter input looked "hung" after a region switch). `clearPages`
  calls `app.SetFocus(pages)` to fix that — keep it when touching `rebuild`.
- The global key handler matches `KeyAction`s by `event.Name()`; while the
  filter input is open it passes every key through to it.
