FROM golang:alpine AS builder 

WORKDIR /app 

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server ./cmd/api

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app
COPY --from=builder /app/server .

RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

EXPOSE 9090

CMD ["./server"]