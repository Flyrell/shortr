FROM --platform=$BUILDPLATFORM node:24-alpine AS client-deps
WORKDIR /app/apps/client
COPY apps/client/package.json apps/client/package-lock.json ./
RUN npm ci

FROM client-deps AS client-dev
CMD ["npm", "run", "dev"]

FROM client-deps AS client-build
COPY apps/client ./
# The find is a guard, not a fix: the production build emits no sourcemaps today.
RUN npm run build && find dist -name '*.map' -delete

FROM golang:1.27-alpine AS api-dev
RUN go install github.com/air-verse/air@v1.67.4
ENV GOMODCACHE=/cache/mod GOCACHE=/cache/build
WORKDIR /app/apps/api
CMD ["air", "-c", "/app/.docker/air.toml"]

# Both build stages run on the builder's own architecture and cross-compile, so a
# multi-platform build needs no emulation: the runtime stage only copies files in.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS api-build
ARG TARGETOS TARGETARCH TARGETVARIANT
WORKDIR /src
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download
COPY apps/api ./
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} \
    go build -trimpath -ldflags="-s -w" -o /out/shortr .

FROM gcr.io/distroless/static-debian12:nonroot AS runtime
COPY --from=api-build /out/shortr /shortr
COPY --from=client-build /app/apps/client/dist /srv/client
ENV STATIC_DIR=/srv/client PORT=8080
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/shortr"]
