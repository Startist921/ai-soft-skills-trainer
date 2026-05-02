FROM golang:1.25-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /server ./cmd/server

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=build /server /server
EXPOSE 8080
ENTRYPOINT ["/server"]
