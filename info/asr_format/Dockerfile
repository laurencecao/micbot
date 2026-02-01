FROM python:3.10-slim

WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y \
    ffmpeg \
    libsndfile1 \
    && rm -rf /var/lib/apt/lists/*

# Install Python dependencies
RUN pip install --no-cache-dir \
    torch \
    whisperx \
    fastapi \
    uvicorn \
    python-multipart \
    requests \
    --index-url https://mirrors.aliyun.com/pypi/simple

# Copy application files
COPY main.py .

# Expose port
EXPOSE 8800

# Run the application
CMD ["python", "main.py"]
