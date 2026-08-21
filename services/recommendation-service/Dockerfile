FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/recommendation-service ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/recommendation-service /usr/local/bin/recommendation-service
EXPOSE 8089
ENTRYPOINT ["recommendation-service"]
