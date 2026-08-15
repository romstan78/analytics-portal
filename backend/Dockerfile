FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server . \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/bootstrap-user ./cmd/bootstrap_user \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/seed-dev ./cmd/seed_dev \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S -G app app
WORKDIR /app
COPY --from=build /out/server /out/bootstrap-user /out/seed-dev /out/migrate /app/
RUN mkdir -p /app/logs && chown -R app:app /app
USER app
EXPOSE 8080
ENTRYPOINT ["/app/server"]
