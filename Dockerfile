# --- Angular app ---
FROM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npx ng build --configuration production

# --- Go binary ---
FROM golang:1.27-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY landing/ static/dist/
COPY --from=web /web/dist/web/browser/ static/dist/app/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/briefklar ./cmd/briefklar

# --- runtime: OCR tools, non-root ---
FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends tesseract-ocr tesseract-ocr-deu poppler-utils ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --system --uid 10001 --home /nonexistent briefklar
COPY --from=build /out/briefklar /usr/local/bin/briefklar
USER briefklar
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 CMD ["briefklar", "-healthcheck"]
ENTRYPOINT ["briefklar"]
