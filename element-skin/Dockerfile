FROM node:20-alpine as builder

WORKDIR /app
COPY package*.json ./
RUN npm install

COPY . .

ARG VITE_BASE_PATH=/
ARG VITE_API_BASE=
ARG BUILD_MODE=standard
ENV VITE_BASE_PATH=${VITE_BASE_PATH}
ENV VITE_API_BASE=${VITE_API_BASE}

RUN if [ "$BUILD_MODE" = "low-memory" ] || [ "$BUILD_MODE" = "fast" ]; then \
      npm run build-fast; \
    else \
      npm run build; \
    fi

FROM nginx:1.25-alpine

COPY --from=builder /app/dist /usr/share/nginx/html

RUN echo 'server { \
    listen 80; \
    root /usr/share/nginx/html; \
    location / { try_files $uri $uri/ /index.html; } \
}' > /etc/nginx/conf.d/default.conf

EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
