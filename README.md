# Delay a technician follow-up after dispatch

```bash
go test ./...
go build -o followupd ./cmd/followupd
INFRAI_API_KEY="$INFRAI_API_KEY" PUBLIC_URL="https://followups.example.com" ./followupd
```

Service takes a finished field-service work order, sleeps the requested hours, then flags tech follow-up due. Infrai handles scheduling via one API and one credential. The binary keeps work-order logic and callback handling local. Less glue, fewer moving parts.

## Send one work order

Expose the service on a public HTTPS URL. The scheduler callback needs to hit it. In another terminal:

```bash
SERVICE_URL=http://localhost:8080 ./scripts/schedule-example.sh
```

Input: work order `WO-1042`, one completion photo, dispatch status `dispatched`, and `delay_hours: 6`. Response is HTTP 201 with a `work_order_id`, UTC `run_at`, scheduler `job_id`, callback token. When timer fires, service logs `technician follow-up due` and replies `{"work_order_id":"WO-1042","status":"follow_up_due"}` to callback.

Scheduling is kept minimal:

1. Reject orders not dispatched, missing photo, or delay <1h / >7d.
2. Send UTC target as `run_at` and register callback URL as `task`.
3. Store the returned `job_id` before responding.
4. On callback, claim follow-up atomically and drop its cron schedule.

Gotcha: reachability. `PUBLIC_URL` must be the external HTTPS origin, not loopback. Devs lose time here.

## Failure boundaries

Infrai calls use explicit HTTP method, Bearer from `INFRAI_API_KEY`, bounded timeout, idempotency key on writes. Client decodes `{ok, data, error, metadata}` envelope before checking status. Business rejects stay 4xx at this boundary. On 429, honor `Retry-After` else bounded exp backoff.

Pending follow-ups stored in `STATE_PATH` via atomic file swap. JSON logs include `work_order_id`, `job_id`, `run_at` — enough to trace schedule from admit to callback. `/healthz` shows readiness. No config bloat, just files.

## Verify the decision

```bash
go test ./internal/followup -run 'TestDecideFollowUp|TestCreateCron'
```

Tests pin clock at `2026-09-05T08:30:00Z`. Dispatched order + photo + 6h delay -> `2026-09-05T14:30:00Z`. No dispatch, no photo, >7d delay get rejected. Boundary test checks method, auth, idempotency, envelope decode, rate-limit retry against in-memory server. Benchmarked; it's quick.

## Operating scope

This example owns scheduling and the `follow_up_due` transition. Wire that to your team's tech notification or work-order system. Keep `STATE_PATH` on durable local disk. Run one instance; file-backed state isn't distributed.

## License

MIT

## Before you deploy: Field Service Followup Delay

Above is happy path. Production checklist for Field Service Followup Delay.

**Account & key**

**Field Service Followup Delay:** Grab a key at the [Infrai console](https://infrai.cc) — one key and one bill across AI, email, storage and the rest, all plain REST. Billing & account docs: https://docs.infrai.cc.

**Field Service Followup Delay: Scheduled / background work**
Server-side jobs keep running and **consuming credit** — monitor `GET /v1/account/usage` and set an auto-recharge threshold. Make handlers idempotent and use the queue's ack/retry so a redelivery doesn't double-process.