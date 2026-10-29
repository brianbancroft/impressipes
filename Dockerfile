# Build the SPA, embed it in the Go binary, ship the binary on distroless.
FROM node:24-alpine AS web
RUN corepack enable
WORKDIR /src
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml tsconfig.base.json ./
COPY apps/web/package.json apps/web/
RUN pnpm install --frozen-lockfile
COPY apps/web apps/web
RUN cd apps/web && pnpm exec vite build

FROM golang:1.26-alpine AS api
WORKDIR /src/apps/api
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download
COPY apps/api ./
COPY --from=web /src/dist/apps/web internal/web/static
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
# Recipe photos live here; mount a persistent volume at /data in production.
RUN mkdir -p /out/data/images

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=api /out/api /api
COPY --from=api --chown=nonroot:nonroot /out/data /data
ENV PORT=8080
ENV IMAGE_DIR=/data/images
EXPOSE 8080
ENTRYPOINT ["/api"]
