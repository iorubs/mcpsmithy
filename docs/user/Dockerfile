FROM node:22-alpine

WORKDIR /docs

COPY package*.json ./

VOLUME /docs/node_modules
RUN if [ -f package.json ]; then npm ci; else npm install; fi

COPY . .

EXPOSE 3000

CMD ["npm", "run", "start", "--", "--host", "0.0.0.0"]
