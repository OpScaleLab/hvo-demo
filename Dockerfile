FROM golang:1.24-alpine AS build
ARG VERSION=dev
WORKDIR /src
COPY go.mod main.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /hvo-demo .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /hvo-demo /hvo-demo
EXPOSE 8080
ENTRYPOINT ["/hvo-demo"]
