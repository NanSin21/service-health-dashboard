FROM golang:1.24-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY main.go schema.sql ./
RUN CGO_ENABLED=0 go build -o /health-api .

FROM scratch
COPY --from=build /health-api /health-api
EXPOSE 8080
ENTRYPOINT ["/health-api"]
