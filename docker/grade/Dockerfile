# Sandbox image: crafters + language runtimes used by challenge starters.
FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY . .
RUN CGO_ENABLED=0 go build -o /crafters ./cmd/crafters

FROM golang:1.25-bookworm

RUN apt-get update && apt-get install -y --no-install-recommends \
      python3 \
      ca-certificates \
      curl \
      unzip \
    && curl -fsSL https://bun.sh/install | bash \
    && ln -sf /root/.bun/bin/bun /usr/local/bin/bun \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build /crafters /usr/local/bin/crafters

WORKDIR /work
ENTRYPOINT ["crafters"]
