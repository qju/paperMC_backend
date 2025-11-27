FROM golang:1.25 AS builder
WORKDIR /app
COPY go.mod ./

RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main cmd/server/main.go


FROM eclipse-temurin:21-jre
WORKDIR /app
COPY --from=builder /app/main .
COPY web/ web/
RUN mkdir paperMC
VOLUME /app/paperMC
EXPOSE 8080 25565

CMD ["./main"]
