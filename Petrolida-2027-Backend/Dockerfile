FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN mkdir -p /data/portfolios && chown -R 65532:65532 /data
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /api .

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /

COPY --from=builder /api /api
COPY --from=builder /app/db/seeder/data/json /db/seeder/data/json
COPY --from=builder --chown=65532:65532 /data /data

ENV APP_HOST=0.0.0.0
ENV APP_PORT=8998
ENV ENVIRONMENT=production
ENV LOG_LEVEL=info
ENV PORTFOLIO_STORAGE_DIR=/data/portfolios

EXPOSE 8998

ENTRYPOINT ["/api"]
