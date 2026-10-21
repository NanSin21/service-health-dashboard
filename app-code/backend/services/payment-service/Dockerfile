FROM golang:1.24-alpine AS build
WORKDIR /src
COPY main.go ./
RUN CGO_ENABLED=0 go build -o /service main.go

FROM scratch
COPY --from=build /service /service
EXPOSE 8083
ENTRYPOINT ["/service"]
