FROM golang:1.22.3-bullseye

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY hub/* ./

RUN CGO_ENABLED=0 GOOS=linux go build -o ./server

RUN adduser --system --uid 1001 go
RUN chown go ./server
USER go
WORKDIR /app



EXPOSE 3000 

CMD ["./server"]