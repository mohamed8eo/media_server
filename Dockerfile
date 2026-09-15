FROM golang:1.27.1-alpine AS build
RUN apk add --no-cache alpine-sdk

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go install github.com/a-h/templ/cmd/templ@latest && \
    templ generate

RUN CGO_ENABLED=1 GOOS=linux go build -o main cmd/api/main.go

FROM alpine:3.20.1 AS prod
RUN apk add --no-cache ffmpeg python3 curl poppler-utils && \
    curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -o /usr/local/bin/yt-dlp && \
    chmod a+rx /usr/local/bin/yt-dlp
WORKDIR /app
COPY --from=build /app/main /app/main
EXPOSE ${PORT}
CMD ["./main"]


