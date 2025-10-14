FROM python:3.11-alpine

RUN apk add --no-cache postgresql-client

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY load_policies.py .
RUN mkdir -p policies

CMD ["python", "load_policies.py"]