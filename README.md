# atomic-dvp-go

> Production-ready Go library for atomic Bitcoin settlement using HTLCs, Taproot Assets, and the Lightning Network.

## What this solves

Institutions integrating Bitcoin settlement face a critical gap: LND and tapd have excellent core networking, but there are no abstract, enterprise-grade Go libraries for safely orchestrating trade lifecycles in complex B2B environments.
`atomic-dvp-go` bridges this gap by providing:

- **Atomic DvP (Delivery vs Payment) state management**: Ensures strict settlement guarantees securely across multiple assets without complex custom scripts.
- **Compliance-Event-Triggered Aborts (Atomic Rollback)**: The unique feature bridging Bitcoin to enterprise constraints. When an AML flag hits, the library guarantees an atomic abort—no orphaned trades, and funds automatically return to the sender.
- **Chain-agnostic settlement interfaces**: Abstracting away LND/tapd internals into a unified `SettlementDriver` architecture, allowing orchestration layers to build on solid primitives without tight coupling.
- **Developer Experience First**: Includes `MockSettlementAdapter` enabling developers to test complex state machines without needing a live LND/tapd backing node.

## Why this matters for Bitcoin adoption

To onboard institutions and enterprise applications (such as tokenized RWAs) into the Bitcoin ecosystem, the settlement infrastructure must align with stringent compliance and banking requirements. Without abstract, predictable, and mockable settlement drivers, institutions are forced to reinvent the wheel, increasing integration risks and slowing down adoption.
This library provides the battle-tested, non-custodial piping needed to build complex enterprise workflows (e.g., compliant B2B swaps) on top of Bitcoin and Taproot Assets.

## Core Abstractions

- `settlement.Driver`: The core interface for preparing, executing, and aborting settlements.
- `adapters/lnd`: Wrappers for Lightning Network (HTLC detection, hold invoice settlement, and cancellation).
- `adapters/tapd`: Wrappers for Taproot Assets (Asset transferring and UTXO management).
- `adapters/mock`: Mock implementations of drivers for isolated testing and development.

## Quick Start
Check out the `examples/` directory for full, executable implementation guides:

- [`examples/basic_htlc_swap/main.go`](examples/basic_htlc_swap/main.go): How to initialize the settlement drivers and process a simple atomic exchange safely.
- [`examples/compliance_abort/main.go`](examples/compliance_abort/main.go): How to intercept an active HTLC, trigger a compliance review, and safely execute an **atomic abort** – ensuring no orphaned trades and automatic return of funds to the sender.

## License

This project is licensed under the [MIT License](LICENSE), maximizing adoptability by both OSS and commercial entities without friction.
