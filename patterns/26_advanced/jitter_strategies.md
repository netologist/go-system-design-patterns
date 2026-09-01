# Advanced Jitter Strategies for Backoff (Full, Equal, Decorrelated)

## 1. Overview & Concept
When distributed clients encounter transient failures, they must back off before retrying. Without **jitter** (randomization), all instances retry simultaneously at synchronized intervals ($2s, 4s, 8s$), creating periodic waves of load known as the **Thundering Herd / Retry Storm**.

This pattern implements three standard AWS-recommended jitter algorithms:
1. **Full Jitter**: $\text{sleep} = \text{rand}(0, \min(\text{cap}, \text{base} \times 2^{\text{attempt}}))$
2. **Equal Jitter**: $\text{half} = \text{cap} / 2;\ \text{sleep} = \text{half} + \text{rand}(0, \text{half})$
3. **Decorrelated Jitter**: $\text{sleep} = \min(\text{cap}, \text{rand}(\text{base}, \text{prevSleep} \times 3))$

## 2. Production Problem & Failure Modes
1. **Synchronized Retry Spikes**: 10,000 clients fail at $T=0$. Without jitter, all 10,000 retry at $T=2s$, and all 10,000 retry again at $T=4s$, guaranteeing downstream failure.
2. **Slow Downstream Recovery**: Downstream databases or microservices recovering from a restart are overwhelmed by coordinated retry pulses.

## 3. Architecture & Mechanism

```text
Without Jitter (Synchronized Waves):
Load |   ||   ||   ||   ||
     +-------------------------> Time

With Full / Decorrelated Jitter (Smooth Continuous Distribution):
Load |   ......................
     +-------------------------> Time
```

## 4. Production Hardening & Trade-offs
- **Full Jitter**: Lowest total work done by clients; best general-purpose default.
- **Equal Jitter**: Guarantees a minimum backoff duration while providing randomization.
- **Decorrelated Jitter**: Does not require attempt counts; bases sleep duration on previous sleep time, providing optimal spread under high concurrency.

## 5. Code Walkthrough & Usage
See `jitter_strategies.go` and `jitter_strategies_test.go`:
- `FullJitter(attempt, base, max)`
- `EqualJitter(attempt, base, max)`
- `DecorrelatedJitter(prevSleep, base, max)`
