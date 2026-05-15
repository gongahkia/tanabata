FROM node:20-alpine

WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci

COPY . .

EXPOSE 5173
CMD ["sh", "-c", "npx vite --host 0.0.0.0"]
