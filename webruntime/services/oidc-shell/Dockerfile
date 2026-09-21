FROM golang:1.22-alpine

WORKDIR /app
COPY go.mod ./
COPY . .

RUN go build -o /app/bin/oidc-shell .

EXPOSE 8081
CMD ["/app/bin/oidc-shell"]
