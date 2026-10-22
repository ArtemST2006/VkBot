FROM golang:1.26.7-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -o /out/vk-bot ./cmd/vk-bot
RUN CGO_ENABLED=0 GOBIN=/out go install github.com/pressly/goose/v3/cmd/goose@v3.24.1

FROM alpine:3.23
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/vk-bot /usr/local/bin/vk-bot
COPY --from=build /out/goose /usr/local/bin/goose
COPY migrations ./migrations
CMD ["sh", "-c", "goose -dir /app/migrations postgres \"$DATABASE_URL\" up && exec vk-bot"]
