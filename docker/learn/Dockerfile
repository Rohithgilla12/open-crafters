# Learner web app — serves challenge catalog and stage instructions.
FROM node:22-bookworm AS css
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci
COPY internal/learn/styles ./internal/learn/styles
COPY internal/learn/templates.go internal/learn/client.js ./internal/learn/
RUN npm run build:learn-css

FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY . .
COPY --from=css /src/internal/learn/dist/style.css ./internal/learn/dist/style.css
RUN CGO_ENABLED=0 go build -o /learn ./cmd/learn

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /learn /learn
EXPOSE 8081
USER nonroot:nonroot
ENTRYPOINT ["/learn"]
