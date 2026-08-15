# Expense Tracker

A personal expense tracking system built with a modular architecture, consisting of a Go REST API, web application, and mobile application.

> 🚧 **This project is currently under active development.**

---

## Project Structure

```text
expense-tracker/
├── .ai/
│   ├── docs/
│   │   ├── architecture.md
│   │   └── requirements.md
│   ├── reviews/
│   └── tasks/
│
├── apps/
│   ├── api/          # Go REST API
│   ├── mobile/       # Flutter mobile application
│   └── web/          # Laravel web application
│
├── infra/            # Deployment and infrastructure configuration
│
├── .gitignore
├── LICENSE
└── README.md
```

Not every application exists yet. Directories will be added as development progresses.

---

## Applications

### API

The backend REST API is built with:

- Go
- `net/http`
- SQLite
- `modernc.org/sqlite`

The API is responsible for:

- Expense CRUD operations
- Validation
- Persistence
- HTTP API responses

API-specific documentation and implementation details are available in:

```text
apps/api/README.md
```

### Mobile

Planned mobile application:

- Flutter
- Dart

The mobile application will consume the REST API.

### Web

Planned web application:

- Laravel
- PHP

The web application will consume the REST API.

---

## Architecture

The project follows a simple architecture that favors clarity over unnecessary abstraction.

The API dependency flow is:

```text
main
  │
  ▼
handler
  │
  ▼
store
  │
  ▼
SQLite
```

Models are shared between the handler and store layers:

```text
main
 │
 └── handler ─── model
       │
       ▼
     store ───── model
```

The project intentionally avoids unnecessary frameworks and abstractions.

For the current architectural decisions, see:

```text
.ai/docs/architecture.md
```

---

## API

Current API endpoints:

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/health` | Health check |
| `POST` | `/expenses` | Create an expense |
| `GET` | `/expenses` | List expenses |
| `GET` | `/expenses/{id}` | Get an expense |
| `PUT` | `/expenses/{id}` | Replace an expense |
| `DELETE` | `/expenses/{id}` | Delete an expense |

### Example Expense

```json
{
  "id": "uuid",
  "amount": 4999,
  "description": "Grocery shopping",
  "category": "food",
  "date": "2026-08-15"
}
```

The API represents monetary values using integers rather than floating-point numbers. Client applications are responsible for formatting the amount for display, including currency formatting such as Indonesian Rupiah.

For example:

```text
4999 → Rp4.999
50000 → Rp50.000
1500000 → Rp1.500.000
```

---

## Development

### Requirements

For the API:

- Go 1.23+
- SQLite

### Run the API

```bash
cd apps/api
go run .
```

The API runs locally on:

```text
http://localhost:8080
```

### Run Tests

```bash
cd apps/api
go test ./...
```

### Run Vet

```bash
cd apps/api
go vet ./...
```

### Build

```bash
cd apps/api
go build ./...
```

---

## Testing the API

The API can be tested using tools such as:

- Postman
- `curl`

Example:

```bash
curl http://localhost:8080/health
```

Expected response:

```json
{
  "status": "ok"
}
```

---

## Development Workflow

This project uses an **AI-assisted development workflow**.

The general process is:

```text
Requirements
     │
     ▼
Architecture
     │
     ▼
Architecture Review
     │
     ▼
Task Definition
     │
     ▼
Implementation
     │
     ▼
Automated Tests
     │
     ▼
Implementation Review
     │
     ▼
Fixes
     │
     ▼
Final Review
```

AI agents may be used for different roles, including:

- Architecture
- Implementation
- Code review
- Testing
- Documentation

Project-related AI documentation is stored under:

```text
.ai/
```

---

## 🤖 Built with AI & Author's Learning Journey

> [!NOTE]
> **Disclaimer:** This project was **100% built using AI**, pair-programmed with advanced AI coding agents.

This project is also an experiment in **AI-assisted software development**.

Rather than treating AI as simply a code generator, the project uses AI agents in different roles throughout the development process:

- 🏗️ **Architecture** — designing and evaluating system architecture
- 🔍 **Review** — independently reviewing requirements, architecture, and implementation
- 💻 **Implementation** — writing and modifying the code
- 🧪 **Testing** — creating and running automated tests
- 📝 **Documentation** — maintaining requirements, architecture decisions, tasks, and reviews

The author uses this project as a **learning journey** to understand how software development changes when AI agents become active collaborators in the engineering process.

The goal is not only to build an Expense Tracker, but also to explore:

- How multiple AI agents can collaborate on the same software project
- How human developers can remain responsible for technical decisions
- How AI-generated code can be reviewed and validated
- How requirements and architecture can guide AI implementation
- How an AI-assisted development workflow can remain maintainable and auditable

All AI-generated work is reviewed, tested, and validated as part of the development process.

---

## Documentation

### Requirements

```text
.ai/docs/requirements.md
```

Defines the functional and non-functional requirements for the project.

### Architecture

```text
.ai/docs/architecture.md
```

Defines the system architecture and major technical decisions.

### Reviews

```text
.ai/reviews/
```

Contains architecture and implementation reviews.

### Tasks

```text
.ai/tasks/
```

Contains implementation tasks and their scope.

---

## Design Principles

The project follows a few simple principles.

### Keep It Simple

Avoid adding abstractions, frameworks, or dependencies without a clear benefit.

### Prefer the Standard Library

When the Go standard library provides a suitable solution, prefer it over introducing another dependency.

### Explicit Over Implicit

API behavior, validation rules, and architectural decisions should be documented rather than assumed.

### Test Behavior

Tests should verify behavior from the perspective of the API consumer whenever practical.

### Evolve When Necessary

The architecture should be allowed to grow when the requirements grow. Avoid solving hypothetical problems before they exist.

---

## Roadmap

### Phase 1 — API

- [x] Project requirements
- [x] API architecture
- [x] Architecture review
- [x] Expense CRUD API
- [x] SQLite persistence
- [x] Validation
- [x] Automated tests
- [x] Implementation review

### Phase 2 — Mobile

- [ ] Flutter project
- [ ] API integration
- [ ] Expense list
- [ ] Create expense
- [ ] Edit expense
- [ ] Delete expense
- [ ] Expense categories
- [ ] Dashboard
- [ ] Indonesian Rupiah formatting

### Phase 3 — Web

- [ ] Laravel project
- [ ] API integration
- [ ] Expense management
- [ ] Dashboard
- [ ] Reports
- [ ] Indonesian Rupiah formatting

### Phase 4 — Infrastructure

- [ ] Production configuration
- [ ] Docker/containerization
- [ ] CI/CD
- [ ] Deployment
- [ ] Monitoring

### Phase 5 — Future

Potential future features:

- [ ] Authentication
- [ ] User accounts
- [ ] Multiple currencies
- [ ] Budget management
- [ ] Recurring expenses
- [ ] Reports and analytics
- [ ] Export/import
- [ ] Notifications

---

## License

This project is licensed under the terms specified in [LICENSE](LICENSE).