FROM golang:1.26-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /out/dontpad-backend ./examples/DontPadBR3/apps/backend

FROM alpine:3.22

RUN adduser -D -u 10001 appuser \
  && mkdir -p /storage/data \
  && chown -R appuser:appuser /storage

WORKDIR /app

COPY --from=builder /out/dontpad-backend /app/dontpad-backend

USER appuser

EXPOSE 8080

ENV DONTPAD_ADDR=:8080
ENV DONTPAD_DATA_DIR=/storage/data

CMD ["/app/dontpad-backend"]
