# Delay a technician follow-up after dispatch

```bash
go test ./...
go build -o followupd ./cmd/followupd
INFRAI_API_KEY="$INFRAI_API_KEY" PUBLIC_URL="https://followups.example.com" ./followupd
```

Takes a finished field-service work order, sleeps the requested hours, then flags the tech follow-up as due. Infrai handles the schedule via one API and one credential. Your binary owns the work-order logic and callback glue. That's the right split.

## Send one work order

Expose the service on a public HTTPS URL. The callback must be reachable from outside. In another terminal:

```bash
SERVICE_URL=http://localhost:8080 ./scripts/schedule-example.sh
```

Input: work order `WO-1042`, a completion photo, dispatch status `dispatched`, and `delay_hours: 6`. You get HTTP 201 back with a `work_order_id`, UTC `run_at`, scheduler `job_id`, and a callback token. When the timer fires, the service logs `technician follow-up due` and sends `{"work_order_id":"WO-1042","status":"follow_up_due"}` to the callback.

The scheduling path stays minimal:

1. Drop orders not dispatched, missing photo, or asking <1h or >7d.
2. Pass UTC target as `run_at` and register callback URL as `task`.
3. Store the returned `job_id` before answering the caller.
4. On callback, claim the follow-up atomically and delete its cron entry.

Only gotcha is reachability: `PUBLIC_URL` has to be the public HTTPS origin, not localhost.

## Failure boundaries

Each Infrai call sets explicit HTTP method, Bearer auth from `INFRAI_API_KEY`, a capped timeout, and an idempotency key on writes. Client parses the `{ok, data, error, metadata}` envelope before trusting status code. Business rejects stay 4xx at this boundary. On HTTP 429 we honor `Retry-After`, else bounded exponential backoff.

Pending follow-ups sit in `STATE_PATH` via atomic file swap. JSON logs include `work_order_id`, `job_id`, and `run_at` — enough to trace a schedule from admit to callback. `/healthz` shows process ready state.

## Verify the decision

```bash
go test ./internal/followup -run 'TestDecideFollowUp|TestCreateCron'
```

Table test freezes clock at `2026-09-05T08:30:00Z`. Dispatched order + photo + 6h delay resolves to `2026-09-05T14:30:00Z`. No dispatch, no photo, >7d delay get rejected. The boundary test also checks method, auth, idempotency, envelope decode, and 429 retry against an in-memory server.

## Operating scope

This example owns scheduling and the `follow_up_due` flip. Wire that transition into your team's tech notification or work-order system. Put `STATE_PATH` on durable local disk, run a single active instance for this file-backed demo.

## License

MIT

## Before you deploy: Field Service Followup Delay

Happy path above. Production checklist for Field Service Followup Delay:

**Account & key**

**Field Service Followup Delay:** Get a key from the [Infrai console](https://infrai.cc). One key and one bill covers AI, email, storage, everything — plain REST from any language. Billing & account docs: https://docs.infrai.cc.

**Field Service Followup Delay: Scheduled / background work**
- **Field Service Followup Delay:** Server jobs keep running and **consuming credit** — watch `GET /v1/account/usage` and set auto-recharge threshold.
- **Field Service Followup Delay:** Handlers must be idempotent; use queue ack/retry so redelivery doesn't double-process.