# Multi-Workload Isolated Bulkhead Pattern

## 1. Overview & Concept
Inspired by watertight compartments in ship hulls, the Multi-Workload Bulkhead pattern partitions application concurrency and worker capacity into dedicated, isolated pools (e.g. 10 workers for `Payment`, 5 for `Email`, 5 for `KYC`).

## 2. Production Problem & Failure Modes
Without workload isolation in a shared worker pool:
1. **Resource Starvation**: A sudden burst of 50,000 marketing email tasks consumes all worker goroutines, completely blocking critical high-priority payment processing jobs.
2. **Cascading Failure**: A third-party verification partner (KYC) experiences high latency, tying up all shared worker threads and bringing down unrelated core checkout features.

## 3. Architecture & Mechanism

```text
Incoming Jobs
      |
      +---> Payment Job ---> [ Payment Queue (10 Workers) ] ---> Dedicated Pool
      |
      +---> Email Job   ---> [ Email Queue   (5 Workers)  ] ---> Dedicated Pool
      |
      +---> KYC Job     ---> [ KYC Queue     (5 Workers)  ] ---> Dedicated Pool
```

## 4. Production Hardening & Trade-offs
- **Dedicated Sizing**: Allocate capacity according to business criticality and SLA requirements rather than equal division.
- **Fail-Fast Rejection**: If a specific workload queue is saturated, reject additional tasks (`ErrWorkloadQueueFull`) immediately without impacting other partitions.
- **Observability**: Expose queue depth and worker utilization metrics per partition to detect bottlenecked domains.

## 5. Code Walkthrough & Usage
See `multi_bulkhead.go` and `multi_bulkhead_test.go`:
- `MultiWorkloadBulkhead`: Manages isolated queues and dedicated goroutine pools per `WorkloadType`.
- `Dispatch(wType, job)`: Submits tasks to the designated workload queue.
