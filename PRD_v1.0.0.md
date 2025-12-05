# Product Requirements Document: Orion (v1.0.0)

## Meta Data

| Field | Value |
| :--- | :--- |
| **Project** | **Orion** (Distributed Container Orchestrator) |
| **Version** | **v1.0.0** (The "Gold Master" Release) |
| **Owner** | Aditya Mishra (`@bit2swaz`) |
| **Status** | **COMPLETED / STABLE** |
| **Type** | Distributed Systems / Infrastructure |
| **Core Stack** | Go, HashiCorp Raft, Memberlist (SWIM/Lifeguard), Docker SDK, BoltDB |

-----

## 1\. Executive Summary

### 1.1 The Vision

**Orion** is a lightweight, distributed container orchestrator designed to demonstrate the core principles of reliability engineering. It turns a chaotic fleet of commodity servers into a single, self-healing logical computer.

### 1.2 Core Value Proposition

  * **Complexity Collapse:** A complete cluster in a single binary. No external dependencies (etcd, Zookeeper) required.
  * **Resilience:** Survives node failures, network partitions, and high-CPU "flapping" scenarios.
  * **Efficiency:** Uses constraint-aware scheduling (Bin Packing) to optimize resource usage across the fleet.

### 1.3 The "Resume" Objective

Demonstrate mastery of:

1.  **Consensus:** Implementing Raft for strong consistency state replication.
2.  **Gossip:** Using SWIM with Lifeguard extensions for robust failure detection.
3.  **Orchestration:** Managing the container lifecycle via the Docker Engine API.

-----

## 2\. System Architecture

Orion operates on a symmetric architecture where every node runs the same binary but assumes different roles (Leader vs. Follower) based on the consensus state.

### 2.1 Cluster Topology

```mermaid
graph TD
    subgraph "Orion Cluster"
        style L fill:#f9f,stroke:#333,stroke-width:2px
        style F1 fill:#e1f5fe,stroke:#333
        style F2 fill:#e1f5fe,stroke:#333
        
        L[Node 1: Leader] <-->|Raft Logs (TCP)| F1[Node 2: Worker]
        L <-->|Raft Logs (TCP)| F2[Node 3: Worker]
        
        F1 -.->|Gossip (UDP)| F2
        F2 -.->|Gossip (UDP)| L
        L -.->|Gossip (UDP)| F1
    end

    subgraph "External"
        CLI[User CLI] -->|HTTP POST /tasks| L
        Web[Web Traffic] -->|TCP :8080| F2
    end
```

### 2.2 The "Brain, Nerves, & Muscle" Design

Orion is architected as a set of decoupled subsystems communicating via Go channels and interfaces.

  * **The Brain (Store/Raft):** The single source of truth. Uses a Replicated Log to ensure all nodes agree on the "Desired State."
  * **The Nerves (Cluster/Memberlist):** The sensory system. Uses SWIM gossip protocol to detect node health and broadcast capacity metrics (RAM/CPU). Implements **Lifeguard** to prevent false positives during high load.
  * **The Judge (Scheduler):** The decision engine. Uses a "Filter & Score" algorithm to place tasks on the optimal node.
  * **The Heart (Manager/Reconciler):** The control loop. Periodically diffs "Desired State" vs. "Actual State" and applies corrections.
  * **The Muscle (Worker/Docker):** The actuator. Interfaces with the Docker Daemon to pull images, start containers, and manage networking.

-----

## 3\. Detailed Functional Specifications

### 3.1 Distributed Consensus (The "Brain")

  * **Implementation:** `hashicorp/raft` backed by `boltdb`.
  * **Mechanism:** Leader Election + Log Replication.
  * **Dynamic Membership:** Nodes joining via Gossip are automatically promoted to Raft Voters by the Leader.
  * **Consistency:** Strong Consistency (CP system). Writes fail if a quorum is lost.

### 3.2 Failure Detection (The "Nerves")

  * **Implementation:** `hashicorp/memberlist` (SWIM Protocol).
  * **Lifeguard Enhancements:**
      * **LHA-Probe:** Dynamic probe timeouts based on local health multipliers.
      * **Buddy System:** Prioritizes suspect messages to target nodes for rapid refutation.
      * **Result:** 50x reduction in false positives compared to standard SWIM.
  * **Metadata:** Piggybacks JSON payloads (Role, MemoryTotal, MemoryUsed, RaftPort) on heartbeat packets.

### 3.3 Scheduling Logic (The "Judge")

  * **Algorithm:** Constraint-Aware Bin Packing.
  * **Phase 1 (Filter):** Eliminates nodes with insufficient RAM/Disk or mismatched Tags.
  * **Phase 2 (Score):** Ranks candidates by `FreeMemory`.
  * **Phase 3 (Assign):** Selects the highest-scoring node and persists the assignment to Raft.

### 3.4 Orchestration Loop (The "Heart")

  * **Mechanism:** Level-Triggered Reconciliation Loop (runs every 5s).
  * **Logic:**
    1.  Check Leadership (Split-Brain Protection).
    2.  Retrieve Task List from Raft FSM.
    3.  If `Pending`: Run Scheduler -\> Assign Node -\> Update Raft.
    4.  If `Scheduled` on `Self`: Call Docker Worker -\> Start Container -\> Update Raft to `Running`.

-----

## 4\. Data Structures

### 4.1 The Task (Unit of Work)

```go
type Task struct {
    ID            uuid.UUID
    Name          string
    Image         string
    State         TaskState     // Pending, Scheduled, Running, Failed
    Memory        int64         // Bytes
    Cpu           float64       // Cores
    NodeSelectors map[string]string
    PortBindings  map[string]string // "80/tcp" -> "8080"
    NodeID        string        // The assigned worker
}
```

### 4.2 The Node Metadata (Gossip Payload)

```go
type NodeMeta struct {
    ID          string
    Role        string
    RaftPort    int     // Critical for auto-joining Raft
    MemoryTotal int64
    MemoryUsed  int64
    CpuTotal    float64
}
```

-----

## 5\. API Reference

The system exposes a REST API on the Leader node.

| Method | Endpoint | Description | Payload Example |
| :--- | :--- | :--- | :--- |
| `POST` | `/tasks` | Submit a new task. | `{"image":"nginx", "memory":1000, "portBindings":{"80/tcp":"8080"}}` |
| `GET` | `/tasks` | List all tasks. | - |
| `GET` | `/nodes` | List cluster members. | - |
| `GET` | `/raft` | Debug Raft state. | - |

-----

## 6\. Validated Scenarios (The "It Works" Section)

### Scenario A: Cluster Bootstrap

  * **Action:** Start Node 1 (`--bootstrap`). Start Node 2 (`--join Node1`).
  * **Result:** Node 2 joins Gossip. Node 1 detects join event. Node 1 adds Node 2 to Raft configuration. Logs replicate. **PASS.**

### Scenario B: Task Scheduling

  * **Action:** `POST /tasks` with Nginx.
  * **Result:** Leader writes to Raft. Scheduler picks Node 2. Node 2 receives Raft log. Reconciler on Node 2 sees assignment. Node 2 pulls image. Nginx starts. **PASS.**

### Scenario C: Resource Constraints

  * **Action:** `POST /tasks` with 100GB RAM requirement on a 16GB cluster.
  * **Result:** Scheduler returns `nil` candidate. Task remains `Pending`. Cluster does not crash. **PASS.**

-----

## 7\. Future Roadmap (v0.2.0 Candidates)

  * **Overlay Networking:** Replace Port Mapping with VXLAN/WireGuard for flat pod IPs.
  * **Persistent Storage:** CSI (Container Storage Interface) driver for volume mounting.
  * **Rolling Updates:** Orchestration logic for zero-downtime deployments.

-----