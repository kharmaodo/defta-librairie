FROM python:3.12.12-slim-bookworm

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    OMP_THREAD_LIMIT=1

RUN apt-get update && apt-get install -y --no-install-recommends \
    tesseract-ocr=5.3.0-2 tesseract-ocr-ara=1:4.1.0-2 \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --system --uid 10001 --create-home ocr
WORKDIR /app
COPY requirements.txt requirements-api.txt ./
RUN pip install -r requirements-api.txt
COPY ocr_core/ ./ocr_core/
COPY api.py ./
USER 10001:10001
EXPOSE 8091
HEALTHCHECK --interval=15s --timeout=5s --start-period=10s --retries=3 \
    CMD python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:8091/health/ready', timeout=3)"
CMD ["uvicorn", "api:app", "--host", "0.0.0.0", "--port", "8091", "--workers", "1", "--limit-concurrency", "16", "--timeout-keep-alive", "5", "--no-access-log"]
