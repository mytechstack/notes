FROM golang:1.22-alpine

WORKDIR /app
COPY go.mod ./
COPY . .

RUN go build -o /app/bin/control-plane .

EXPOSE 8093
CMD ["/app/bin/control-plane"]
