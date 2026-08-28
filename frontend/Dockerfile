FROM node:24-alpine AS build

WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
ARG VITE_API_BASE=http://localhost:8080
ENV VITE_API_BASE=${VITE_API_BASE}
RUN npm run build

FROM nginx:1.29-alpine
# Тот же адрес backend, что вшит в бандл: connect-src обязан совпадать с ним,
# иначе CSP заблокирует первый же запрос к API.
ARG VITE_API_BASE=http://localhost:8080
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY security-headers.conf /etc/nginx/security-headers.conf
RUN sed -i "s|__API_ORIGIN__|${VITE_API_BASE}|" /etc/nginx/security-headers.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
