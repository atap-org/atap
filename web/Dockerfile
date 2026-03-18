FROM node:22-alpine AS build
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY spec/ /app/spec/
COPY web/ .
RUN npm run build

FROM nginx:alpine
COPY --from=build /app/web/out /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
