FROM python:3.10-slim

WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y \
    && rm -rf /var/lib/apt/lists/*

# Install Python dependencies
RUN pip install --no-cache-dir \
    fastapi \
    uvicorn \
    pydantic \
    openai \
    python-dotenv \
    --index-url https://mirrors.aliyun.com/pypi/simple

# Copy application files
COPY main.py .
COPY soap.tmpl .

# Expose port
EXPOSE 8000

# Run the application
CMD ["python", "main.py"]
