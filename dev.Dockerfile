# Development image: Go plus the same OCR tools as production, so tests run without a local Go install.
FROM golang:1.27-bookworm
RUN apt-get update \
    && apt-get install -y --no-install-recommends tesseract-ocr tesseract-ocr-deu poppler-utils \
    && rm -rf /var/lib/apt/lists/*
