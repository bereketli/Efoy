# syntax=docker/dockerfile:1
# One image definition for all five Go binaries: docker build --build-arg SERVICE=core-api .

FROM golang:1.26-alpine AS build
ARG SERVICE
ARG VERSION=dev
ARG COMMIT=none
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    test -n "$SERVICE" && \
    CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X github.com/blinge12/efoy/internal/buildinfo.Version=${VERSION} -X github.com/blinge12/efoy/internal/buildinfo.Commit=${COMMIT}" \
      -o /out/app ./cmd/${SERVICE}

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app"]
