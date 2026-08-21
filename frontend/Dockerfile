# NEXT_PUBLIC_* vars are baked into the client bundle at build time, not
# read at container start -- so each one has to be a build ARG, not a
# runtime ENV like every Go service's config. They must resolve to
# publicly-reachable URLs (the browser calls them directly), not in-cluster
# Service DNS names -- see infra/k8s/15-ingress.yaml's per-service hosts.
FROM node:22-alpine AS build
WORKDIR /app

ARG NEXT_PUBLIC_AUTH_URL
ARG NEXT_PUBLIC_STREAM_URL
ARG NEXT_PUBLIC_CHAT_URL
ARG NEXT_PUBLIC_SUBSCRIPTION_URL
ARG NEXT_PUBLIC_COMMERCE_URL
ARG NEXT_PUBLIC_NOTIFICATION_URL
ARG NEXT_PUBLIC_SEARCH_URL
ARG NEXT_PUBLIC_RECOMMENDATION_URL
ENV NEXT_PUBLIC_AUTH_URL=$NEXT_PUBLIC_AUTH_URL \
    NEXT_PUBLIC_STREAM_URL=$NEXT_PUBLIC_STREAM_URL \
    NEXT_PUBLIC_CHAT_URL=$NEXT_PUBLIC_CHAT_URL \
    NEXT_PUBLIC_SUBSCRIPTION_URL=$NEXT_PUBLIC_SUBSCRIPTION_URL \
    NEXT_PUBLIC_COMMERCE_URL=$NEXT_PUBLIC_COMMERCE_URL \
    NEXT_PUBLIC_NOTIFICATION_URL=$NEXT_PUBLIC_NOTIFICATION_URL \
    NEXT_PUBLIC_SEARCH_URL=$NEXT_PUBLIC_SEARCH_URL \
    NEXT_PUBLIC_RECOMMENDATION_URL=$NEXT_PUBLIC_RECOMMENDATION_URL

COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:22-alpine AS run
WORKDIR /app
ENV NODE_ENV=production PORT=3000

COPY --from=build /app/public ./public
COPY --from=build /app/.next/standalone ./
COPY --from=build /app/.next/static ./.next/static

EXPOSE 3000
CMD ["node", "server.js"]
