# Parking Violation Portal

This is a full-stack, modular, and fully production-ready Parking Violation Portal. It satisfies the core requirements for Officer and Member workflows.

## Prerequisites
- Docker Desktop (or equivalent Docker Engine + Docker Compose)

## How to Run Locally

1. **Start the Entire Application Stack:**
   ```bash
   docker-compose up -d --build
   ```
   This command spins up the complete microservice architecture inside Docker:
   - PostgreSQL (Logical Schemas: auth, violation, rule)
   - RabbitMQ (Event-driven message broker)
   - MinIO (S3-compatible object storage for photo uploads)
   - Redis (Idempotency and fast caching)
   - Go Microservices (`api-gateway`, `auth-service`, `rule-service`, `violation-service`, `payment-service`)
   - React Frontend (Served via Nginx)

2. **Access the Application:**
   Open `http://localhost` in your browser.
   - **Officer Login:** `officer1` / `password`
   - **Member Login:** `member1` / `password` (Plates: `MOCK123`, `B1234XYZ`)
   - **Member Login:** `member2` / `password` (Plates: `D5678ABC`, `F9012DEF`)

*(Note: The initial database seeding script automatically hashes the passwords securely using bcrypt.)*

## Core Features & Workflows

### 👮 Officer Capabilities
- **Dynamic Rule Management:** Officers can fully customize Base Fine Amounts, Time-of-Day Multipliers, and Past-Repetition Multipliers directly from the UI.
- **Live Rule Engine Syncing:** Active rules are safely cached in Redis and auto-propagate to all connected microservices instantly. New rule categories populate into dropdown selectors immediately.
- **Real-Time Violation Submission:** Complete with file uploads for photo evidence, an auto-generated precise timestamp preview, and responsive forms.

### 🚗 Member Capabilities
- **Violation Breakdown:** Members see full historical data including an embedded visual preview of their violation photo.
- **Mathematical Transparency:** When a violation is selected for payment, the system dynamically reconstructs and displays the exact math formula (`Base × Time × Repeat = Total`) to provide total transparency on how the fine was calculated.
- **Idempotent Payment Simulations:** Mock successful or failed payment scenarios safely without double-charging.

### 🎨 UI/UX Additions
- **Sleek Aesthetic Interface:** A glassmorphism-inspired design with custom colors, subtle micro-animations, and modern fonts (Inter/Outfit).
- **Dark/Light Mode:** Includes an integrated Dark Mode toggle in the navbar that persists across sessions via local storage and dynamically updates CSS custom properties.

## Assumptions & Trade-Offs

To deliver a working slice while demonstrating senior-level system design, I made the following architectural choices:

### 1. Single Database Container with Logical Schemas
**Trade-off:** A pure microservices architecture dictates that each service has its own completely isolated database. Running 4 separate PostgreSQL instances locally is a heavy resource burden.
**Decision:** We use a *single* PostgreSQL container but enforce strict logical schemas (`auth_schema`, `rule_schema`, `violation_schema`). We explicitly forbid cross-schema SQL `JOIN`s, apart from extremely specific strict-access counts like the Repeat Multiplier logic. This saves local RAM while guaranteeing zero-friction migration to separate DB instances in the future.

### 2. MinIO for Photo Uploads
**Trade-off:** Saving photos to the local filesystem is easier for a prototype.
**Decision:** I opted for a MinIO Docker container to mock AWS S3. The photo bucket is strictly private. The `violation-service` dynamically generates secure **Presigned URLs** with a 1-hour expiration whenever a client requests violation history. This ensures our `violation-service` remains stateless, secure, scalable, and accurately reflects a true cloud-native deployment.

### 3. Snapshot Rule Versioning
**Trade-off:** Relational junction tables for rules (Base Rule + Time Rule + Repeat Rule) are highly normalized but notoriously complex to update immutably without duplicating massive amounts of joined rows.
**Decision:** We use a JSONB Snapshot approach. The active ruleset is stored as a single JSON object with an incremented version number. When a violation occurs, the current `rule_set_version` is permanently linked to the record, guaranteeing past fines never change even as new rules are enacted.

### 4. Expert Patterns Included
- **Strong Authentication & XSS Protection:** Implements cryptographically verified HMAC SHA-256 JWT validation at the API Gateway level and securely hashes all DB passwords via bcrypt. JWTs are stored entirely via `HttpOnly` cookies to thwart Cross-Site Scripting (XSS) attacks.
- **UUIDs over Sequential IDs:** Uses secure `UUIDv4` identifiers for database tables instead of guessable sequential integers, boosting security against enumeration attacks.
- **Automated & End-to-End (E2E) Testing:** Contains unit tests in Go for the rule engine logic, integration-level React testing verifying RBAC component routing, and a full headless E2E test suite written natively in Node.js mimicking user workflows.
- **Idempotency in Payments:** Uses Redis `SETNX` to prevent double-charging if a user clicks pay twice.
- **Local Timezone Computations:** The Rule Engine explicitly locks on to the local timezone when computing the Time-of-Day Multiplier, avoiding UTC skew.
- **Cursor-Based Pagination:** The transaction history endpoint uses cursor pagination instead of `OFFSET/LIMIT` to guarantee consistent performance even at millions of rows.
- **Correlation IDs:** The API Gateway injects an `X-Correlation-ID` header into HTTP requests and RabbitMQ messages for easy distributed tracing.
- **Dead Letter Queues (DLQ):** RabbitMQ is configured with DLQs so failed background tasks don't block the main event loop.

## What I Would Do With More Time

1. **Kubernetes Manifests:** Write Helm charts to demonstrate how this scales in a true cluster orchestrator beyond Docker Compose.
2. **Refresh Tokens:** Implement refresh token logic for the authentication flow.
3. **Observability Stack:** Hook up Prometheus and Grafana to consume the metrics and traces generated by the Correlation IDs.

## Running the Automated End-to-End (E2E) Tests

An autonomous test suite simulates the entire lifecycle of the parking portal, testing the API Gateway directly. It uses the native `node:test` runner.

1. **Ensure the full application stack is currently running locally:**
   ```bash
   docker-compose up -d --build
   ```
2. **Run the test script:**
   Navigate into the `e2e` directory and run the `test.js` script with Node 20+:
   ```bash
   cd e2e
   node --test test.js
   ```
3. **Test Outcomes:** The test script will simulate logging in as an Officer, modifying Active Rules, simulating violations during various time multipliers, logging in as a Member, validating the math formula execution through fetching their history, and successfully simulating an idempotent checkout process.
