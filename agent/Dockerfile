FROM node:25-slim

# Install claude-code-acp globally
RUN npm install -g @zed-industries/claude-code-acp

WORKDIR /app

# Copy package files and install dependencies
COPY package.json package-lock.json* ./
RUN npm install

# Copy source files and build
COPY tsconfig.json ./
COPY src ./src
RUN npm run build

# Expose the default port
EXPOSE 3001

# Start the application
CMD ["npm", "start"]
