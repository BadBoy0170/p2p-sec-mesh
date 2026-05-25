# ── Python AI Sidecar Dockerfile ──────────────────────────────────────────────
FROM python:3.11-slim

WORKDIR /app

# Install dependencies first (layer cache)
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy application source
COPY app.py        ./
COPY llm_client.py ./

# Generate gRPC stubs inside the container
COPY stubs/ ./stubs/

# Create non-root user
RUN useradd -m -u 1000 appuser
USER appuser

ENV SIDECAR_PORT=50052
ENV OLLAMA_URL=http://ollama:11434
ENV OLLAMA_MODEL=llama3

EXPOSE 50052

CMD ["python", "app.py"]
