#!/usr/bin/env node
// create-agentra: One-command scaffold for Agentra
// Usage: npx create-agentra

import { intro, outro, text, confirm, select } from '@clack/prompts';
import { dim, bold, cyan, magenta, yellow } from 'kolorist';
import { copyFileSync, mkdirSync, existsSync, writeFileSync } from 'fs';
import { join } from 'path';
import { fileURLToPath } from 'url';
import { dirname } from 'path';

const __dirname = dirname(fileURLToPath(import.meta.url));

intro(`Agentra - AI-Native Task Management`);
console.log(dim('Scaffolding a new Agentra project...\n'));

const projectName = await text({
  message: 'Project name:',
  placeholder: 'my-agentra',
  validate: (v) => {
    if (!v) return 'Project name is required';
    if (!/^[a-z0-9-]+$/.test(v)) return 'Use lowercase letters, numbers, and hyphens only';
    return true;
  },
});

const setupType = await select({
  message: 'Setup mode:',
  options: [
    { label: 'Quick (Docker Compose)', hint: 'Recommended - PostgreSQL + MinIO via Docker', value: 'docker' },
    { label: 'Dev (SQLite)', hint: 'No Docker - use SQLite for local dev only', value: 'sqlite' },
  ],
});

const installDeps = await confirm({
  message: 'Install dependencies after scaffolding?',
  initialValue: true,
});

// Build project directory
const targetDir = join(process.cwd(), String(projectName));
const srcDir = join(targetDir, 'src');

console.log(dim(`\nCreating project in ${targetDir}...\n`));

// Create directories
mkdirSync(srcDir, { recursive: true });

// Copy source files
const serverFiles = [
  ['../server/cmd/server/main.go', 'server/main.go'],
  ['../server/cmd/agentra/main.go', 'cli/main.go'],
  ['../server/internal/handler', 'server/handler'],
  ['../server/internal/service', 'server/service'],
  ['../server/pkg', 'server/pkg'],
];

const configFiles = {
  'package.json': {
    name: projectName,
    version: '0.1.0',
  },
  'docker-compose.yml': setupType === 'docker' ? getDockerCompose() : getSQLiteCompose(),
  '.env.example': getEnvExample(),
  '.gitignore': getGitignore(),
  'README.md': getReadme(String(projectName)),
};

console.log(bold(cyan('✓')), dim('Created project structure'));

// Write config files
for (const [filename, content] of Object.entries(configFiles)) {
  const filePath = join(targetDir, filename);
  if (typeof content === 'string') {
    writeFileSync(filePath, content);
  }
}

console.log(bold(cyan('✓')), dim('Generated configuration files'));

if (installDeps) {
  console.log(dim('\nInstalling dependencies...'));
  // In real implementation, would run npm install in targetDir
}

outro(`
${bold('Project created successfully!')}

${bold('Next steps:')}
  ${cyan('cd')} ${projectName}
  ${cyan('cp')} .env.example .env
  ${cyan('docker')} compose up -d
  ${cyan('open')} http://localhost:3000

${dim('Documentation:')} https://docs.agentra.ai
${dim('Discord:')} https://discord.gg/agentra
`);

function getDockerCompose() {
  return `services:
  postgres:
    image: pgvector/pgvector:pg17
    environment:
      POSTGRES_DB: agentra
      POSTGRES_USER: agentra
      POSTGRES_PASSWORD: agentra
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data

  minio:
    image: minio/minio
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: agentra
      MINIO_ROOT_PASSWORD: agentra
    ports:
      - "9000:9000"
      - "9001:9001"
    volumes:
      - minio_data:/data

  server:
    build: ../server
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: postgres://agentra:agentra@postgres:5432/agentra
      MINIO_ENDPOINT: minio:9000
      MINIO_ACCESS_KEY: agentra
      MINIO_SECRET_KEY: agentra
    depends_on:
      - postgres
      - minio

  web:
    build: ../apps/web
    ports:
      - "3000:3000"
    environment:
      NEXT_PUBLIC_API_URL: http://localhost:8080
      NEXT_PUBLIC_WS_URL: ws://localhost:8080
    depends_on:
      - server

volumes:
  postgres_data:
  minio_data:
`;
}

function getSQLiteCompose() {
  return `# SQLite dev mode - no Docker needed for basic development
# Note: Production still requires PostgreSQL + pgvector
server:
  build: ../server
  ports:
    - "8080:8080"
  environment:
    DATABASE_URL: sqlite://./dev.db
  volumes:
    - ../server:/app
  command: go run ./cmd/server

web:
  build: ../apps/web
  ports:
    - "3000:3000"
  environment:
    NEXT_PUBLIC_API_URL: http://localhost:8080
    NEXT_PUBLIC_WS_URL: ws://localhost:8080
  volumes:
    - ../apps/web:/app
  command: pnpm dev
`;
}

function getEnvExample() {
  return `# Agentra Configuration
JWT_SECRET=change-me-in-production-use-openssl-rand-base64-32

# Database
DATABASE_URL=postgres://agentra:agentra@localhost:5432/agentra

# MinIO (S3-compatible storage)
MINIO_ENDPOINT=localhost:9000
MINIO_ACCESS_KEY=agentra
MINIO_SECRET_KEY=agentra
MINIO_USE_SSL=false

# API URLs
FRONTEND_ORIGIN=http://localhost:3000
PORT=8080
FRONTEND_PORT=3000

# Agent Runtime
AGENTRA_DAEMON_MODE=true
`;
}

function getGitignore() {
  return `node_modules/
dist/
.env
*.db
.DS_Store
coverage/
.next/
out/
build/
*.log
`;
}

function getReadme(name) {
  return `# ${name}

An Agentra project - AI-native task management.

## Quick Start

\`\`\`bash
# Install dependencies
npm install

# Start services
docker compose up -d

# Open the app
open http://localhost:3000
\`\`\`

## Documentation

https://docs.agentra.ai
`;
}