# Latency EMA Adaptive Load Shedding Pattern

## 1. Overview & Concept
Adaptive Load Shedding protects backend servers from total collapse by tracking the **Exponential Moving Average (EMA)** of response latencies and shedding incoming requests when response times degrade past an acceptable SLA threshold.

## 2. Production Problem & Failure Modes
1. **Server Death Spiral**: When request concurrency exceeds capacity, CPU queuing and GC pauses cause response latencies to spike from 20ms to 5000ms. Clients time out and retry, adding even more load until all pods crash from OOM or health check timeouts.
2. **Static Limit Inefficiency**: Static request limits don't adapt to changing system conditions (e.g. database degradation, slow network, CPU throttling in Kubernetes).

## 3. Architecture & Mechanism

```text
Incoming Request
       |
       v
Is Latency EMA > Target SLA (e.g. 50ms)?
       |
       +---> YES ---> Shed Request (429 / 503 / ErrShedded)
       |
       +---> NO  ---> Admit Request ---> Measure Latency ---> Update EMA
```

$$\text{EMA}_{\text{new}} = (\text{Sample} \times \alpha) + (\text{EMA}_{\text{old}} \times (1 - \alpha))$$

## 4. Production Hardening & Trade-offs
- **Controlled Rejection vs Total Failure**: Returning fast 503s to 20% of traffic allows the remaining 80% to complete within SLA, preserving overall business operations.
- **Smooth Smoothing Factor**: Choose decay factor $\alpha \approx 0.1$ to prevent single anomalous requests from causing premature load shedding.
- **Fast Recovery**: As soon as shedding relieves pressure and latency drops, the shedder automatically re-admits traffic.

## 5. Code Walkthrough & Usage
See `adaptive_shedder.go` and `adaptive_shedder_test.go`:
- `LatencyEMAShedder`: Manages `targetSLA`, `currentEMA`, and active in-flight tracking.
- `Allow()`: Admits request or returns `ErrShedded`.
