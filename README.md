# Orion

**orchestration in a single binary. zero bloat.**

![Status](https://img.shields.io/badge/status-gold_master-green)
![Go](https://img.shields.io/badge/go-1.22-blue)
![Vibe](https://img.shields.io/badge/vibe-immaculate-purple)

i wanted to understand how kubernetes *actually* works. so i built one from scratch.

**Orion** is a distributed container orchestrator. it turns a fleet of linux servers into a single, self healing supercomputer.

it's not a wrapper. it's an engine.

---

## the problem
you have two choices today:
1.  **docker-compose:** great for localhost, useless if a server catches fire.
2.  **kubernetes:** massive operational overhead. requires etcd, apiservers, and 3 PhDs to maintain.

i needed something in the middle. something that runs as a **single binary**, uses negligible RAM, and doesn't panic when a node gets overloaded.

## the solution: complexity collapse
i stripped the control plane down to the physics of distributed systems:
* **no external db:** the database is embedded (raft + boltdb).
* **no external load balancer:** service discovery via gossip.
* **symmetric architecture:** every node is the same. leader election handles the rest.

---

## architecture

* **raft:** handles consensus. if it's not in the raft log, it didn't happen. guarantees strong consistency (CP).
* **gossip:** uses `memberlist` (SWIM protocol) + **Lifeguard**. it detects "flapping" nodes (high CPU) and prevents false positives.
* **docker:** direct integration with the docker engine api to spin up containers and dynamic port bindings.

```mermaid
graph TD
    subgraph "the cluster"
        style L fill:#f9f,stroke:#333,stroke-width:2px
        style F1 fill:#e1f5fe,stroke:#333
        style F2 fill:#e1f5fe,stroke:#333
        
        L[node 1: leader] <-->|raft logs (tcp)| F1[node 2: worker]
        L <-->|raft logs (tcp)| F2[node 3: worker]
        
        F1 -.->|gossip (udp)| F2
        F2 -.->|gossip (udp)| L
        L -.->|gossip (udp)| F1
    end

    subgraph "external"
        CLI[user cli] -->|HTTP POST /tasks| L
        Web[web traffic] -->|TCP dynamic port| F2
    end
```

-----

## how to run

prerequisites: `go 1.24+` and `docker`.

### 1\. build it

```bash
go build -o orion ./cmd/orion/
```

### 2\. start the leader (bootstrapping)

spin up the first node. it will elect itself leader.

```bash
./orion --id node1 --port 8000 --gossip-port 6000 --raft-port 7000 --bootstrap
```

### 3\. join a worker (auto-discovery)

spin up a second node. just tell it where the leader is.

```bash
# make sure to use your LAN IP if running across machines
./orion --id node2 --port 8001 --gossip-port 6001 --raft-port 7001 --join 127.0.0.1:6000
```

> *magic moment: watch the leader logs. it detects the join event, automatically adds node 2 to the raft configuration, and starts replicating logs.*

### 4\. deploy a payload

schedule an nginx container. the scheduler will bin-pack it to the least loaded node.

```bash
curl -X POST localhost:8000/tasks -d '{
    "image": "nginx",
    "memory": 100000000,
    "portBindings": {"80/tcp": "8080"}
}'
```

-----

## benchmarks / resilience

| metric | result |
| :--- | :--- |
| **startup latency** | \< 50ms (binary start) |
| **scheduling latency** | \< 10ms (in-memory bin packing) |
| **failover time** | \~2-4s (raft election + gossip convergence) |
| **network traffic** | constant O(1) per node (SWIM gossip) |

**the "kill" test:**
start a task on node 2. `kill -9` node 2. watch node 1 detect the failure via gossip, mark it dead, and reschedule the task to a healthy node automatically.

-----

## roadmap

  * [ ] **TUI:** the CLI works, but i want a `k9s` style dashboard.
  * [ ] **overlay networking:** replacing port mapping with VXLAN for flat pod IPs.
  * [ ] **persistent storage:** CSI driver integration.

-----

*built with \<3 by [@bit2swaz](https://x.com/bit2swaz).*