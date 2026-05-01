#!/usr/bin/env node
/**
 * Local proxy that translates Anthropic Messages API requests (from Claude Code)
 * to OpenAI-compatible Chat Completions format for OpenCode Go.
 *
 * Usage:
 *   ANTHROPIC_API_KEY=sk-xxx node anthropic2openai-proxy.mjs
 *
 * Then point Claude Code at http://127.0.0.1:8082
 *
 * Claude Code v2.1.126+ will auto-discover models via GET /v1/models.
 */
import http from "node:http";
import fs from "node:fs";
import path from "node:path";
import { randomUUID } from "node:crypto";

const PORT = parseInt(process.env.PROXY_PORT || "8082", 10);
const TARGET = "https://opencode.ai/zen/go/v1/chat/completions";
const MODELS_URL = "https://opencode.ai/zen/go/v1/models";
const API_KEY = process.env.ANTHROPIC_API_KEY || "";
const DEFAULT_MODEL = process.env.OPENCODE_GO_MODEL || "deepseek-v4-pro";
const TRACE_LOG = process.env.CLAUDE_GO_TRACE_LOG || `${process.env.HOME || "."}/.cache/claude-go/traces.jsonl`;
const RETRY_ATTEMPTS = parseInt(process.env.CLAUDE_GO_RETRY_ATTEMPTS || "2", 10);
const RETRY_BASE_MS = parseInt(process.env.CLAUDE_GO_RETRY_BASE_MS || "350", 10);
const FALLBACK_MODELS = (process.env.CLAUDE_GO_FALLBACK_MODELS || "glm-5.1,kimi-k2.6,minimax-m2.7,qwen3.6-plus")
  .split(",")
  .map(m => m.trim())
  .filter(Boolean);
const RETRY_STATUSES = new Set([408, 409, 425, 429, 500, 502, 503, 504]);

if (!API_KEY) {
  console.error("ANTHROPIC_API_KEY env var is required");
  process.exit(1);
}

// ── Model helpers ─────────────────────────────────────────────────────

let cachedModels = null;
let cachedModelsAt = 0;

const GO_MODEL_PREFIXES = new Set(["deepseek-", "glm-", "kimi-", "minimax-", "qwen", "mimo-"]);

function isGoModel(model) {
  if (!model) return false;
  return [...GO_MODEL_PREFIXES].some(p => model.startsWith(p));
}

function normalizeModel(model) {
  if (!model || typeof model !== "string") return "";
  let name = model.replace(/^opencode-go\//, "");
  name = name.replace(/^(claude|anthropic)-/i, "");
  return isGoModel(name) ? name : DEFAULT_MODEL;
}

async function fetchGoModels() {
  if (cachedModels && Date.now() - cachedModelsAt < 300_000) return cachedModels;
  try {
    const resp = await fetch(MODELS_URL, {
      headers: { Authorization: `Bearer ${API_KEY}` },
    });
    if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
    const data = await resp.json();
    const models = (data.data || []).map(m => ({
      id: `claude-${m.id}`,
      display_name: m.id,
      created: m.created || 0,
      owned_by: "opencode-go",
    }));
    cachedModels = models;
    cachedModelsAt = Date.now();
    return models;
  } catch (err) {
    console.error(`[proxy] failed to fetch models: ${err.message}`);
    if (cachedModels) return cachedModels;
    return [];
  }
}

// ── Helpers ───────────────────────────────────────────────────────────

function nowIso() { return new Date().toISOString(); }
function sleep(ms) { return new Promise(r => setTimeout(r, ms)); }

function redact(value) {
  if (typeof value !== "string") return value;
  return value
    .replace(/sk-[A-Za-z0-9_-]{12,}/g, "sk-REDACTED")
    .replace(/[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}/gi, "[email-redacted]");
}

function writeTrace(trace) {
  try {
    fs.mkdirSync(path.dirname(TRACE_LOG), { recursive: true });
    fs.appendFileSync(TRACE_LOG, `${JSON.stringify(trace)}\n`, "utf8");
  } catch {}
}

function createTrace(req, openaiReq) {
  return {
    id: req.headers["x-claude-opencode-trace-id"] || randomUUID(),
    ts: nowIso(),
    model: openaiReq.model,
    finalModel: openaiReq.model,
    status: "started",
    upstreamStatus: null,
    latencyMs: null,
    stream: Boolean(openaiReq.stream),
    retries: 0,
    failovers: [],
    usage: null,
    error: null,
  };
}

function recordUsage(trace, data) {
  if (!data?.usage) return;
  trace.usage = {
    inputTokens: data.usage.prompt_tokens || data.usage.input_tokens || 0,
    outputTokens: data.usage.completion_tokens || data.usage.output_tokens || 0,
    totalTokens: data.usage.total_tokens || 0,
    cost: data.cost || null,
  };
}

function candidateModels(primary, failoverEnabled) {
  if (!failoverEnabled) return [primary];
  return [primary, ...FALLBACK_MODELS.filter(m => m !== primary)];
}

function shouldRetryStatus(s) { return RETRY_STATUSES.has(s); }

// ── Request translation: Anthropic → OpenAI ──────────────────────────

function translateToolResultBlocks(content) {
  const toolMessages = [];
  const nonTool = [];
  for (const block of content) {
    if (block.type === "tool_result") {
      toolMessages.push({
        role: "tool",
        tool_call_id: block.tool_use_id,
        content: typeof block.content === "string"
          ? block.content
          : (Array.isArray(block.content) ? block.content.map(c => c.text || "").join("\n") : ""),
      });
    } else {
      nonTool.push(block);
    }
  }
  return { toolMessages, nonTool };
}

function translateMessages(anthropicMsgs) {
  const openaiMsgs = [];
  for (const msg of anthropicMsgs) {
    if (typeof msg.content === "string") {
      openaiMsgs.push({ role: msg.role, content: msg.content });
      continue;
    }
    if (!Array.isArray(msg.content)) {
      openaiMsgs.push({ role: msg.role, content: "" });
      continue;
    }
    const { toolMessages, nonTool } = translateToolResultBlocks(msg.content);
    for (const tm of toolMessages) openaiMsgs.push(tm);

    const toolUses = nonTool.filter(b => b.type === "tool_use");
    const textBlocks = nonTool.filter(b => b.type === "text");

    if (toolUses.length > 0 && msg.role === "assistant") {
      openaiMsgs.push({
        role: "assistant",
        content: textBlocks.map(b => b.text).join("\n") || null,
        tool_calls: toolUses.map(tu => ({
          id: tu.id,
          type: "function",
          function: { name: tu.name, arguments: JSON.stringify(tu.input) },
        })),
      });
    } else if (textBlocks.length > 0 || nonTool.length > 0) {
      openaiMsgs.push({
        role: msg.role,
        content: textBlocks.map(b => b.text).join("\n") || "",
      });
    }
  }
  return openaiMsgs;
}

function translateTools(anthropicTools) {
  if (!anthropicTools || !Array.isArray(anthropicTools)) return undefined;
  return anthropicTools.map(tool => ({
    type: "function",
    function: {
      name: tool.name,
      description: tool.description || "",
      parameters: tool.input_schema || { type: "object", properties: {} },
    },
  }));
}

function translateToolChoice(toolChoice) {
  if (!toolChoice) return undefined;
  if (typeof toolChoice === "string") {
    if (toolChoice === "any" || toolChoice === "required") return "required";
    if (toolChoice === "auto") return "auto";
    return "auto";
  }
  if (toolChoice.type === "any") return "required";
  if (toolChoice.type === "tool" && toolChoice.name) {
    return { type: "function", function: { name: toolChoice.name } };
  }
  return "auto";
}

function anthropicToOpenAI(body) {
  const req = {
    model: normalizeModel(body.model) || DEFAULT_MODEL,
    max_tokens: Math.max(body.max_tokens ?? 4096, 500),
    messages: [],
  };

  // DeepSeek thinking mode
  if (req.model.startsWith("deepseek-")) {
    req.thinking = { type: "disabled" };
  }

  // System prompt (pass through as-is, no proxy note injection needed)
  if (body.system) {
    let systemContent;
    if (typeof body.system === "string") {
      systemContent = body.system;
    } else if (Array.isArray(body.system)) {
      systemContent = body.system
        .map(b => (typeof b === "string" ? b : b.text || b.content || ""))
        .join("\n");
    }
    if (systemContent) {
      req.messages.push({ role: "system", content: systemContent });
    }
  }

  // Messages
  req.messages.push(...translateMessages(body.messages || []));

  // Tools
  const tools = translateTools(body.tools) || [];
  if (tools.length > 0) {
    req.tools = tools;
    req.tool_choice = translateToolChoice(body.tool_choice) || "auto";
  }

  // Common params
  if (body.temperature !== undefined) req.temperature = body.temperature;
  if (body.top_p !== undefined) req.top_p = body.top_p;
  if (body.top_k !== undefined) req.top_k = body.top_k;
  if (body.stop_sequences) req.stop = body.stop_sequences;
  if (body.stream) req.stream = body.stream;

  return req;
}

// ── Response translation: OpenAI → Anthropic ─────────────────────────

function openAIToAnthropic(data) {
  const choice = data.choices?.[0];
  if (!choice) {
    return {
      id: data.id || "msg_" + Math.random().toString(36).slice(2),
      type: "message", role: "assistant", model: data.model,
      content: [{ type: "text", text: "" }],
      stop_reason: "end_turn", stop_sequence: null,
      usage: { input_tokens: data.usage?.prompt_tokens || 0, output_tokens: data.usage?.completion_tokens || 0 },
    };
  }

  const content = [];

  if (choice.message?.tool_calls) {
    for (const tc of choice.message.tool_calls) {
      content.push({
        type: "tool_use", id: tc.id, name: tc.function.name,
        input: (() => { try { return JSON.parse(tc.function.arguments); } catch { return {}; } })(),
      });
    }
  }

  let textContent = choice.message?.content;
  if (!textContent && choice.message?.reasoning_content) {
    textContent = choice.message.reasoning_content;
  }
  if (textContent) content.push({ type: "text", text: textContent });

  let stopReason = "end_turn";
  if (choice.finish_reason === "length") stopReason = "max_tokens";
  else if (choice.finish_reason === "tool_calls" || choice.message?.tool_calls?.length > 0) stopReason = "tool_use";
  else if (choice.finish_reason === "stop") stopReason = "end_turn";

  return {
    id: data.id || "msg_" + Math.random().toString(36).slice(2),
    type: "message", role: "assistant", model: data.model,
    content: content.length > 0 ? content : [{ type: "text", text: "" }],
    stop_reason: stopReason, stop_sequence: null,
    usage: { input_tokens: data.usage?.prompt_tokens || 0, output_tokens: data.usage?.completion_tokens || 0 },
  };
}

// ── Streaming translation ─────────────────────────────────────────────

function openAIDeltaToAnthropicEvent(data, state) {
  const delta = data.choices?.[0]?.delta;
  if (!delta) return null;

  if (delta.tool_calls) {
    const tc = delta.tool_calls[0];
    if (tc.id) {
      const index = state.counter++;
      state.lastType = "tool_use";
      let events = `event: content_block_start\ndata: ${JSON.stringify({
        type: "content_block_start", index,
        content_block: { type: "tool_use", id: tc.id, name: tc.function?.name || "", input: {} },
      })}\n\n`;
      if (tc.function?.arguments) {
        events += `event: content_block_delta\ndata: ${JSON.stringify({
          type: "content_block_delta", index,
          delta: { type: "input_json_delta", partial_json: tc.function.arguments },
        })}\n\n`;
      }
      return events;
    }
    if (tc.function?.arguments) {
      return `event: content_block_delta\ndata: ${JSON.stringify({
        type: "content_block_delta", index: state.counter - 1,
        delta: { type: "input_json_delta", partial_json: tc.function.arguments },
      })}\n\n`;
    }
  }

  if (delta.reasoning_content) return null;

  if (delta.content) {
    if (state.lastType !== "text") {
      const index = state.counter++;
      state.lastType = "text";
      return (
        `event: content_block_start\ndata: ${JSON.stringify({
          type: "content_block_start", index, content_block: { type: "text", text: "" },
        })}\n\n` +
        `event: content_block_delta\ndata: ${JSON.stringify({
          type: "content_block_delta", index, delta: { type: "text_delta", text: delta.content },
        })}\n\n`
      );
    }
    return `event: content_block_delta\ndata: ${JSON.stringify({
      type: "content_block_delta", index: state.counter - 1,
      delta: { type: "text_delta", text: delta.content },
    })}\n\n`;
  }

  return null;
}

// ── HTTP helpers ──────────────────────────────────────────────────────

function readBody(req) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    req.on("data", c => chunks.push(c));
    req.on("end", () => {
      try { resolve(Buffer.concat(chunks).toString("utf8")); } catch (e) { reject(e); }
    });
    req.on("error", reject);
  });
}

async function forwardRequest(openaiReq, trace, options = {}) {
  const failoverEnabled = options.failoverEnabled !== false;
  let lastResp = null;
  let lastErr = null;

  for (const model of candidateModels(openaiReq.model, failoverEnabled)) {
    const reqForModel = { ...openaiReq, model };
    if (model.startsWith("deepseek-")) {
      reqForModel.thinking = { type: "disabled" };
    } else {
      delete reqForModel.thinking;
    }

    if (model !== openaiReq.model) {
      trace.failovers.push({ from: trace.finalModel, to: model, at: nowIso() });
    }
    trace.finalModel = model;

    for (let attempt = 0; attempt <= RETRY_ATTEMPTS; attempt++) {
      if (attempt > 0) { trace.retries += 1; await sleep(RETRY_BASE_MS * attempt); }
      try {
        const resp = await fetch(TARGET, {
          method: "POST",
          headers: { "Content-Type": "application/json", Authorization: `Bearer ${API_KEY}` },
          body: JSON.stringify(reqForModel),
        });
        trace.upstreamStatus = resp.status;
        lastResp = resp;
        if (resp.ok || !shouldRetryStatus(resp.status)) return resp;
      } catch (err) {
        lastErr = err;
        trace.error = redact(err.message);
      }
    }
  }

  if (lastResp) return lastResp;
  throw lastErr || new Error("upstream request failed");
}

// ── Server ────────────────────────────────────────────────────────────

const server = http.createServer(async (req, res) => {
  res.setHeader("Access-Control-Allow-Origin", "*");
  res.setHeader("Access-Control-Allow-Methods", "GET, POST, OPTIONS");
  res.setHeader("Access-Control-Allow-Headers", "*");

  if (req.method === "OPTIONS") { res.writeHead(204); res.end(); return; }

  // Health check
  if (req.method === "GET" && req.url === "/health") {
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ status: "ok", target: TARGET, model: DEFAULT_MODEL }));
    return;
  }

  // Model discovery (Claude Code v2.1.126+ queries this on startup)
  if (req.method === "GET" && req.url === "/v1/models") {
    const models = await fetchGoModels();
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ object: "list", data: models }));
    return;
  }

  // Token counting
  if (req.method === "POST" && req.url.includes("/count_tokens")) {
    const rawBody = await readBody(req);
    try {
      const ctReq = JSON.parse(rawBody);
      let text = "";
      for (const msg of ctReq.messages || []) {
        text += (typeof msg.content === "string" ? msg.content : JSON.stringify(msg.content)) + " ";
      }
      if (ctReq.system) {
        text += (typeof ctReq.system === "string" ? ctReq.system : JSON.stringify(ctReq.system)) + " ";
      }
      const tokenCount = Math.ceil(text.split(/\s+/).length * 1.3);
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ input_tokens: tokenCount }));
    } catch {
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ input_tokens: 0 }));
    }
    return;
  }

  if (req.method !== "POST") { res.writeHead(405); res.end("Method Not Allowed"); return; }

  let rawBody;
  try { rawBody = await readBody(req); } catch { res.writeHead(400); res.end("Bad Request"); return; }

  let anthropicReq;
  try { anthropicReq = JSON.parse(rawBody); } catch { res.writeHead(400); res.end("Invalid JSON"); return; }

  const openaiReq = anthropicToOpenAI(anthropicReq);
  const trace = createTrace(req, openaiReq);
  const started = Date.now();
  const failoverEnabled = req.headers["x-claude-opencode-no-failover"] !== "1";
  res.setHeader("x-claude-opencode-trace-id", trace.id);

  try {
    const upstreamResp = await forwardRequest(openaiReq, trace, { failoverEnabled });

    if (!upstreamResp.ok) {
      const errText = await upstreamResp.text();
      trace.status = "error";
      trace.upstreamStatus = upstreamResp.status;
      trace.latencyMs = Date.now() - started;
      trace.error = redact(errText.slice(0, 500));
      writeTrace(trace);
      console.error(`[proxy] ${trace.id} upstream error ${upstreamResp.status}: ${errText.slice(0, 500)}`);
      res.writeHead(upstreamResp.status, { "Content-Type": "application/json" });
      res.end(JSON.stringify({
        type: "error",
        error: { type: "api_error", message: `Upstream error: ${upstreamResp.status} - ${errText.slice(0, 200)}` },
      }));
      return;
    }

    if (openaiReq.stream) {
      // Streaming response
      res.writeHead(200, { "Content-Type": "text/event-stream", "Cache-Control": "no-cache", Connection: "keep-alive" });

      const model = openaiReq.model;
      const state = { counter: 0, lastType: null };

      res.write(`event: message_start\ndata: ${JSON.stringify({
        type: "message_start",
        message: { id: "msg_" + Math.random().toString(36).slice(2), type: "message", role: "assistant", model, content: [], stop_reason: null, stop_sequence: null, usage: { input_tokens: 0, output_tokens: 0 } },
      })}\n\n`);
      res.write(": ping\n\n");

      const body = await upstreamResp.text();
      const lines = body.split("\n");
      let finishReason = null;
      let lastUsage = null;

      for (const line of lines) {
        if (line.startsWith("data: ")) {
          const jsonStr = line.slice(6).trim();
          if (!jsonStr || jsonStr === "[DONE]") continue;
          try {
            const chunk = JSON.parse(jsonStr);
            if (chunk.choices?.[0]?.finish_reason) finishReason = chunk.choices[0].finish_reason;
            if (chunk.usage) lastUsage = chunk.usage;
            const event = openAIDeltaToAnthropicEvent(chunk, state);
            if (event) res.write(event);
          } catch { /* skip unparseable chunks */ }
        }
      }

      for (let i = 0; i < state.counter; i++) {
        res.write(`event: content_block_stop\ndata: ${JSON.stringify({ type: "content_block_stop", index: i })}\n\n`);
      }

      let stopReason = "end_turn";
      if (finishReason === "length") stopReason = "max_tokens";
      else if (finishReason === "tool_calls") stopReason = "tool_use";

      res.write(`event: message_delta\ndata: ${JSON.stringify({
        type: "message_delta", delta: { stop_reason: stopReason, stop_sequence: null }, usage: { output_tokens: 0 },
      })}\n\n`);
      res.write(`event: message_stop\ndata: ${JSON.stringify({ type: "message_stop" })}\n\n`);

      if (lastUsage) recordUsage(trace, { usage: lastUsage });
      trace.status = "ok";
      trace.latencyMs = Date.now() - started;
      writeTrace(trace);
      res.end();
    } else {
      // Non-streaming response
      const openaiResp = await upstreamResp.json();
      recordUsage(trace, openaiResp);
      const anthropicResp = openAIToAnthropic(openaiResp);
      trace.status = "ok";
      trace.latencyMs = Date.now() - started;
      writeTrace(trace);
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(JSON.stringify(anthropicResp));
    }
  } catch (err) {
    trace.status = "error";
    trace.latencyMs = Date.now() - started;
    trace.error = redact(err.message);
    writeTrace(trace);
    console.error(`[proxy] ${trace.id} error: ${err.message}`);
    res.writeHead(502, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ type: "error", error: { type: "api_error", message: err.message } }));
  }
});

server.listen(PORT, "127.0.0.1", () => {
  console.log(`[proxy] Listening on http://127.0.0.1:${PORT}`);
  console.log(`[proxy] Forwarding to ${TARGET}`);
  console.log(`[proxy] Default model: ${DEFAULT_MODEL}`);
  fetchGoModels().catch(() => {}); // prefetch models cache
});
