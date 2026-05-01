#!/usr/bin/env node
/**
 * Local proxy that translates Anthropic Messages API requests (from Claude Code)
 * to OpenAI-compatible Chat Completions format for OpenCode Go.
 *
 * Usage:
 *   ANTHROPIC_API_KEY=sk-xxx node anthropic2openai-proxy.mjs
 *
 * Then point Claude Code at http://127.0.0.1:8082
 */

import http from "node:http";

const PORT = parseInt(process.env.PROXY_PORT || "8082", 10);
const TARGET = "https://opencode.ai/zen/go/v1/chat/completions";
const API_KEY = process.env.ANTHROPIC_API_KEY || "";
const DEFAULT_MODEL = normalizeModel(process.env.OPENCODE_GO_MODEL || "deepseek-v4-pro");

if (!API_KEY) {
  console.error("ANTHROPIC_API_KEY env var is required");
  process.exit(1);
}

function normalizeModel(model) {
  if (!model || typeof model !== "string") return "deepseek-v4-pro";
  return model.replace(/^opencode-go\//, "");
}

// ── Request translation: Anthropic → OpenAI ──────────────────────────

function translateToolResultBlocks(content) {
  // Separate tool_result blocks into individual tool messages
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

function buildTextContent(content) {
  if (typeof content === "string") return content;
  if (Array.isArray(content)) {
    return content
      .filter(b => b.type === "text")
      .map(b => b.text)
      .join("\n");
  }
  return "";
}

function translateMessages(anthropicMsgs) {
  const openaiMsgs = [];

  for (const msg of anthropicMsgs) {
    if (typeof msg.content === "string") {
      // Simple text message
      openaiMsgs.push({ role: msg.role, content: msg.content });
      continue;
    }

    if (!Array.isArray(msg.content)) {
      openaiMsgs.push({ role: msg.role, content: "" });
      continue;
    }

    // Handle content blocks
    const { toolMessages, nonTool } = translateToolResultBlocks(msg.content);

    // Add tool messages first (they follow the user message)
    for (const tm of toolMessages) {
      openaiMsgs.push(tm);
    }

    // Check for tool_use blocks (assistant)
    const toolUses = nonTool.filter(b => b.type === "tool_use");
    const textBlocks = nonTool.filter(b => b.type === "text");

    if (toolUses.length > 0 && msg.role === "assistant") {
      openaiMsgs.push({
        role: "assistant",
        content: textBlocks.map(b => b.text).join("\n") || null,
        tool_calls: toolUses.map(tu => ({
          id: tu.id,
          type: "function",
          function: {
            name: tu.name,
            arguments: JSON.stringify(tu.input),
          },
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
    // Disable thinking mode — reasoning_content causes issues with
    // multi-turn tool conversations since Claude Code doesn't track it
    thinking: { type: "disabled" },
  };

  // System prompt
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
  const tools = translateTools(body.tools);
  if (tools && tools.length > 0) {
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
      type: "message",
      role: "assistant",
      model: data.model,
      content: [{ type: "text", text: "" }],
      stop_reason: "end_turn",
      stop_sequence: null,
      usage: {
        input_tokens: data.usage?.prompt_tokens || 0,
        output_tokens: data.usage?.completion_tokens || 0,
      },
    };
  }

  const content = [];

  // Tool calls
  if (choice.message?.tool_calls) {
    for (const tc of choice.message.tool_calls) {
      content.push({
        type: "tool_use",
        id: tc.id,
        name: tc.function.name,
        input: (() => {
          try { return JSON.parse(tc.function.arguments); }
          catch { return {}; }
        })(),
      });
    }
  }

  // Text content — fall back to reasoning if content is empty
  let textContent = choice.message?.content;
  if (!textContent && choice.message?.reasoning_content) {
    textContent = choice.message.reasoning_content;
  }
  if (textContent) {
    content.push({ type: "text", text: textContent });
  }

  // Determine stop reason
  let stopReason = "end_turn";
  if (choice.finish_reason === "length") stopReason = "max_tokens";
  else if (choice.finish_reason === "tool_calls" || choice.message?.tool_calls?.length > 0) stopReason = "tool_use";
  else if (choice.finish_reason === "stop") stopReason = "end_turn";

  return {
    id: data.id || "msg_" + Math.random().toString(36).slice(2),
    type: "message",
    role: "assistant",
    model: data.model,
    content: content.length > 0 ? content : [{ type: "text", text: "" }],
    stop_reason: stopReason,
    stop_sequence: null,
    usage: {
      input_tokens: data.usage?.prompt_tokens || 0,
      output_tokens: data.usage?.completion_tokens || 0,
    },
  };
}

// ── Streaming translation ─────────────────────────────────────────────

function openAIDeltaToAnthropicEvent(data, messageId, model, state) {
  const delta = data.choices?.[0]?.delta;
  if (!delta) return null;

  // Tool call start
  if (delta.tool_calls) {
    const tc = delta.tool_calls[0];
    if (tc.id) {
      // New tool call starting
      const index = state.counter++;
      state.lastType = "tool_use";
      const event = {
        type: "content_block_start",
        index,
        content_block: {
          type: "tool_use",
          id: tc.id,
          name: tc.function?.name || "",
          input: {},
        },
      };
      return `event: content_block_start\ndata: ${JSON.stringify(event)}\n\n`;
    }
    if (tc.function?.arguments) {
      // Tool call arguments delta
      const event = {
        type: "content_block_delta",
        index: state.counter - 1,
        delta: {
          type: "input_json_delta",
          partial_json: tc.function.arguments,
        },
      };
      return `event: content_block_delta\ndata: ${JSON.stringify(event)}\n\n`;
    }
  }

  // Reasoning delta — skip (DeepSeek internal thinking)
  if (delta.reasoning_content) {
    return null;
  }

  // Text delta
  if (delta.content) {
    if (state.lastType !== "text") {
      // Start a new text block
      const index = state.counter++;
      state.lastType = "text";
      const startEvent = {
        type: "content_block_start",
        index,
        content_block: { type: "text", text: "" },
      };
      const deltaEvent = {
        type: "content_block_delta",
        index,
        delta: { type: "text_delta", text: delta.content },
      };
      return (
        `event: content_block_start\ndata: ${JSON.stringify(startEvent)}\n\n` +
        `event: content_block_delta\ndata: ${JSON.stringify(deltaEvent)}\n\n`
      );
    }
    const event = {
      type: "content_block_delta",
      index: state.counter - 1,
      delta: { type: "text_delta", text: delta.content },
    };
    return `event: content_block_delta\ndata: ${JSON.stringify(event)}\n\n`;
  }

  return null;
}

// ── HTTP helpers ──────────────────────────────────────────────────────

function readBody(req) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    req.on("data", c => chunks.push(c));
    req.on("end", () => {
      try {
        resolve(Buffer.concat(chunks).toString("utf8"));
      } catch (e) {
        reject(e);
      }
    });
    req.on("error", reject);
  });
}

async function forwardRequest(openaiReq) {
  const body = JSON.stringify(openaiReq);
  const resp = await fetch(TARGET, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${API_KEY}`,
    },
    body,
  });
  return resp;
}

// ── Server ────────────────────────────────────────────────────────────

const server = http.createServer(async (req, res) => {
  res.setHeader("Access-Control-Allow-Origin", "*");
  res.setHeader("Access-Control-Allow-Methods", "GET, POST, OPTIONS");
  res.setHeader("Access-Control-Allow-Headers", "*");

  if (req.method === "OPTIONS") {
    res.writeHead(204);
    res.end();
    return;
  }

  // Health check
  if (req.method === "GET" && req.url === "/health") {
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ status: "ok", target: TARGET, model: DEFAULT_MODEL }));
    return;
  }

  // Root level models info (Claude Code may query this)
  if (req.method === "GET" && (req.url === "/" || req.url === "/v1")) {
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ provider: "opencode-go", endpoint: "/v1/messages" }));
    return;
  }

  // Token counting endpoint
  if (req.method === "POST" && req.url.includes("/count_tokens")) {
    const countRawBody = await readBody(req);
    try {
      const countReq = JSON.parse(countRawBody);
      // Simple token estimate: count words and multiply by 1.3
      let text = "";
      for (const msg of countReq.messages || []) {
        text += (typeof msg.content === "string" ? msg.content : JSON.stringify(msg.content)) + " ";
      }
      if (countReq.system) {
        text += (typeof countReq.system === "string" ? countReq.system : JSON.stringify(countReq.system)) + " ";
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

  if (req.method !== "POST") {
    res.writeHead(405);
    res.end("Method Not Allowed");
    return;
  }

  let rawBody;
  try {
    rawBody = await readBody(req);
  } catch {
    res.writeHead(400);
    res.end("Bad Request");
    return;
  }

  let anthropicReq;
  try {
    anthropicReq = JSON.parse(rawBody);
  } catch {
    res.writeHead(400);
    res.end("Invalid JSON");
    return;
  }

  const openaiReq = anthropicToOpenAI(anthropicReq);

  try {
    const upstreamResp = await forwardRequest(openaiReq);

    if (!upstreamResp.ok) {
      const errText = await upstreamResp.text();
      console.error(`[proxy] upstream error ${upstreamResp.status}: ${errText.slice(0, 500)}`);
      res.writeHead(upstreamResp.status, { "Content-Type": "application/json" });
      res.end(JSON.stringify({
        type: "error",
        error: {
          type: "api_error",
          message: `Upstream error: ${upstreamResp.status} - ${errText.slice(0, 200)}`,
        },
      }));
      return;
    }

    if (openaiReq.stream) {
      // Streaming response
      res.writeHead(200, {
        "Content-Type": "text/event-stream",
        "Cache-Control": "no-cache",
        Connection: "keep-alive",
      });

      const messageId = "msg_" + Math.random().toString(36).slice(2);
      const model = openaiReq.model;
      const state = { counter: 0, lastType: null };

      // Send message_start
      const msgStart = {
        type: "message_start",
        message: {
          id: messageId,
          type: "message",
          role: "assistant",
          model: model,
          content: [],
          stop_reason: null,
          stop_sequence: null,
          usage: { input_tokens: 0, output_tokens: 0 },
        },
      };
      res.write(`event: message_start\ndata: ${JSON.stringify(msgStart)}\n\n`);

      // Ping to keep alive
      res.write(": ping\n\n");

      const body = await upstreamResp.text();
      const lines = body.split("\n");
      let finishReason = null;

      for (const line of lines) {
        if (line.startsWith("data: ")) {
          const jsonStr = line.slice(6).trim();
          if (!jsonStr || jsonStr === "[DONE]") continue;

          try {
            const chunk = JSON.parse(jsonStr);
            if (chunk.choices?.[0]?.finish_reason) {
              finishReason = chunk.choices[0].finish_reason;
            }
            const event = openAIDeltaToAnthropicEvent(chunk, messageId, model, state);
            if (event) res.write(event);
          } catch {
            // Skip unparseable chunks
          }
        }
      }

      // content_block_stop for each block
      for (let i = 0; i < state.counter; i++) {
        const stopEvent = { type: "content_block_stop", index: i };
        res.write(`event: content_block_stop\ndata: ${JSON.stringify(stopEvent)}\n\n`);
      }

      // message_delta with stop reason + usage
      let stopReason = "end_turn";
      if (finishReason === "length") stopReason = "max_tokens";
      else if (finishReason === "tool_calls") stopReason = "tool_use";

      const msgDelta = {
        type: "message_delta",
        delta: { stop_reason: stopReason, stop_sequence: null },
        usage: { output_tokens: 0 },
      };
      res.write(`event: message_delta\ndata: ${JSON.stringify(msgDelta)}\n\n`);

      // message_stop
      res.write(`event: message_stop\ndata: ${JSON.stringify({ type: "message_stop" })}\n\n`);

      res.end();
    } else {
      // Non-streaming response
      const openaiResp = await upstreamResp.json();
      const anthropicResp = openAIToAnthropic(openaiResp);
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(JSON.stringify(anthropicResp));
    }
  } catch (err) {
    console.error(`[proxy] error: ${err.message}`);
    res.writeHead(502, { "Content-Type": "application/json" });
    res.end(JSON.stringify({
      type: "error",
      error: { type: "api_error", message: err.message },
    }));
  }
});

server.listen(PORT, "127.0.0.1", () => {
  console.log(`[proxy] Listening on http://127.0.0.1:${PORT}`);
  console.log(`[proxy] Forwarding to ${TARGET}`);
  console.log(`[proxy] Default model: ${DEFAULT_MODEL}`);
});
