# LLM & Agent Production Engineering — Principal-Level Playbook

> The gap between "it works in a notebook" and "it works in production" is exactly the content of this document: LLM engineering, agent reliability, evaluation, and observability. Everything else (frameworks, prompt libraries, wrapper SDKs) is decoration on top of these four pillars.

---

## Table of Contents

1. [LLM Engineering](#1-llm-engineering)
   - 1.1 Prompt Design
   - 1.2 Structured Generation
   - 1.3 Tool Calling
   - 1.4 Context Engineering
   - 1.5 Model Selection
   - 1.6 Token Economics
   - 1.7 Streaming
   - 1.8 Fallbacks
2. [Agent Reliability](#2-agent-reliability)
   - 2.1 Timeouts
   - 2.2 Retries
   - 2.3 Idempotency
   - 2.4 State Persistence
   - 2.5 Failure Recovery
   - 2.6 Guardrails
   - 2.7 Validation
   - 2.8 Human Escalation
3. [Evaluation](#3-evaluation)
4. [Observability](#4-observability)
5. [Putting It All Together — Reference Architecture](#5-putting-it-all-together)

---

## 1. LLM Engineering

### 1.1 Prompt Design

**Principle:** A prompt is not prose, it's an API contract with a probabilistic function. Treat it like code: versioned, tested, reviewed.

**Best practices**

- **Separate system, developer, and user layers explicitly.** System prompt = identity + non-negotiable constraints. Developer/context prompt = task-specific instructions, retrieved data. User prompt = the actual ask. Never blend all three into one blob — it makes debugging and versioning impossible.
- **Be explicit, not implicit.** State format, tone, length, and edge-case handling instead of hoping the model infers it. "Answer concisely" is weaker than "Answer in 2-3 sentences, no preamble."
- **Use positive instructions over negative ones.** "Only reference the provided documents" beats "Don't make things up." Models follow "do X" more reliably than "don't do Y."
- **Order matters.** Put the most important constraints last (recency bias) or first (primacy bias) depending on model family — test both. For long contexts, critical instructions repeated at the end of the prompt ("reminder" pattern) measurably improve adherence.
- **Few-shot > zero-shot for format-sensitive tasks.** 2-5 diverse, realistic examples reduce format drift dramatically more than more verbose instructions.
- **Use XML/markdown structural tags for long or multi-part prompts** (`<context>`, `<task>`, `<constraints>`, `<examples>`). Models trained with heavy RLHF respond well to structural delimiters — it reduces instruction leakage between sections.
- **Chain-of-thought only when it earns its cost.** For classification/extraction it often *hurts* latency and cost with no accuracy gain. For multi-step reasoning or math it helps. A/B test — don't assume.
- **Version every prompt like code.** Store prompts in files/config, not inline strings scattered through the codebase. Git-diff-able, with changelogs.
- **Treat prompt regressions like code regressions** — one wording tweak can silently break a downstream JSON parser three services away.

**Design pattern: Prompt Template Registry**

```python
from dataclasses import dataclass
from string import Template
from pathlib import Path
import hashlib

@dataclass(frozen=True)
class PromptVersion:
    name: str
    version: str
    template: str
    checksum: str

class PromptRegistry:
    """Central, versioned source of truth for all prompts.
    Never format prompts inline in business logic."""

    def __init__(self, prompt_dir: Path):
        self._prompts: dict[str, PromptVersion] = {}
        for file in prompt_dir.glob("*.tmpl"):
            name, version = file.stem.rsplit("__", 1)
            text = file.read_text()
            checksum = hashlib.sha256(text.encode()).hexdigest()[:8]
            self._prompts[f"{name}:{version}"] = PromptVersion(name, version, text, checksum)

    def render(self, name: str, version: str, **kwargs) -> str:
        key = f"{name}:{version}"
        pv = self._prompts[key]
        return Template(pv.template).substitute(**kwargs)

# Usage
registry = PromptRegistry(Path("prompts/"))
prompt = registry.render("support_triage", "v3", ticket_body="...", policy="...")
```

**Anti-patterns**

- Building prompts via nested f-strings scattered in business logic — impossible to diff, test, or roll back.
- Silent prompt drift: someone tweaks a prompt in prod hotfix without updating the eval dataset that validates it.
- Overloading a single prompt with 15 instructions — split into a pipeline (classify → extract → generate) instead.

---

### 1.2 Structured Generation

**Principle:** If downstream code parses the output, the model must be constrained to only produce parseable output — never rely on "please respond in valid JSON" alone.

**Best practices**

- **Prefer native structured output / tool-forced JSON modes** (e.g., Anthropic tool_use with a JSON schema, OpenAI `response_format=json_schema`, `strict` mode) over prompt-only JSON requests. Grammar-constrained decoding eliminates an entire class of parsing failures.
- **Use Pydantic (or similar) schemas as the single source of truth** — the same schema drives the tool spec sent to the model, the runtime validation, and your type hints.
- **Make schemas as narrow as possible.** Enums instead of free strings wherever the domain is closed. `Literal["low","medium","high"]` beats `str` for a severity field.
- **Always validate, never trust.** Even with constrained decoding, validate ranges, cross-field invariants, and business rules that the schema alone can't express.
- **Design for partial/streamed structured output** if you stream — use libraries that support incremental JSON parsing rather than waiting for the full payload.
- **Version your schemas.** A schema change is a breaking API change for every consumer of that agent's output.

**Design pattern: Schema-first generation with self-repair**

```python
from pydantic import BaseModel, ValidationError, Field
from typing import Literal
import json

class TriageResult(BaseModel):
    category: Literal["billing", "technical", "abuse", "other"]
    severity: Literal["low", "medium", "high", "critical"]
    summary: str = Field(max_length=280)
    requires_human: bool

def generate_structured(client, prompt: str, schema: type[BaseModel], max_repairs: int = 2):
    """Generate + validate + self-repair loop.
    Never let a malformed payload silently propagate downstream."""
    messages = [{"role": "user", "content": prompt}]

    for attempt in range(max_repairs + 1):
        response = client.messages.create(
            model="claude-sonnet-4-6",
            max_tokens=1024,
            tools=[{
                "name": "emit_result",
                "input_schema": schema.model_json_schema(),
            }],
            tool_choice={"type": "tool", "name": "emit_result"},
            messages=messages,
        )
        tool_call = next(b for b in response.content if b.type == "tool_use")
        try:
            return schema.model_validate(tool_call.input)
        except ValidationError as e:
            if attempt == max_repairs:
                raise
            # Feed the validation error back — self-repair, don't just retry blindly
            messages.append({"role": "assistant", "content": response.content})
            messages.append({
                "role": "user",
                "content": f"Your output failed validation: {e}. Correct it and call emit_result again."
            })
```

**Anti-patterns**

- Asking for JSON in prose and regex-extracting it from markdown fences — brittle against any model update.
- Silently `.get()`-ing missing fields with defaults, masking that the model didn't actually produce them.
- One giant schema for everything — split by task; a 40-field schema increases hallucination risk in unrelated fields.

---

### 1.3 Tool Calling

**Principle:** A tool definition is a contract the model reasons over, not documentation for humans. Every ambiguity in the spec becomes a runtime failure mode.

**Best practices**

- **Tool names and descriptions should be unambiguous and action-oriented**: `get_customer_invoices` not `invoices`. Include what it does, when to use it, and — critically — when *not* to use it if there's overlap with another tool.
- **Keep tool surfaces small per call.** More than ~15-20 tools in one context measurably degrades selection accuracy. Use tool *groups* loaded conditionally, or a router agent that narrows the toolset first.
- **Design tools to be idempotent and side-effect-explicit** in their names: `create_order` vs `get_or_create_order`. The model can't know your backend semantics unless the name says so.
- **Return structured, compact tool results.** Don't dump a raw 50KB API response back into context — summarize/paginate server-side before returning to the model.
- **Model the failure states in the tool result itself** (`{"status": "not_found"}` instead of throwing an opaque exception string) so the model can reason about recovery.
- **Bound tool call loops explicitly** — a max iteration count and a max wall-clock budget per agent turn, not just per individual call.
- **Log every tool call + result pair** with correlation IDs — this is your primary reliability debugging surface (see Observability).
- **Never give a tool more permission than the task needs.** A "search_orders" tool should not also expose `delete_order`. Split by blast radius.

**Design pattern: Tool Router for large toolsets**

```python
from typing import Protocol

class Tool(Protocol):
    name: str
    description: str
    schema: dict

class ToolRouter:
    """Two-stage tool calling: first select relevant tool *groups*,
    then expose only those to the execution call. Keeps working-set
    small and selection accuracy high even with 100+ tools registered."""

    def __init__(self, groups: dict[str, list[Tool]]):
        self.groups = groups

    def route(self, client, user_intent: str) -> list[Tool]:
        group_names = list(self.groups.keys())
        response = client.messages.create(
            model="claude-haiku-4-5",  # cheap, fast model for routing
            max_tokens=50,
            messages=[{
                "role": "user",
                "content": f"Given the request: '{user_intent}', which tool groups "
                            f"are relevant? Options: {group_names}. Reply with a JSON list."
            }],
        )
        selected = json.loads(response.content[0].text)
        tools = []
        for g in selected:
            tools.extend(self.groups.get(g, []))
        return tools
```

**Anti-patterns**

- One monolithic tool that takes a free-text `action` string parameter — this defeats the purpose of structured tool calling entirely.
- Tools that return raw stack traces on failure.
- No cap on tool-call loops → infinite "let me search again" spirals burning tokens and latency budget.

---

### 1.4 Context Engineering

**Principle:** Context window is a scarce, expensive, and *attention-diluting* resource. More context is not more accuracy — irrelevant context actively degrades reasoning ("lost in the middle" effect, distraction from noise).

**Best practices**

- **Retrieve, rank, and trim before you inject.** RAG is a context-engineering discipline, not just an embeddings lookup — apply reranking, deduplication, and relevance thresholds before anything enters the prompt.
- **Put the most decision-relevant content near the start or end of the context**, not buried in the middle — models attend less reliably to mid-context content on long inputs.
- **Summarize/compact conversation history instead of appending forever.** Use a rolling summary + last-N-turns window rather than raw transcript concatenation for long-running agent sessions.
- **Separate "static" context (system instructions, tool specs) from "dynamic" context (retrieved docs, conversation).** Static context benefits from prompt caching; dynamic doesn't — structure prompts so caching boundaries align with what actually changes.
- **Chunk retrieved documents with metadata** (source, timestamp, confidence) so the model can reason about *provenance*, not just content — critical for citation and hallucination reduction.
- **Actively prune tool results and old turns** that are no longer relevant to the current sub-task — a context-compaction step is a first-class agent responsibility, not an afterthought.
- **Measure your context "signal-to-noise ratio."** Track the fraction of injected tokens that are actually referenced/used in the eventual response — a proxy for retrieval quality.

**Design pattern: Context Budgeter**

```python
from dataclasses import dataclass, field

@dataclass
class ContextBudget:
    max_tokens: int
    system_reserve: int
    output_reserve: int

    @property
    def available_for_dynamic(self) -> int:
        return self.max_tokens - self.system_reserve - self.output_reserve

class ContextAssembler:
    """Assembles final prompt context under a strict token budget,
    prioritizing by relevance score, never silently overflowing."""

    def __init__(self, budget: ContextBudget, tokenizer):
        self.budget = budget
        self.tokenizer = tokenizer

    def assemble(self, ranked_chunks: list[tuple[str, float]]) -> str:
        # ranked_chunks: (text, relevance_score), pre-sorted descending
        selected, used = [], 0
        for text, score in ranked_chunks:
            n = len(self.tokenizer.encode(text))
            if used + n > self.budget.available_for_dynamic:
                break
            selected.append(text)
            used += n
        return "\n---\n".join(selected)
```

**Anti-patterns**

- "Just append the whole conversation history" for multi-hour agent sessions — guaranteed context-window blowout and quality degradation before that.
- Dumping raw retrieved documents with no relevance filtering "just in case."
- No compaction strategy → agents that get slower and dumber the longer they run.

---

### 1.5 Model Selection

**Principle:** Model choice is a cost/latency/quality optimization problem per *task*, not a single global decision. Principal engineers route, not pick-once.

**Best practices**

- **Profile tasks, not products.** Classification, extraction, and routing rarely need your most expensive model. Complex multi-step reasoning, code generation, and open-ended writing often do.
- **Build a model router / tiering strategy**: small/fast model for triage and simple sub-tasks, larger model for the hard steps, with an escalation path between them.
- **Benchmark on *your* eval set, not public leaderboards.** Public benchmarks are a weak proxy for your specific task distribution.
- **Track model version pinning explicitly.** "Latest" model aliases can silently change behavior — pin exact model IDs in production and re-run your eval suite before bumping versions.
- **Consider fine-tuning / distillation only after prompt + context engineering is exhausted** — it's the most expensive lever and the least reversible.
- **Re-evaluate model choice periodically.** The cost/quality frontier moves fast; a task pinned to an expensive model 6 months ago may now be servable by a cheaper one at equal quality.

**Design pattern: Tiered Model Router**

```python
from enum import Enum

class TaskComplexity(Enum):
    TRIVIAL = "trivial"      # routing, simple classification
    MODERATE = "moderate"    # extraction, single-step reasoning
    COMPLEX = "complex"      # multi-step reasoning, planning, code gen

MODEL_TIER = {
    TaskComplexity.TRIVIAL: "claude-haiku-4-5-20251001",
    TaskComplexity.MODERATE: "claude-sonnet-5",
    TaskComplexity.COMPLEX: "claude-opus-4-8",
}

def select_model(complexity: TaskComplexity, escalate: bool = False) -> str:
    if escalate:
        # bump one tier on retry-after-failure, capped at the top
        tiers = list(TaskComplexity)
        idx = min(tiers.index(complexity) + 1, len(tiers) - 1)
        complexity = tiers[idx]
    return MODEL_TIER[complexity]
```

**Anti-patterns**

- Using the most capable (and expensive) model for every call "to be safe" — this is a cost/latency bug, not a safety measure.
- Never re-benchmarking after a model deprecation notice — silently degrading on auto-migrated aliases.

---

### 1.6 Token Economics

**Principle:** Tokens are your primary unit cost *and* your primary latency driver. Treat token budgets like a cloud infra cost center, with dashboards and alerts.

**Best practices**

- **Measure cost per successful task outcome, not per API call.** A cheap-but-flaky call that needs 3 retries can be more expensive than one premium call that succeeds first try.
- **Use prompt caching aggressively** for static system prompts, tool specs, and long shared context (few-shot examples, retrieved knowledge bases reused across requests).
- **Truncate/paginate tool outputs before they hit context** — most token bloat comes from raw tool results, not from prompts themselves.
- **Batch non-interactive workloads** via async/batch APIs where available — significant cost reduction for offline evaluation, bulk classification, etc.
- **Set hard per-request and per-session token caps** — a runaway agent loop without a token ceiling is a cost incident waiting to happen.
- **Track cost per feature/customer/tenant**, not just aggregate spend — you need to know *which* workflows are expensive to optimize the right ones.

**Design pattern: Token Budget Guard**

```python
class TokenBudgetExceeded(Exception):
    pass

class SessionBudget:
    """Hard ceiling per agent session — prevents runaway loops
    from becoming a cost incident."""

    def __init__(self, max_tokens: int):
        self.max_tokens = max_tokens
        self.used = 0

    def charge(self, input_tokens: int, output_tokens: int):
        self.used += input_tokens + output_tokens
        if self.used > self.max_tokens:
            raise TokenBudgetExceeded(
                f"Session used {self.used} tokens, budget was {self.max_tokens}"
            )

    @property
    def remaining(self) -> int:
        return max(0, self.max_tokens - self.used)
```

**Anti-patterns**

- No per-session or per-tenant token ceiling — a single misbehaving agent loop can silently 10x your daily bill.
- Optimizing prompt length without measuring end-to-end success-rate impact — cutting a few-shot example to save tokens can cost far more in retries.

---

### 1.7 Streaming

**Principle:** Streaming is a UX and reliability tool, not just a latency trick — treat partial output as a first-class data structure to parse and validate incrementally.

**Best practices**

- **Stream to the user for anything over ~1-2 seconds of expected latency** — perceived latency drops dramatically even at equal total time.
- **Design for incremental parsing of structured output** if you stream JSON/tool calls — buffer safely and validate once a structurally complete unit is available, not char-by-char.
- **Handle mid-stream cancellation cleanly** — a user closing a tab mid-generation should stop billing and free the connection, not leak a background completion.
- **Distinguish "stream chunk" errors from "final validation" errors** in your error-handling — a stream can complete successfully at the transport level and still fail schema validation at the end.
- **Buffer tool-call streaming separately from text streaming** — interleaved content blocks need independent incremental parsers.
- **Set stream-level timeouts** (time-to-first-token, time-between-chunks), not just total-request timeouts — a stalled stream is a different failure mode than a slow-but-progressing one.

**Design pattern: Streaming with incremental validation**

```python
import json

async def stream_structured_response(client, prompt: str, schema: type[BaseModel]):
    buffer = ""
    async with client.messages.stream(
        model="claude-sonnet-5",
        max_tokens=2048,
        messages=[{"role": "user", "content": prompt}],
    ) as stream:
        async for event in stream:
            if event.type == "content_block_delta":
                buffer += event.delta.text
                yield {"type": "partial", "text": buffer}  # UX: show progress

        final = await stream.get_final_message()

    try:
        parsed = schema.model_validate_json(buffer)
        yield {"type": "final", "data": parsed}
    except ValidationError as e:
        yield {"type": "error", "detail": str(e)}
```

**Anti-patterns**

- Buffering the entire response server-side and only then "streaming" it to the client — defeats the entire purpose.
- No time-to-first-token timeout — a hung connection looks identical to a slow-but-working one without this signal.

---

### 1.8 Fallbacks

**Principle:** Every external call (model API, tool, retrieval backend) *will* fail in production. The question is never "if," it's "what happens next," and that must be designed, not improvised.

**Best practices**

- **Multi-provider / multi-region fallback for model calls** — if your primary model API degrades or rate-limits, fail over to a secondary provider or region rather than surfacing an error to the user.
- **Degrade gracefully, not silently.** A fallback to a smaller model or cached answer should be logged and, where relevant, surfaced ("this answer used a backup system") rather than pretending nothing happened.
- **Define fallback *quality* tiers explicitly**: full agent → simpler single-call prompt → cached/templated response → human handoff. Know which tier you're in at all times.
- **Circuit-break, don't just retry** — after N consecutive failures to a dependency, stop hammering it and switch to fallback immediately; retry the primary path on a cooldown.
- **Test your fallback paths in CI, not just your happy path** — a fallback that's never actually exercised until a real outage is a fallback you don't actually have.
- **Fallbacks need their own evaluation** — a degraded-mode answer should still pass a (lower but nonzero) quality bar, or it's better to fail fast to human escalation.

**Design pattern: Layered Fallback Chain**

```python
from typing import Callable, Any

class FallbackChain:
    """Ordered chain of strategies, each with its own circuit breaker.
    Falls through to the next tier on failure, never surfaces
    a raw exception to the caller."""

    def __init__(self, strategies: list[Callable[..., Any]]):
        self.strategies = strategies

    def execute(self, *args, **kwargs) -> tuple[Any, int]:
        last_error = None
        for tier, strategy in enumerate(self.strategies):
            try:
                return strategy(*args, **kwargs), tier
            except Exception as e:  # noqa: broad on purpose — fallback boundary
                last_error = e
                continue
        raise RuntimeError(f"All fallback tiers exhausted: {last_error}")

# Usage
chain = FallbackChain([
    lambda q: full_agent_answer(q),        # tier 0: full quality
    lambda q: single_call_answer(q),       # tier 1: degraded quality
    lambda q: cached_template_answer(q),   # tier 2: static fallback
])
answer, tier_used = chain.execute(user_query)
if tier_used > 0:
    logger.warning("degraded_response", tier=tier_used)
```

**Anti-patterns**

- Retrying the same failing provider indefinitely instead of failing over.
- Fallback paths that have never been load-tested or eval-tested — they become the actual bug the first time they're needed.

---

## 2. Agent Reliability

### 2.1 Timeouts

**Principle:** Every network call, model call, and tool call needs an explicit timeout — "no timeout" is not neutral, it's an unbounded liability.

**Best practices**

- **Set timeouts at every layer**: per-LLM-call, per-tool-call, per-agent-turn, per-session. A single missing layer means one hang can consume the whole budget.
- **Use different timeout classes for different call types** — a retrieval call and a multi-step reasoning call have very different acceptable latencies; one global timeout constant is a smell.
- **Timeout ≠ failure** — treat timeout as a distinct outcome from an application error; it should route to its own retry/fallback policy, not be conflated with a 500.
- **Propagate deadlines, don't just set local timeouts.** If an agent turn has an 30s overall deadline, each sub-call should receive the *remaining* budget, not its own independent 30s.
- **Cancel cleanly on timeout** — free the underlying connection/thread, don't leave orphaned background work still consuming compute/tokens after the caller gave up.

**Design pattern: Deadline propagation**

```python
import time
import asyncio

class Deadline:
    """Propagates a shared wall-clock deadline through nested async calls,
    rather than each layer inventing its own independent timeout."""

    def __init__(self, timeout_s: float):
        self.deadline = time.monotonic() + timeout_s

    @property
    def remaining(self) -> float:
        return max(0.0, self.deadline - time.monotonic())

    def check(self):
        if self.remaining <= 0:
            raise TimeoutError("Deadline exceeded")

async def run_agent_turn(deadline: Deadline, agent_step):
    deadline.check()
    return await asyncio.wait_for(agent_step(), timeout=deadline.remaining)

async def agent_loop(user_query: str):
    deadline = Deadline(timeout_s=30)
    for step in build_steps(user_query):
        result = await run_agent_turn(deadline, step)
        deadline.check()
```

**Anti-patterns**

- One global `requests.get(url)` with no timeout anywhere in the tool layer — the classic silent hang that eventually exhausts a thread pool.
- Independent per-call timeouts that don't respect an overall session deadline — you can "successfully" time out five sub-calls and still blow the user-facing SLA.

---

### 2.2 Retries

**Principle:** Retries convert transient failures into successes and *amplify* permanent failures into cascading load. The retry policy must know the difference.

**Best practices**

- **Classify errors before retrying**: retryable (rate limit, 5xx, timeout) vs non-retryable (4xx validation, auth failure, content policy block). Retrying a 400 forever is a bug, not resilience.
- **Exponential backoff with jitter, always.** Fixed-interval retries synchronize across clients and create thundering-herd spikes on your dependency.
- **Cap total retry budget per request AND per time window** — a retry storm during a provider outage can be worse than the outage itself.
- **Retries must be paired with idempotency** (see 2.3) — retrying a non-idempotent side-effecting tool call (e.g., `charge_card`) without a dedup key causes duplicate side effects, not resilience.
- **Different retry policies per call type.** A read-only retrieval call can retry aggressively; a payment tool call should retry conservatively and only with an idempotency key present.
- **Surface retry exhaustion distinctly from first-attempt failure** in your logs/metrics — "failed after 3 retries" and "failed immediately" are different reliability signals.

**Design pattern: Policy-driven retry with backoff+jitter**

```python
import random
import time
from dataclasses import dataclass

@dataclass
class RetryPolicy:
    max_attempts: int
    base_delay_s: float
    max_delay_s: float
    retryable: Callable[[Exception], bool]

def with_retry(policy: RetryPolicy):
    def decorator(fn):
        def wrapped(*args, **kwargs):
            last_exc = None
            for attempt in range(policy.max_attempts):
                try:
                    return fn(*args, **kwargs)
                except Exception as e:
                    if not policy.retryable(e) or attempt == policy.max_attempts - 1:
                        raise
                    last_exc = e
                    delay = min(policy.base_delay_s * (2 ** attempt), policy.max_delay_s)
                    delay *= random.uniform(0.5, 1.5)  # jitter
                    logger.warning("retrying", attempt=attempt, delay=delay, error=str(e))
                    time.sleep(delay)
            raise last_exc
        return wrapped
    return decorator

RATE_LIMIT_POLICY = RetryPolicy(
    max_attempts=4, base_delay_s=1.0, max_delay_s=20.0,
    retryable=lambda e: isinstance(e, (RateLimitError, TimeoutError, ServerError)),
)

@with_retry(RATE_LIMIT_POLICY)
def call_llm(prompt): ...
```

**Anti-patterns**

- Blanket `try/except: retry()` around an entire agent turn, retrying validation errors and business-logic failures the same way as network blips.
- No jitter → synchronized retry storms across concurrent requests during a partial outage.
- Retrying a side-effecting tool call without an idempotency key.

---

### 2.3 Idempotency

**Principle:** Any tool call that mutates external state must be safe to execute more than once with the same input — because retries, timeouts, and process crashes *will* cause duplicate invocations.

**Best practices**

- **Generate an idempotency key per logical action**, not per HTTP request — derive it from the agent's decision (e.g., hash of `session_id + step_id + action`), so a retried call reuses the same key.
- **Push idempotency to the tool/backend layer**, not just the agent orchestration layer — the agent framework retrying "nicely" doesn't help if the downstream payment API isn't idempotent itself.
- **Design tools as upserts where possible** (`ensure_ticket_exists` vs `create_ticket`) — inherently idempotent operations remove an entire failure class.
- **Store a dedup ledger** for actions taken, keyed by idempotency key, with a TTL matching your retry window — check-before-act, not just hope-the-backend-handles-it.
- **Idempotency and exactly-once are different guarantees** — most systems realistically offer *at-least-once delivery + idempotent processing* = effectively-once outcome. Design for that, not a false "exactly once" promise.

**Design pattern: Idempotency ledger**

```python
import hashlib
from datetime import datetime, timedelta

class IdempotencyLedger:
    """Before executing a side-effecting action, check if it was
    already performed under this key. Store, a Redis/DB table in
    production; dict here for illustration."""

    def __init__(self, store: dict, ttl: timedelta = timedelta(hours=24)):
        self.store = store
        self.ttl = ttl

    def make_key(self, session_id: str, step_id: str, action: str, payload: dict) -> str:
        raw = f"{session_id}:{step_id}:{action}:{sorted(payload.items())}"
        return hashlib.sha256(raw.encode()).hexdigest()

    def execute_once(self, key: str, fn: Callable[[], Any]) -> Any:
        existing = self.store.get(key)
        if existing and existing["expires_at"] > datetime.utcnow():
            return existing["result"]  # already done — return cached result, don't repeat side effect

        result = fn()
        self.store[key] = {"result": result, "expires_at": datetime.utcnow() + self.ttl}
        return result

# Usage inside a tool call
def charge_customer_tool(session_id, step_id, amount):
    key = ledger.make_key(session_id, step_id, "charge_customer", {"amount": amount})
    return ledger.execute_once(key, lambda: payment_api.charge(amount))
```

**Anti-patterns**

- Assuming "the agent won't call this tool twice" — under retries and timeouts, it will.
- Idempotency keys derived from wall-clock time (`datetime.now()`) — different on every retry, defeating the purpose.
- Relying only on orchestration-layer retry suppression with no backend-side dedup.

---

### 2.4 State Persistence

**Principle:** An agent's working state (conversation, plan, intermediate results, tool call history) must survive process restarts, deploys, and crashes — treat it as durable data, not in-memory convenience.

**Best practices**

- **Persist state after every meaningful step, not just at the end** — a crash mid-plan should be resumable from the last checkpoint, not from scratch.
- **Separate "conversation state" from "execution state."** Conversation state (messages) is mostly append-only; execution state (current plan step, tool results, retry counts) is mutable and needs its own schema.
- **Version your state schema** — a deploy that changes the agent's internal state shape must handle in-flight sessions created under the old schema (migrate or gracefully fail closed).
- **Make state externally inspectable** — support/on-call engineers need to be able to look at a stuck session's state without redeploying debug code.
- **Choose the storage tier by durability need**: Redis for hot short-lived session state, a real database/event log for anything that must survive infra failure or needs audit history.
- **Model long-running agents as resumable workflows** (state machine / durable execution) rather than a single long-lived in-process loop — process death should lose at most one step, not the whole session.

**Design pattern: Checkpointed state machine**

```python
from enum import Enum, auto
from dataclasses import dataclass, field, asdict
import json

class AgentState(Enum):
    PLANNING = auto()
    EXECUTING_TOOL = auto()
    AWAITING_HUMAN = auto()
    DONE = auto()
    FAILED = auto()

@dataclass
class AgentCheckpoint:
    session_id: str
    state: AgentState
    plan: list[str]
    current_step: int
    tool_results: dict = field(default_factory=dict)
    retry_counts: dict = field(default_factory=dict)

class CheckpointStore:
    """Durable persistence for agent state — Postgres/Redis in prod."""

    def __init__(self, backend):
        self.backend = backend

    def save(self, checkpoint: AgentCheckpoint):
        payload = asdict(checkpoint)
        payload["state"] = checkpoint.state.name
        self.backend.set(checkpoint.session_id, json.dumps(payload))

    def load(self, session_id: str) -> AgentCheckpoint | None:
        raw = self.backend.get(session_id)
        if not raw:
            return None
        data = json.loads(raw)
        data["state"] = AgentState[data["state"]]
        return AgentCheckpoint(**data)

def resume_or_start(store: CheckpointStore, session_id: str, initial_plan):
    checkpoint = store.load(session_id)
    if checkpoint is None:
        checkpoint = AgentCheckpoint(session_id, AgentState.PLANNING, initial_plan, 0)
        store.save(checkpoint)
    return checkpoint
```

**Anti-patterns**

- Holding full agent state only in a process-local variable/queue — a pod restart during a long agent run silently drops the entire session.
- No schema versioning → deploy breaks every in-flight session mid-execution.
- Debugging "stuck" sessions only via application logs, with no way to inspect current persisted state directly.

---

### 2.5 Failure Recovery

**Principle:** Failure recovery is a designed state machine, not a `try/except: log and move on`. Every failure mode needs an explicit, tested recovery path.

**Best practices**

- **Classify failures by recoverability**: transient (retry), degradable (fallback tier), terminal (fail + escalate). Don't handle all three the same way.
- **Recovery should resume from the last good checkpoint**, not restart the whole agent run — expensive multi-step plans shouldn't be thrown away for one failed sub-step.
- **Distinguish partial success from total failure.** If an agent completed 3 of 5 planned actions before failing, the recovery path must know what's already done (idempotency + state persistence make this possible) rather than re-doing completed side effects.
- **Build a dead-letter path** for un-recoverable failures — don't let them vanish into a log line nobody reads; route to a queue/dashboard for manual triage.
- **Test failure injection in staging** (chaos engineering lite) — kill the process mid-tool-call, simulate a timeout mid-stream, and verify the recovery path actually works, not just that it exists on paper.
- **Recovery actions themselves can fail** — design recovery paths with their own bounded retry, not infinite recursive recovery-of-recovery.

**Design pattern: Recovery-aware executor**

```python
class RecoverableExecutionError(Exception):
    def __init__(self, checkpoint: AgentCheckpoint, cause: Exception):
        self.checkpoint = checkpoint
        self.cause = cause

def execute_plan(checkpoint: AgentCheckpoint, store: CheckpointStore):
    try:
        for i in range(checkpoint.current_step, len(checkpoint.plan)):
            step = checkpoint.plan[i]
            result = execute_step(step, checkpoint)  # idempotent per 2.3
            checkpoint.tool_results[step] = result
            checkpoint.current_step = i + 1
            store.save(checkpoint)  # checkpoint after every step
        checkpoint.state = AgentState.DONE
        store.save(checkpoint)
        return checkpoint
    except Exception as e:
        checkpoint.state = AgentState.FAILED
        store.save(checkpoint)
        raise RecoverableExecutionError(checkpoint, e)

def recover(session_id: str, store: CheckpointStore):
    checkpoint = store.load(session_id)
    if checkpoint is None:
        raise ValueError("no checkpoint to recover from")
    if checkpoint.state == AgentState.FAILED:
        # resumes from checkpoint.current_step, doesn't redo completed steps
        checkpoint.state = AgentState.PLANNING
        return execute_plan(checkpoint, store)
```

**Anti-patterns**

- Restarting a multi-step agent plan from step 0 after any failure — wastes tokens/cost and can duplicate side effects if steps weren't idempotent.
- Swallowing exceptions with a bare `except: pass` "to keep the demo running" — this is the single most common cause of silent production data loss in agent systems.
- No dead-letter/triage path — unrecoverable failures just disappear.

---

### 2.6 Guardrails

**Principle:** Guardrails constrain what the agent is *allowed* to do, independent of what it was *instructed* to do — defense in depth against prompt injection, model error, and scope creep.

**Best practices**

- **Enforce guardrails outside the model, in code** — a system prompt saying "never delete production data" is a suggestion; a tool-permission layer that simply doesn't expose `delete_*` tools in that context is a guarantee.
- **Layer guardrails**: input filtering (prompt injection detection on retrieved/untrusted content), output filtering (PII leakage, policy violations), and action filtering (permission checks before tool execution).
- **Treat any content the agent reads from the outside world as untrusted input**, on par with user input — a webpage, email, or document fetched by a tool can contain injected instructions ("ignore previous instructions...") and must not be able to directly trigger tool calls without a trust boundary.
- **Use allow-lists over block-lists for high-risk actions** — enumerate what's permitted (financial limits, allowed recipients, allowed file paths) rather than trying to enumerate every bad action.
- **Rate-limit and scope actions by session/tenant** — a single compromised or malfunctioning session should not be able to exhaust a shared resource or affect other tenants.
- **Guardrail failures should fail closed**, not open — if the guardrail check itself errors, block the risky action rather than letting it through.

**Design pattern: Layered guardrail pipeline**

```python
from dataclasses import dataclass

@dataclass
class GuardrailViolation(Exception):
    layer: str
    reason: str

class GuardrailPipeline:
    """Applies input, output, and action guardrails as independent,
    composable layers. Fails closed on internal error."""

    def __init__(self, input_checks, output_checks, action_checks):
        self.input_checks = input_checks
        self.output_checks = output_checks
        self.action_checks = action_checks

    def check_input(self, untrusted_content: str):
        for check in self.input_checks:
            try:
                if not check(untrusted_content):
                    raise GuardrailViolation("input", check.__name__)
            except GuardrailViolation:
                raise
            except Exception:
                raise GuardrailViolation("input", f"{check.__name__}_errored")  # fail closed

    def check_action(self, tool_name: str, tool_args: dict, session_scope: dict):
        for check in self.action_checks:
            if not check(tool_name, tool_args, session_scope):
                raise GuardrailViolation("action", f"{tool_name}_denied")

def detect_prompt_injection(text: str) -> bool:
    """Returns True if content passes (is safe)."""
    suspicious_markers = ["ignore previous instructions", "system:", "you are now"]
    return not any(m in text.lower() for m in suspicious_markers)

def within_financial_limit(tool_name, args, scope):
    if tool_name != "issue_refund":
        return True
    return args.get("amount", 0) <= scope.get("max_refund", 0)
```

**Anti-patterns**

- Relying solely on the system prompt to prevent dangerous actions, with no code-level enforcement.
- Treating tool-retrieved web/email/document content as trusted the same way user chat input is (after all, it wasn't typed by the user, but it wasn't vetted either — treat as untrusted).
- Guardrail checks that fail open (allow the action) when the check itself throws an error.

---

### 2.7 Validation

**Principle:** Validation is the boundary between "the model produced tokens" and "the system acted on facts." Every boundary crossing needs a validator — inputs, tool arguments, and outputs alike.

**Best practices**

- **Validate at every boundary**: user input → agent, retrieved context → agent, tool arguments → tool execution, model output → downstream consumer. Four distinct boundaries, four distinct validators.
- **Prefer schema validation (Pydantic/JSON Schema) over ad-hoc `if` checks** — declarative, testable, and self-documenting.
- **Validate semantics, not just shape.** A `date` field can be a syntactically valid ISO date that's still nonsensical (e.g., a refund date in the future, an age of 250) — add business-rule validators beyond type-checking.
- **Fail validation loudly and specifically** — a validation error should carry enough detail for a self-repair loop (see 1.2) or a human to fix, not just "invalid input."
- **Validate tool arguments *before* execution**, not just the final output — an agent hallucinating an out-of-range parameter should be caught before the side effect, not after.
- **Add "sanity check" validators for known failure modes** particular to your domain — e.g., an amount field that's off by a factor of 100 due to cents/dollars confusion is common enough to warrant an explicit bound check.

**Design pattern: Multi-boundary validation**

```python
from pydantic import BaseModel, field_validator
from datetime import date

class RefundRequest(BaseModel):
    order_id: str
    amount_cents: int
    reason: str

    @field_validator("amount_cents")
    @classmethod
    def sane_amount(cls, v):
        if v <= 0:
            raise ValueError("amount must be positive")
        if v > 1_000_000:  # $10,000 — business-rule ceiling, not just type validity
            raise ValueError("amount exceeds automatic-approval ceiling")
        return v

def validate_and_execute_tool(tool_name: str, raw_args: dict, schema: type[BaseModel]):
    try:
        validated = schema.model_validate(raw_args)
    except ValidationError as e:
        return {"status": "validation_error", "detail": e.errors()}
    return execute_tool(tool_name, validated)
```

**Anti-patterns**

- Validating only the final user-facing output while trusting tool arguments blindly — the more dangerous boundary is usually the one closer to the side effect.
- Type-only validation with no business-rule checks — "syntactically valid but semantically absurd" data still causes production incidents.
- Vague validation errors ("invalid request") that give a self-repair loop nothing to correct.

---

### 2.8 Human Escalation

**Principle:** Human-in-the-loop is not a failure of automation — it's a designed reliability tier that should be as deliberately engineered as any other fallback.

**Best practices**

- **Define explicit escalation triggers upfront**: confidence-below-threshold, guardrail violation, repeated failure, high-stakes action (irreversible or above a cost/impact threshold), and user-requested escalation.
- **Escalation should hand off full context**, not just "something went wrong" — the human needs the conversation, the agent's reasoning/plan, tool results so far, and *why* it escalated.
- **Design the escalation UX as a queue with priority, not a raw log dump** — group by urgency and give the reviewer the minimum context needed to act fast.
- **Track escalation rate as a first-class product metric** — a rising escalation rate is an early signal of model/prompt drift or a new failure mode, often before it shows up in explicit error metrics.
- **Close the loop: feed human resolutions back into evaluation data** — every escalation resolved by a human is a labeled example for your next eval set or fine-tuning round.
- **Make escalation reversible where possible** — allow the human decision to be captured as a policy update (a new guardrail rule, a new few-shot example) rather than a one-off manual fix that recurs forever.

**Design pattern: Escalation with full context handoff**

```python
from dataclasses import dataclass, field
from datetime import datetime

@dataclass
class EscalationTicket:
    session_id: str
    reason: str
    confidence: float | None
    conversation: list[dict]
    agent_plan: list[str]
    tool_results: dict
    created_at: datetime = field(default_factory=datetime.utcnow)
    priority: str = "normal"

class EscalationQueue:
    def __init__(self, backend):
        self.backend = backend

    def escalate(self, checkpoint: AgentCheckpoint, reason: str, confidence: float | None = None):
        priority = "high" if reason in {"guardrail_violation", "financial_high_risk"} else "normal"
        ticket = EscalationTicket(
            session_id=checkpoint.session_id,
            reason=reason,
            confidence=confidence,
            conversation=checkpoint.tool_results.get("conversation", []),
            agent_plan=checkpoint.plan,
            tool_results=checkpoint.tool_results,
            priority=priority,
        )
        self.backend.enqueue(ticket)
        return ticket

def should_escalate(confidence: float, threshold: float = 0.6) -> bool:
    return confidence < threshold
```

**Anti-patterns**

- Escalating with just an error message and no conversation/plan context — forces the human to reconstruct the situation from scratch.
- No confidence signal or explicit trigger — escalation happens ad hoc / inconsistently, or never happens until a customer complains.
- Treating every human resolution as disposable instead of feeding it back into eval sets and guardrail rules.

---

## 3. Evaluation

**Principle:** "It works" is not a metric. An agent system needs a measurable, repeatable, versioned evaluation harness *before* it ships, not after the first incident.

### 3.1 What to measure

```
                     Agent
                       │
               ┌───────┴───────┐
               ▼               ▼
            Success          Failure
               │                │
        ┌──────┼──────┐    ┌────┴────┐
        ▼      ▼      ▼    ▼         ▼
      quality latency cost  type   frequency
```

- **Quality**: task-specific correctness (exact match, rubric-scored, LLM-judged, human-reviewed) — pick the cheapest method that's still valid for the task.
- **Latency**: p50/p90/p99, *and* time-to-first-token separately from time-to-completion for streamed responses.
- **Cost**: tokens and $ per successful outcome (not per call — a retried call costs more than its token count implies).
- **Failure taxonomy**: classify failures (hallucination, tool error, timeout, guardrail block, validation error, escalation) — an aggregate "failure rate" hides which failure mode to fix first.

### 3.2 Best practices

- **Build a golden dataset before you build the agent, not after.** Even 30-50 hand-labeled representative cases catch most regressions; grow it over time from production near-misses.
- **Separate unit-level evals from end-to-end evals.** Unit: does the extraction tool call get the right arguments given this input? End-to-end: does the full multi-turn session reach the correct final outcome? Both are needed; end-to-end alone hides *where* things break.
- **Use LLM-as-judge carefully**: calibrate the judge against human-labeled examples first, use a rubric (not "rate 1-10"), and periodically audit judge/human agreement — an uncalibrated judge just launders bias into a number that looks objective.
- **Track eval metrics per prompt/model version**, not just as a rolling aggregate — you need to attribute a quality change to the change that caused it.
- **Run the eval suite in CI on every prompt or agent-logic change** — a prompt tweak is a code change and needs the same regression gate as a code change.
- **Build regression tests from every production incident** — every real failure becomes a permanent eval case, so it can never silently regress again.
- **Separate offline eval (dataset-driven, pre-deploy) from online eval (live traffic sampling, post-deploy)** — offline catches known regressions fast; online catches distribution shift and long-tail failures offline data doesn't cover.
- **A/B test model/prompt changes in production behind a flag** before full rollout, with the eval metrics as the guardrail for automatic rollback.

**Design pattern: Eval harness skeleton**

```python
from dataclasses import dataclass
from typing import Callable, Any

@dataclass
class EvalCase:
    id: str
    input: Any
    expected: Any
    tags: list[str]

@dataclass
class EvalResult:
    case_id: str
    passed: bool
    score: float
    latency_ms: float
    cost_tokens: int
    failure_mode: str | None = None

class EvalHarness:
    def __init__(self, cases: list[EvalCase], scorer: Callable[[Any, Any], float],
                 pass_threshold: float = 0.8):
        self.cases = cases
        self.scorer = scorer
        self.pass_threshold = pass_threshold

    def run(self, agent_fn: Callable[[Any], Any]) -> list[EvalResult]:
        results = []
        for case in self.cases:
            start = time.monotonic()
            try:
                output = agent_fn(case.input)
                score = self.scorer(output, case.expected)
                results.append(EvalResult(
                    case_id=case.id,
                    passed=score >= self.pass_threshold,
                    score=score,
                    latency_ms=(time.monotonic() - start) * 1000,
                    cost_tokens=getattr(output, "token_usage", 0),
                ))
            except Exception as e:
                results.append(EvalResult(
                    case_id=case.id, passed=False, score=0.0,
                    latency_ms=(time.monotonic() - start) * 1000,
                    cost_tokens=0, failure_mode=type(e).__name__,
                ))
        return results

    def summary(self, results: list[EvalResult]) -> dict:
        n = len(results)
        return {
            "pass_rate": sum(r.passed for r in results) / n,
            "avg_score": sum(r.score for r in results) / n,
            "p90_latency_ms": sorted(r.latency_ms for r in results)[int(n * 0.9)],
            "total_cost_tokens": sum(r.cost_tokens for r in results),
            "failure_modes": {r.failure_mode for r in results if r.failure_mode},
        }
```

**Anti-patterns**

- Shipping an agent with "I tested it manually a few times" as the entire evaluation strategy.
- One aggregate accuracy number with no failure-mode breakdown — you can't prioritize a fix for "38% failure rate," but you can for "22% tool-argument hallucination, 10% timeout, 6% guardrail block."
- Eval dataset that never grows — production incidents that aren't converted into eval cases will recur.
- LLM-judge scores trusted without ever checking them against human judgment.

---

## 4. Observability

**Principle:** If you can't reconstruct exactly what happened for any single request after the fact — every prompt, every tool call, every model response, every retry — you don't have a production agent system, you have a black box with a UI.

### 4.1 Trace hierarchy

```
request
  ↓
graph            (the overall agent workflow / DAG for this request)
  ↓
node             (a step within the graph — plan, retrieve, generate, act)
  ↓
agent            (a specific sub-agent or role, if multi-agent)
  ↓
tool             (individual tool invocation)
  ↓
LLM              (individual model API call — prompt, response, tokens, latency)
```

### 4.2 Best practices

- **Every layer of the hierarchy gets a span with a shared trace ID** — a single `request_id` must let you reconstruct the entire tree, from top-level request down to individual LLM calls, in your tracing backend (OpenTelemetry-compatible ideally).
- **Log the full prompt and full response for every LLM call**, not a truncated summary — you cannot debug a bad output without the exact input that produced it. Redact PII at the logging layer, don't skip logging entirely.
- **Attach structured metadata to every span**: model version, prompt version, token counts (input/output/cached), latency, cost, retry count, tier used (see fallbacks).
- **Correlate agent decisions with outcomes** — tag each span with the eventual success/failure of the overall request so you can query "show me all traces where the extraction tool was called and the overall request failed."
- **Separate infra observability (latency, error rate, uptime) from behavioral observability (what did the agent decide, why, was it correct)** — both are necessary, neither substitutes for the other.
- **Sample intelligently, not uniformly** — always fully trace failures, escalations, and guardrail triggers (rare events, high value); sample successes at a lower rate to control cost/volume.
- **Build a session replay view** — support engineers should be able to see a stuck/failed session as a readable timeline (prompt → response → tool call → tool result → next prompt), not raw JSON logs.
- **Alert on behavioral metrics, not just infra metrics** — rising escalation rate, rising hallucination rate (from automated checks), or falling eval-proxy scores on live traffic are leading indicators infra metrics miss entirely.

**Design pattern: Hierarchical tracing**

```python
from contextvars import ContextVar
from dataclasses import dataclass, field
from datetime import datetime
import uuid

current_trace: ContextVar[str] = ContextVar("current_trace", default="")

@dataclass
class Span:
    span_id: str
    parent_id: str | None
    trace_id: str
    layer: str  # request | graph | node | agent | tool | llm
    name: str
    start: datetime = field(default_factory=datetime.utcnow)
    end: datetime | None = None
    metadata: dict = field(default_factory=dict)

class Tracer:
    """Minimal hierarchical tracer — swap the backend for
    OpenTelemetry/Honeycomb/Datadog in production."""

    def __init__(self, sink: Callable[[Span], None]):
        self.sink = sink
        self._stack: list[Span] = []

    def start_span(self, layer: str, name: str, **metadata) -> Span:
        trace_id = current_trace.get() or str(uuid.uuid4())
        current_trace.set(trace_id)
        parent = self._stack[-1].span_id if self._stack else None
        span = Span(str(uuid.uuid4()), parent, trace_id, layer, name, metadata=metadata)
        self._stack.append(span)
        return span

    def end_span(self, span: Span, **result_metadata):
        span.end = datetime.utcnow()
        span.metadata.update(result_metadata)
        self._stack.pop()
        self.sink(span)  # ship to logging/tracing backend

# Usage
tracer = Tracer(sink=lambda s: logger.info("span", **s.__dict__))

def call_llm_traced(tracer, prompt, model):
    span = tracer.start_span("llm", model, prompt_preview=prompt[:200])
    response = client.messages.create(model=model, messages=[{"role": "user", "content": prompt}], max_tokens=1024)
    tracer.end_span(span,
        input_tokens=response.usage.input_tokens,
        output_tokens=response.usage.output_tokens,
        cached_tokens=getattr(response.usage, "cache_read_input_tokens", 0),
    )
    return response
```

**Anti-patterns**

- Logging only "agent completed" / "agent failed" with no intermediate span data — completely undebuggable the first time a specific failure mode needs root-causing.
- Truncating or omitting the actual prompt/response content "to save log volume" — this is exactly the data you need most during an incident.
- No shared trace ID across the request → graph → node → agent → tool → LLM hierarchy — every layer's logs become an unjoinable island.
- Uniform sampling that drops most failure traces along with the noise of successes.

---

## 5. Putting It All Together

A production-grade agent request should, in principle, flow through every layer above:

```
User request
   │
   ▼
[Guardrail: input check] ──fail──► reject / sanitize
   │ pass
   ▼
[Context Assembler: budgeted, ranked retrieval]
   │
   ▼
[Model Router: select tier by task complexity]
   │
   ▼
[Deadline-bound agent loop]
   │
   ├─► [Tool call] ──idempotency key──► [Validation] ──► execute ──► [Trace: tool span]
   │        │
   │        └─fail──► [Retry policy] ──exhausted──► [Fallback chain]
   │
   ▼
[Structured output generation] ──validation fail──► [Self-repair loop] ──exhausted──► [Escalation]
   │ pass
   ▼
[Guardrail: output check] ──fail──► [Escalation]
   │ pass
   ▼
[Checkpoint state persisted] ──► response streamed to user
   │
   ▼
[Full trace shipped: request → graph → node → agent → tool → LLM]
   │
   ▼
[Outcome recorded] ──► [Eval dataset / regression suite updated]
```

**The one-sentence version of this whole document:** an agent is not "done" when it produces a correct answer once — it's done when it produces a correct answer reliably, cheaply, observably, and recoverably, under the assumption that every external call it makes will eventually fail.
