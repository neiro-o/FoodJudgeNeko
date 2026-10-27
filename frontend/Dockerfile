# syntax=docker/dockerfile:1
FROM node:22.23.4-bookworm AS builder
WORKDIR /app
ENV NEXT_TELEMETRY_DISABLED=1
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
# Public client variables must exist during next build. The env file itself
# stays out of image layers and the repository; NEXT_PUBLIC values are
# intentionally compiled into the browser bundle.
RUN --mount=type=secret,id=frontend_env,target=/app/.env,required=true \
    NODE_OPTIONS=--max-old-space-size=4096 npm run build \
    && rm -f .next/standalone/.env .next/standalone/.env.production

FROM node:22.23.4-bookworm-slim AS runner
WORKDIR /app
ENV NODE_ENV=production NEXT_TELEMETRY_DISABLED=1 HOSTNAME=0.0.0.0 PORT=13000
COPY --from=builder --chown=node:node /app/.next/standalone ./
COPY --from=builder --chown=node:node /app/.next/static ./.next/static
COPY --from=builder --chown=node:node /app/public ./public
USER node
EXPOSE 13000
CMD ["node", "server.js"]
