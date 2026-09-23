FROM python:3.13-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1

RUN useradd --system --uid 10001 --create-home moderator
WORKDIR /app
COPY requirements.txt .
RUN pip install --require-hashes -r requirements.txt
COPY app.py .
USER 10001:10001
EXPOSE 8090
CMD ["uvicorn", "app:app", "--host", "0.0.0.0", "--port", "8090", "--workers", "1"]
