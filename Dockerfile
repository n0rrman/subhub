FROM golang:1.22.3-bullseye

# Install dependencies
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

# Compile server
COPY hub/* ./
RUN CGO_ENABLED=0 GOOS=linux go build -o ./server

# Create user with only necessary access
RUN adduser --system --uid 1001 go
RUN chown go ./server

# Change to new user
USER go
WORKDIR /app

# Exposing port 80 (probably already exposed)
EXPOSE 80

# Run server
CMD ["./server"]