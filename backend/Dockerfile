FROM golang:alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG SERVICE_NAME
RUN go build -o /main ./cmd/${SERVICE_NAME}

FROM alpine:latest

RUN apk add --no-cache tzdata

WORKDIR /app
COPY --from=builder /main .

CMD ["./main"]
