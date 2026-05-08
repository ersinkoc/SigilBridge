# syntax=docker/dockerfile:1.7

FROM node:22-alpine AS ui
WORKDIR /src
COPY ui/package.json ui/pnpm-lock.yaml ./ui/
RUN corepack enable && corepack prepare pnpm@10.33.4 --activate && pnpm --dir ui install --frozen-lockfile
COPY ui ./ui
RUN pnpm --dir ui run build

FROM golang:1.26.3-alpine AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=ui /src/ui/dist ./ui/dist
RUN rm -rf ./internal/admin/ui/dist && mkdir -p ./internal/admin/ui/dist && cp -R ./ui/dist/. ./internal/admin/ui/dist/
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${BUILD_DATE}" \
    -o /out/sigilbridge ./cmd/sigilbridge

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /var/lib/sigilbridge
COPY --from=build /out/sigilbridge /usr/local/bin/sigilbridge
EXPOSE 8787 8788
VOLUME ["/var/lib/sigilbridge", "/etc/sigilbridge"]
ENTRYPOINT ["/usr/local/bin/sigilbridge"]
CMD ["serve", "--config", "/etc/sigilbridge/config.yaml"]
