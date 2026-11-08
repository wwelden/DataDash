# DataDash checkpoints

Hidden tests, like a college autograder. This file tells you **what** to build:
the package, the names, the signatures, and the rules. The tests check that
your code follows them. **How** you build it is up to you.

Don't open the `checkpoint_*_test.go` files or `internal/testkit/`. The failure
messages are written to tell you what went wrong without giving the answer away.

## Running checks

```bash
./check.sh list     # every checkpoint
./check.sh 1.1      # one checkpoint
./check.sh 4        # every checkpoint in milestone 4
./check.sh all      # everything (CI runs this at the end)
./check.sh 4.5 -v   # extra flags go to `go test` (-v lists each passing test)
```

Every run uses the race detector (`-race`), so a data race fails a checkpoint
even when the answer looks right.

Before you start a checkpoint, it fails with `undefined: RunPool` (or similar).
That just means you haven't written that name yet.

**About build tags:** each test file starts with a `//go:build` line, so it only
compiles when `check.sh` asks for it. That's why a plain `go build ./...` or
`go vet ./...` ignores the tests, and why your editor may say "No packages found"
if you open one.

**Rules that apply everywhere**

- Module path: `github.com/wwelden/datadash`.
- Names in this file are exact: the tests call them by name.
- "Must not block" means it must return promptly even if something else is stuck.
- Several checks fail if goroutines from your code are still running after
  everything should have stopped (a **goroutine leak**). The message shows
  where each one is stuck.

---

## Milestone 0: Go basics, errors and types

### 0.1 Market data types (`internal/market`)

These are *your* types. The rest of the app uses them instead of Alpaca's, so
changing vendors later only touches one package.

```go
type Quote struct {
    Symbol    string
    BidPrice  float64
    BidSize   int
    AskPrice  float64
    AskSize   int
    Timestamp time.Time
}

type Trade struct {
    Symbol    string
    Price     float64
    Size      int
    Timestamp time.Time
}

func (q Quote) Spread() float64 // ask minus bid
func (q Quote) Mid() float64    // halfway between bid and ask
```

### 0.2 Parsing browser messages (`internal/protocol`)

Browsers send `{"action":"subscribe","symbols":["AAPL","MSFT"]}` (or `"unsubscribe"`).

```go
const ActionSubscribe = "subscribe"
const ActionUnsubscribe = "unsubscribe"

type ClientMessage struct {
    Action  string
    Symbols []string
}

var ErrInvalidJSON, ErrUnknownAction, ErrNoSymbols, ErrInvalidSymbol error  // four distinct errors

func ParseClientMessage(data []byte) (ClientMessage, error)
```

Rules, checked in this order:

1. Not a JSON object with the right shape (including `"symbols"` not being a
   list of strings) → `ErrInvalidJSON`. Unknown extra fields are fine.
2. `action` isn't exactly `subscribe` or `unsubscribe` (case-sensitive, and
   missing counts) → `ErrUnknownAction`.
3. `symbols` missing or empty → `ErrNoSymbols`.
4. Each symbol: trim surrounding whitespace, then uppercase. It must be 1–10
   characters, each `A`–`Z` or `.` (so `BRK.B` is fine, `AAPL1` and `*` are not).
   Otherwise → `ErrInvalidSymbol`, and the error text must **include the bad
   symbol**.
5. Remove duplicates, keeping first-seen order.

The tests use `errors.Is`, so you can return the sentinel itself or wrap it
(`fmt.Errorf("...: %w", ...)`). Wrapping is how you add the symbol to the message.

### 0.3 Encoding server messages (`internal/protocol`)

```go
const StatusConnected = "connected"
const StatusReconnecting = "reconnecting"
const CodeBadRequest = "bad_request"
const CodeSymbolLimit = "symbol_limit"

func EncodeQuote(q market.Quote) ([]byte, error)
func EncodeTrade(t market.Trade) ([]byte, error)
func EncodeStatus(upstream string) ([]byte, error)
func EncodeError(code, message string) ([]byte, error)
```

The JSON must have exactly these keys:

| Function | JSON |
|---|---|
| `EncodeQuote` | `{"type":"quote","symbol":"AAPL","bid":187.1,"bidSize":2,"ask":187.12,"askSize":3,"ts":"2026-11-06T18:30:00.123Z"}` |
| `EncodeTrade` | `{"type":"trade","symbol":"AAPL","price":187.11,"size":100,"ts":"..."}` |
| `EncodeStatus` | `{"type":"status","upstream":"reconnecting"}` |
| `EncodeError` | `{"type":"error","code":"symbol_limit","message":"Too many symbols"}` |

`ts` is RFC 3339 **in UTC** (ends in `Z`), even if the input time isn't UTC.
Use `encoding/json` with struct tags; don't build strings by hand.

---

## Milestone 1: goroutines, channels, select

These are throwaway programs: each folder is `package main` with its own
`func main()` (print whatever you like), so you can also `go run ./playground/<name>`.

### 1.1 Worker pool (`playground/workerpool`)

```go
func RunPool(jobs []int, workers int, work func(int) int) []int
```

- Run `work` on every job using `workers` goroutines; return one result per job, in any order.
- Never more than `workers` jobs running at once, and actually concurrent when `workers > 1`.
- `workers < 1` means 1. No jobs → empty result, no hang.
- When `RunPool` returns, all its goroutines have exited.

### 1.2 Pipeline and fan-in (`playground/pipeline`)

```go
func Generate(nums ...int) <-chan int        // sends nums in order, then closes
func Square(in <-chan int) <-chan int        // squares each value in order; closes when in closes
func Merge(cs ...<-chan int) <-chan int      // fan-in: everything from every input; closes when all inputs close
```

Each must return its channel **immediately** and do its work in a goroutine.

### 1.3 select (`playground/selects`)

```go
func TrySend(ch chan<- int, v int) bool                     // send without blocking; false if it couldn't
func RecvTimeout(ch <-chan int, d time.Duration) (int, bool) // (value, true), or (0, false) on timeout or closed channel
```

A closed channel returns `(0, false)` right away, not after `d`. `TrySend` is
exactly the hub's "drop if the queue is full" move later on.

---

## Milestone 2: races, mutexes, atomics, context (`playground/counter`)

```go
type Counter interface {
    Inc()
    Value() int64
}

func NewUnsafeCounter() *UnsafeCounter // plain int64 field, no protection
func NewMutexCounter() *MutexCounter   // protected by sync.Mutex
func NewAtomicCounter() *AtomicCounter // uses sync/atomic
func StartIncrementers(ctx context.Context, n int, c Counter) (wait func())
```

- **2.1** (`UnsafeCounter`) is **meant to fail** with `WARNING: DATA RACE`.
  Run it, read the report, and see how many increments were lost. It's left out of `./check.sh 2` and `all`.
- **2.2 / 2.3**: the same 100-goroutine hammering must give exactly 100000.
- **2.4** `StartIncrementers` starts `n` goroutines that call `c.Inc()` in a
  loop until `ctx` is cancelled or times out, then returns right away.
  `wait()` blocks until every one of them has exited. (2.4 uses your 2.2 and 2.3 counters.)

---

## Milestone 3: WebSockets

### 3.1 Echo server (`playground/echo`)

Move your current `main.go` to `playground/echo/main.go` (`git mv`).
Keep the handler named `wsEndpoint(w http.ResponseWriter, r *http.Request)`.

- Echo every message back with the same type (text stays text, binary stays binary), in order.
- Many clients at once, under `-race`. Shared settings like the upgrader's
  `CheckOrigin` must be set once, not written on every request.
- A plain non-WebSocket GET gets a 400 and no panic.
- When a client hangs up, the handler returns **and closes its side of the connection**.

### 3.2 Chat room (`playground/chat`)

```go
type Room struct{ ... }
func NewRoom() *Room
func (r *Room) Run(ctx context.Context)                               // the one goroutine that owns the client list
func (r *Room) ServeHTTP(w http.ResponseWriter, req *http.Request)    // makes Room an http.Handler
func (r *Room) ClientCount() int
```

- Every text message from any client goes to **every** connected client, including the sender.
  Don't drop messages for a client that's a bit behind. Here every message must
  arrive (dropping slow clients is the hub's job in milestone 4).
- Each sender's messages arrive in the order sent.
- Clients that leave are removed (`ClientCount` drops).
- Cancelling `ctx` makes `Run` return and closes every client connection, with no leaked goroutines.
- Suggested shape (from the project docs): one reader and one writer goroutine
  per connection. Only the writer writes to the socket, because gorilla allows
  just one concurrent writer.

---

## Milestone 4: the hub (the core of the project)

### 4.1–4.7 Hub (`internal/hub`)

The hub knows nothing about WebSockets. It deals in clients that each have a
queue of already-encoded messages.

```go
type Upstream interface {
    Subscribe(symbols []string) error
    Unsubscribe(symbols []string) error
}

type Config struct {
    Upstream   Upstream
    MaxSymbols int // distinct symbols allowed upstream; 0 means 30
    QueueSize  int // per-client queue (buffered channel) size; 0 means 256
}

type Stats struct {
    Clients       int
    Subscriptions map[string]int // symbol -> number of clients subscribed (no zero entries)
}

var ErrSymbolLimit, ErrClosed error

func New(cfg Config) *Hub
func (h *Hub) Run(ctx context.Context)
func (h *Hub) Register() (*Client, error)
func (h *Hub) Unregister(c *Client)
func (h *Hub) Subscribe(c *Client, symbols []string) error
func (h *Hub) Unsubscribe(c *Client, symbols []string) error
func (h *Hub) PublishQuote(q market.Quote)
func (h *Hub) PublishTrade(t market.Trade)
func (h *Hub) SetUpstreamStatus(status string)
func (h *Hub) Stats() Stats
func (c *Client) Messages() <-chan []byte
```

**4.1 Lifecycle.** You need `New`, `Run`, `Register`, `Unregister`, `Stats`,
`Messages` and `ErrClosed` (the other methods can wait).
- `Run` is the single goroutine that owns all hub state. The other methods
  send it requests over channels and, where they return something, wait for the answer.
- `Unregister` closes that client's `Messages` channel. Unregistering twice is safe.
- Cancelling `ctx`: `Run` returns and every client's channel is closed.
- After `Run` has returned, calls must not block. `Register` returns
  `ErrClosed`, `Stats` returns the zero `Stats`, and `Unregister`, `Publish*`
  and `SetUpstreamStatus` do nothing.

**4.2 Subscriptions and fan-out.**
- A client only gets quotes and trades for symbols it subscribed to, encoded
  with `protocol.EncodeQuote`/`EncodeTrade`, in publish order.
- Subscribing twice to the same symbol counts once. Unsubscribing from
  something you never subscribed to is fine (returns nil).
- `Subscribe` returns only after the hub has applied it, so a publish right
  after `Subscribe` returns is delivered.
- `Stats` returns a **copy** of the map.
- `Subscribe` and `Unsubscribe` for an unregistered or evicted client return an
  error (one you define, such as `ErrUnknownClient`) and change nothing.

**4.3 Reference counting.** The first client to want a symbol causes
`Upstream.Subscribe`; the last one to leave (unsubscribe, unregister or
eviction) causes `Upstream.Unsubscribe`. Upstream calls:
- never overlap (Alpaca's SDK forbids concurrent changes), so one goroutine makes them, in order;
- never subscribe a symbol that's already subscribed upstream, or unsubscribe one that isn't;
- may run in `Run` itself or in a dedicated goroutine. Tests wait for them to happen rather than assuming they're instant.

**4.4 Symbol cap.** If a `Subscribe` would push the number of distinct
symbols over `MaxSymbols`, return `ErrSymbolLimit` and change **nothing**,
not even the symbols in the request that would have fit. Symbols that are
already subscribed by anyone don't count as new.

**4.5 Slow clients.** The hub never blocks on a client. If a client's queue
is full, the hub **evicts** it: removes it (and its subscriptions, releasing
upstream symbols), then closes its channel. Publishing must never stall
because one client isn't reading.

**4.6 Status.** `SetUpstreamStatus(s)` sends `protocol.EncodeStatus(s)` to every client.

**4.7 Load.** 100 clients, 10 symbols, 1000 quotes. Everyone gets exactly
their symbols, in order. It logs your throughput, which is a number for the README.

### 4.8 Fake feed (`internal/fakefeed`)

Fake market data so you can develop offline and outside market hours.

```go
type Publisher interface {
    PublishQuote(market.Quote)
    PublishTrade(market.Trade)
}

func New(interval time.Duration) *Feed
func (f *Feed) Subscribe(symbols []string) error   // makes *Feed a hub.Upstream
func (f *Feed) Unsubscribe(symbols []string) error
func (f *Feed) Run(ctx context.Context, pub Publisher)
```

- Every `interval`, for each subscribed symbol, publish one quote and one trade.
- Prices are a random walk: they start somewhere sensible, move a little each
  tick (under 5%), and change at least sometimes. `AskPrice >= BidPrice > 0`,
  sizes `> 0`, `Timestamp` is now.
- `Subscribe` and `Unsubscribe` are called from other goroutines while `Run` is ticking.
- `Run` returns when `ctx` is done.

Why does `Run` take the publisher instead of `New`? The hub needs its upstream
when it's created, and the feed needs the hub to publish to. Passing the
publisher later breaks that loop.

### 4.9 Browser sessions (`internal/client`)

```go
type Config struct {
    PingInterval time.Duration // 0 means a sensible default
    PongWait     time.Duration
    WriteWait    time.Duration
}

func NewHandler(h *hub.Hub, cfg Config) http.Handler
```

Each WebSocket connection:
- registers with the hub, and unregisters when the browser leaves;
- parses each message with `protocol.ParseClientMessage` and calls `Subscribe`/`Unsubscribe`;
- answers a bad message with an error message (`code` `bad_request`), and
  `ErrSymbolLimit` with code `symbol_limit`, then stays open;
- forwards everything from `client.Messages()` to the browser, and closes the
  WebSocket when that channel is closed (eviction or shutdown);
- uses two goroutines (reader and writer), and **only the writer writes** to
  the socket. Think about how the reader gets an error message to the writer.

`Config` is only exercised in 6.1, but define it now.

---

## Milestone 5: Alpaca (`internal/upstream`)

Add the SDK first: `go get github.com/alpacahq/alpaca-trade-api-go/v3@latest`.

### 5.1 Backoff

```go
func Backoff(attempt int, base, max time.Duration, jitter float64) time.Duration
```

`d = base × 2^attempt`, capped at `max` (watch out for overflow on huge
attempts; negative attempts count as 0). Return `d/2 + jitter × d/2`, where
`jitter` is in [0, 1]. The caller passes `rand.Float64()`; taking it as a
parameter makes the function testable.

### 5.2 Mapping Alpaca's types to yours

```go
func QuoteFromAlpaca(q stream.Quote) market.Quote  // stream = .../v3/marketdata/stream
func TradeFromAlpaca(t stream.Trade) market.Trade
```

Copy the fields you have, convert sizes to `int`, and convert timestamps to UTC.

### 5.3 Live client

```go
type Publisher interface {
    PublishQuote(market.Quote)
    PublishTrade(market.Trade)
    SetUpstreamStatus(string)
}

type Config struct {
    KeyID, SecretKey string
    Feed             string // "test" (always on, symbol FAKEPACA) or "iex"
}

func New(cfg Config) *Client
func (c *Client) Connect(ctx context.Context, pub Publisher) error // blocks until connected or failed
func (c *Client) Subscribe(symbols []string) error                 // makes *Client a hub.Upstream
func (c *Client) Unsubscribe(symbols []string) error
```

- `Subscribe` before `Connect` returns an error.
- Once connected, call `pub.SetUpstreamStatus("connected")`, and `"reconnecting"` when the SDK is reconnecting.
- Bad keys make `Connect` return an error quickly, with no endless retry.
- The live data test only runs when your keys are in `.env` (check.sh loads it).
  Without keys it's skipped. The bad-keys test needs the internet but not your keys.

---

## Milestone 6: hardening

### 6.1 Heartbeats and deadlines (`internal/client`)

- The server pings every `PingInterval`. Each pong (or message) pushes the
  read deadline out to `PongWait`. A client that goes silent is dropped
  shortly after `PongWait`.
- Set a write deadline of `WriteWait` before every write. A client whose
  socket stops accepting data is dropped, and **neither of its goroutines
  stays stuck**.
- Zero values mean defaults that don't drop healthy clients.

### 6.2 The server (`internal/server`)

```go
type Config struct {
    Feed         string        // "fake" or "alpaca"
    FakeInterval time.Duration // for the fake feed; 0 means a default
    MaxSymbols   int
    QueueSize    int
    Alpaca       upstream.Config
    Client       client.Config
}

func Run(ctx context.Context, ln net.Listener, cfg Config) error
```

- Wires everything together: hub, feed (fake or Alpaca) and HTTP server on `ln`.
- `/ws` is the client handler. `/healthz` returns 200 with JSON
  `{"status":"ok","clients":N}`.
- Cancelling `ctx` shuts down cleanly: every browser connection closed, HTTP
  server stopped, all goroutines exited, and `Run` returns `nil`.
- `cmd/server/main.go` (not tested) makes the listener, reads config and keys
  from the environment, cancels on Ctrl-C (`signal.NotifyContext`), and calls
  `server.Run`. Add `log/slog` logging here and in the server.

---

## Milestone 7: measure and ship

### 7.1 Percentiles (`internal/stats`)

```go
func Percentile(samples []time.Duration, p float64) time.Duration
type Summary struct {
    Count               int
    Min, Max, Mean      time.Duration
    P50, P90, P99       time.Duration
}
func Summarize(samples []time.Duration) Summary
```

Use the nearest-rank method: sort, then take the value at rank
⌈p/100 × n⌉ (counting from 1, at least 1). Clamp `p` to [0, 100], return 0 for
no samples, and **don't reorder the caller's slice**. Then use these in
`cmd/loadtest` (not tested): open N clients, measure delivery latency, print a `Summary`.

### 7.2 Dashboard page (`internal/server`)

`GET /` serves an HTML page that has a
`<table>`, says the data is from **IEX**, and connects to `/ws`. Other
paths return 404.

Put the page in `web/index.html` at the repo root, and make `web/` its own
small package (`package web`) that embeds it with `//go:embed` and exports
the result. `//go:embed` can't reach `../` directories, so `internal/server`
imports the `web` package instead of embedding the file itself.

### 7.3 Ready to ship (`internal/repocheck`)

- `Dockerfile` with a multi-stage build and no secrets in it.
- A GitHub Actions workflow in `.github/workflows/` that runs `go vet`, `go test`, `-race` and `-tags all`.
- `README.md` with architecture, a benchmark table (including p99) and a design decisions section.
- No Alpaca key or secret anywhere in git history, and `.env` not tracked.

Deploying (Railway or Fly.io) isn't checked automatically; the live link is your proof.
