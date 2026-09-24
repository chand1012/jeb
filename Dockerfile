# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.27.1 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
    -ldflags="-s -w -X github.com/chand1012/jeb/pkg/version.Version=$VERSION -X github.com/chand1012/jeb/pkg/version.CommitHash=$COMMIT -X github.com/chand1012/jeb/pkg/version.BuildDate=$BUILD_DATE" \
    -o /out/jeb .

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/jeb /jeb
USER nonroot:nonroot
ENTRYPOINT ["/jeb"]
CMD ["serve"]
