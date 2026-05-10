#!/usr/bin/env node
// task-gate.js: Claude Code PreToolUse gate — block code edits without active in-progress issue

const { execSync } = require('child_process');
const fs = require('fs');
const path = require('path');
const os = require('os');

try {
  if (process.env.SKIP_TASK_GATE === '1') process.exit(0);

  let input;
  try {
    input = JSON.parse(fs.readFileSync(process.stdin.fd, 'utf8'));
  } catch (e) {
    process.exit(0); // no stdin, skip
  }

  const filePath = input?.tool_input?.file_path;
  if (!filePath) process.exit(0);

  // Only gate code edits
  const codeExts = ['.ts', '.tsx', '.js', '.jsx', '.py', '.go', '.rs', '.java', '.c', '.cpp', '.h', '.sql'];
  const ext = path.extname(filePath);
  if (!codeExts.includes(ext)) process.exit(0);

  // Skip scratchpads and tmp files
  if (filePath.includes('scratchpad') || filePath.includes('tmp')) process.exit(0);

  const branch = execSync('git branch --show-current', { encoding: 'utf8' }).trim();
  const match = branch.match(/(AGENTRA-\d+)/i);
  if (!match) {
    console.error('[Agentra] No active issue on branch. Create/claim an issue before editing code.');
    process.exit(2); // block
  }

  const apiUrl = process.env.AGENTRA_API_URL || 'http://localhost:8080';
  const token = process.env.AGENTRA_API_TOKEN;

  const headers = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const res = await fetch(`${apiUrl}/api/git/active-task?branch=${encodeURIComponent(branch)}`, { headers });
  const data = await res.json();

  if (data?.active && data?.status === 'in_progress') {
    process.exit(0); // allow
  }

  // Use parent PID to allow concurrent calls
  const flagFile = path.join(os.tmpdir(), `.agentra-gate-${process.ppid || 0}`);
  if (!fs.existsSync(flagFile)) {
    fs.writeFileSync(flagFile, '');
    console.error('[Agentra] No in-progress issue found. Start an issue before editing code.');
    console.error('[Agentra] Set SKIP_TASK_GATE=1 to bypass.');
  }
  process.exit(2); // block
} catch (e) {
  process.exit(0); // permissive on failure
}