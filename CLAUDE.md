# Cloudflare Prometheus Exporter

A Cloudflare Prometheus Exporter built with Effect TypeScript for Cloudflare Workers.

## Project Structure

```
src/
├── index.ts              # Worker entry point, HTTP handlers
├── Config.ts             # Configuration service (env vars)
├── Types.ts              # TypeScript types for Cloudflare API
├── CloudflareClient.ts   # Cloudflare API client service
├── PrometheusRegistry.ts # Prometheus metrics registry service
└── MetricsCollector.ts   # Metrics collection orchestration
```

## Technology Stack

- **Effect** - Type-safe functional programming
- **Cloudflare Workers** - Serverless edge runtime
- **TypeScript** - With strict settings and Effect Language Service

## Key Patterns

### Effect Services
Each module exports a service using `Context.Tag`:
```typescript
export class CloudflareClient extends Context.Tag("CloudflareClient")<
  CloudflareClient,
  { readonly fetchZones: () => Effect.Effect<...> }
>() {}
```

### Layer Composition
Services are composed via layers in `index.ts`:
```typescript
const configLayer = makeConfigFromEnv(env)
const clientLayer = CloudflareClientLive.pipe(Layer.provide(configLayer))
```

### Error Handling
Tagged errors using `Data.TaggedError`:
```typescript
export class CloudflareApiError extends Data.TaggedError("CloudflareApiError")<{
  readonly message: string
}>() {}
```

## Development Commands

```bash
pnpm dev        # Local development server
pnpm typecheck  # TypeScript type checking
pnpm deploy     # Deploy to Cloudflare Workers
```

## Environment Variables

Required:
- `CF_API_TOKEN` - Cloudflare API token

Optional:
- `SCRAPE_DELAY` - Delay before fetching metrics (default: 60)
- `CF_QUERY_LIMIT` - GraphQL query limit (default: 1000)
- `METRICS_DENYLIST` - Comma-separated metrics to exclude
- `CF_ZONES` - Comma-separated zone IDs to include
- `CF_EXCLUDE_ZONES` - Comma-separated zone IDs to exclude

<!-- effect-solutions:start -->
## Effect Solutions Usage

The Effect Solutions CLI provides curated best practices and patterns for Effect TypeScript. Before working on Effect code, check if there's a relevant topic that covers your use case.

- `effect-solutions list` - List all available topics
- `effect-solutions show <slug...>` - Read one or more topics
- `effect-solutions search <term>` - Search topics by keyword

**Local Effect Source:** The Effect repository is cloned to `~/.local/share/effect-solutions/effect` for reference. Use this to explore APIs, find usage examples, and understand implementation details when the documentation isn't enough.
<!-- effect-solutions:end -->
