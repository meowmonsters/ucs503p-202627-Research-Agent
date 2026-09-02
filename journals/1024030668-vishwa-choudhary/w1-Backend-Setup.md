# Week 1 : Backend Server Setup

This week, I worked on setting up the initial backend for the project.

## What I did

- Set up a new Go project for the backend using `go mod init`.
- Wrote the basic backend server in `cmd/server/main.go`.
- Added a `/healthz` route so the server can be checked to confirm it is
  running correctly.
- Ran and tested the server locally using `go run ./cmd/server` to make
  sure it starts up and responds as expected.

At this stage, the server doesn't have any real functionality yet — no
agent logic, no AI model calls, and no data-fetching tools. It's just the
basic skeleton that the rest of the backend will be built on top of in the
coming weeks.
