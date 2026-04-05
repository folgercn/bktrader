FROM node:20-alpine AS frontend-builder
WORKDIR /app/web/console
COPY web/console/package*.json ./
RUN npm ci
COPY web/console/ ./
RUN npm run build

FROM golang:1.22-alpine AS backend-builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY configs ./configs
COPY db ./db
COPY research ./research
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /out/platform-api ./cmd/platform-api

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata
COPY --from=backend-builder /out/platform-api /usr/local/bin/platform-api
COPY --from=frontend-builder /app/web/console/dist ./web/console/dist
COPY configs/app.example.env ./configs/app.example.env
EXPOSE 8080
CMD ["platform-api"]
