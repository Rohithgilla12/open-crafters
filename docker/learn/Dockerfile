# Learner web app — serves challenge catalog and stage instructions.
FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY . .
RUN CGO_ENABLED=0 go build -o /learn ./cmd/learn

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /learn /learn
EXPOSE 8081
USER nonroot:nonroot
ENTRYPOINT ["/learn"]
