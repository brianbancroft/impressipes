# Build the SPA, embed it in the Go binary, ship the binary on distroless.
FROM node:24-alpine AS web
RUN corepack enable
WORKDIR /src
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml tsconfig.base.json ./
COPY apps/web/package.json apps/web/
RUN pnpm install --frozen-lockfile
COPY apps/web apps/web
RUN cd apps/web && pnpm exec vite build

FROM golang:1.24-alpine AS api
WORKDIR /src/apps/api
COPY apps/api/go.mod ./
RUN go mod download
COPY apps/api ./
COPY --from=web /src/dist/apps/web internal/web/static
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=api /out/api /api
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/api"]
