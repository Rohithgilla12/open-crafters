# Runner API service — spawns isolated grade containers via the host Docker socket.
FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY . .
RUN CGO_ENABLED=0 go build -o /runner ./cmd/runner

FROM docker:27-cli
COPY --from=build /runner /usr/local/bin/runner
EXPOSE 8080
ENTRYPOINT ["runner"]
