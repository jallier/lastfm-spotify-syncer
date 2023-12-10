# Build stage
FROM golang:1.21-alpine3.18 AS builder

WORKDIR /usr/src/app

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -v -o /usr/local/bin/app .

# tailwind
FROM node:21-alpine3.18 as tailwind-builder

WORKDIR /usr/src/app

COPY ./templates /usr/src/app/templates
COPY ./tailwind.config.js /usr/src/app
RUN npm install -g tailwindcss
RUN npx tailwindcss -o build.css --minify

# Final stage
FROM alpine:3.18

WORKDIR /app

COPY --from=builder /usr/local/bin/app .
COPY --from=builder /usr/src/app/templates /app/templates
COPY --from=builder /usr/src/app/static /app/static
COPY --from=tailwind-builder /usr/src/app/build.css /app/static/app.css

EXPOSE 8000
CMD ["/app/app"]
