FROM golang:1.22-alpine AS build
WORKDIR /src
ARG SERVICE=api
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/taskflow ./cmd/${SERVICE}

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/taskflow /taskflow
EXPOSE 8080
ENTRYPOINT ["/taskflow"]
