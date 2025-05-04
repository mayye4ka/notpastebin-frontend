FROM golang:1.23.8
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY ./ ./
RUN CGO_ENABLED=0 GOOS=linux go build ./cmd/...
CMD ["/notpastebin-frontend"]
