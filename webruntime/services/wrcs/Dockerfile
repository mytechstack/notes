FROM golang:1.22-alpine

WORKDIR /app
COPY go.mod ./
COPY . .

# go.mod names the one direct dependency (minio-go); `go mod tidy` resolves
# the full graph and writes go.sum during the image build.
RUN go mod tidy && go build -o /app/bin/wrcs .

EXPOSE 8091
CMD ["/app/bin/wrcs"]
