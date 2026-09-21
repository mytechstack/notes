FROM golang:1.22-alpine

WORKDIR /app
COPY go.mod ./
COPY . .

RUN go build -o /app/bin/wrcs .

EXPOSE 8091
CMD ["/app/bin/wrcs"]
