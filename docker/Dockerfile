# Build stage
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /bin/messenger ./cmd/messenger

# Runtime stage
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /bin/messenger /messenger
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/messenger"]
CMD ["-addr", ":8080"]
