FROM golang:alpine AS builder
ENV GOTOOLCHAIN=auto

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /ws-chat ./cmd/server/

FROM alpine:3.21

WORKDIR /app

COPY --from=builder /ws-chat /app/ws-chat

EXPOSE 8081

CMD ["/app/ws-chat"]
