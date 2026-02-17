FROM golang:1.22-alpine AS builder
RUN apk add --no-cache gcc musl-dev npm make
WORKDIR /app
COPY . .
RUN go install [github.com/a-h/templ/cmd/templ@latest](https://github.com/a-h/templ/cmd/templ@latest)
RUN go install [github.com/sqlc-dev/sqlc/cmd/sqlc@latest](https://github.com/sqlc-dev/sqlc/cmd/sqlc@latest)
RUN make build

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/bin/goth .
CMD ["./goth"]

