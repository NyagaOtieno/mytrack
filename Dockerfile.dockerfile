FROM golang:1.23-alpine

WORKDIR /app
COPY . .

RUN go mod tidy
RUN go build -o fmb920 cmd/server.go

EXPOSE 8080 5027

CMD ["./fmb920"]
