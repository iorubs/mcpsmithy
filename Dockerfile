FROM golang:1.26-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /mcpsmithy ./cmd/mcpsmithy

FROM alpine:3.23
RUN apk add --no-cache git
COPY --from=builder /mcpsmithy /usr/local/bin/mcpsmithy
ENTRYPOINT ["mcpsmithy"]
CMD ["serve"]
