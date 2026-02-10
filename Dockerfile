# Stage 1: Build the Go application
FROM golang:1.25 AS builder

# Set the current working directory inside the container
WORKDIR /app

# Copy the go.mod and go.sum files to download dependencies
COPY go.mod go.sum ./

# Download all the dependencies
RUN go mod download

# Copy the source code 
COPY . .

# Build the Go application
ARG TARGETOS
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -o /app/main ./cmd/api/main.go

# Stage 2: Prepare CA certificates and timezone data
FROM debian:bullseye-slim AS certs-and-tzdata

# Install ca-certificates and tzdata
RUN apt-get update && apt-get install -y ca-certificates tzdata

# Stage 3: Run the Go application using scratch
FROM scratch

# Copy the compiled Go binary from the build stage
COPY --from=builder /app/main /main
COPY --from=builder /app/docs/swagger.yaml /docs/swagger.yaml
COPY --from=builder /app/docs/swagger-ui /docs/swagger-ui

# Copy CA certificates from the certs stage
COPY --from=certs-and-tzdata /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Copy timezone data from the certs-and-tzdata stage
COPY --from=certs-and-tzdata /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=certs-and-tzdata /etc/localtime /etc/localtime
COPY --from=certs-and-tzdata /etc/timezone /etc/timezone

# Set the working directory
WORKDIR /


# Set the default timezone to UTC
ENV TZ=UTC

# Command to run the Go application
CMD ["/main"]

