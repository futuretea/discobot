# Stage 1: Build agentfs from source
FROM rust:1.83-slim AS agentfs-builder

RUN apt-get update && apt-get install -y \
    git \
    pkg-config \
    libssl-dev \
    libunwind-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build

# Clone and build agentfs (uses nightly toolchain per rust-toolchain.toml)
RUN git clone https://github.com/tursodatabase/agentfs.git
WORKDIR /build/agentfs/cli
RUN cargo build --release

# Stage 2: Node.js runtime
FROM node:25-slim

# Create octobot user with UID 1000
RUN groupadd -g 1000 octobot && \
    useradd -u 1000 -g octobot -m -s /bin/bash octobot

# Install dependencies for agentfs runtime
RUN apt-get update && apt-get install -y \
    libssl3 \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Copy agentfs binary from builder
COPY --from=agentfs-builder /build/agentfs/cli/target/release/agentfs /usr/local/bin/agentfs

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

# Change ownership to octobot user
RUN chown -R octobot:octobot /app

# Switch to octobot user
USER octobot

# Expose the default port
EXPOSE 3001

# Start the application
CMD ["npm", "start"]
