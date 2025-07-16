FROM golang:1.23-alpine AS build

WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN go build -o app .

FROM gcr.io/distroless/static
COPY --from=build /app/app /
ENTRYPOINT ["/app"]

# EXPOSE 8080

# CMD ["go", "test", "-v"]