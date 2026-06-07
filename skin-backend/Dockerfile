# --- Build frontend ---
FROM node:24-alpine AS frontend-builder
WORKDIR /web
COPY element-skin/package*.json ./
RUN npm install
COPY element-skin/ .
ENV VITE_BASE_PATH=/VITE_BASE_PATH_PLACEHOLDER/
ENV VITE_API_BASE=VITE_API_BASE_PLACEHOLDER
RUN npm run build

# --- Build Go backend ---
FROM golang:1.26-alpine AS backend-builder
WORKDIR /src
COPY skin-backend-go/go.mod skin-backend-go/go.sum ./
RUN go mod download
COPY skin-backend-go/ .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/element-skin ./cmd/element-skin

# --- Runtime ---
FROM alpine:3.22
WORKDIR /app
RUN apk add --no-cache ca-certificates sed findutils
COPY --from=backend-builder /out/element-skin /app/element-skin
COPY --from=frontend-builder /web/dist /app/frontend_dist
COPY skin-backend-go/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

EXPOSE 8000
ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["/app/element-skin"]
