FROM python:3.11-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

RUN mkdir -p /data/textures && \
    python gen_key.py || true

EXPOSE 8000

CMD ["uvicorn", "routes_reference:app", "--host", "0.0.0.0", "--port", "8000"]
