FROM node:22-alpine

WORKDIR /docs

# Copy package files first for better layer caching
COPY package*.json ./

# Install dependencies if package.json exists
RUN if [ -f package.json ]; then npm ci; else npm install; fi

# Copy all documentation files
COPY . .

# Expose Docusaurus default port
EXPOSE 3000

# Default command serves docs, but can be overridden for other commands
# Override examples:
#   docker run <image> npx create-docusaurus@3.9.2 . classic
#   docker run <image> npm run build
CMD ["npm", "run", "start", "--", "--host", "0.0.0.0"]
