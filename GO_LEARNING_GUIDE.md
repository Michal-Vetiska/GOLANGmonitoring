# PulseWatch — Go Learning Guide (česky)

Tento dokument je mentorský průvodce pro zkušeného React / Node.js / C# / Python vývojáře, který je v Go nový. Každý koncept odkazuje na konkrétní místo v PulseWatch a obsahuje otázku typickou pro pohovor.

---

## Jak číst tento projekt

1. Začni v [`backend/cmd/server/main.go`](backend/cmd/server/main.go) — start procesu, config, graceful shutdown.
2. Pak [`backend/internal/monitor/engine.go`](backend/internal/monitor/engine.go) — jádro concurrency.
3. Pak [`backend/internal/api/handlers.go`](backend/internal/api/handlers.go) — REST + SSE.
4. Teprve potom storage a SSRF detaily.

V Go je zvyk: **malé balíčky, málo abstrakcí, explicitní chyby, data blízko chování**.

---

## 1. `package` a `import`

**Vysvětlení:** Každý `.go` soubor patří do jednoho `package`. Veřejné identifikátory začínají **velkým písmenem** (`Engine`), privátní malým (`enqueue`). To je visibility model Go — ne `public`/`private`.

```go
package monitor

import (
    "context"
    "log/slog"
)
```

**Srovnání:** V TypeScriptu máš `export` / `import`. V C# `namespace` + `using` + `public`. V Go je export jen capitalizace.

**V projektu:** `internal/` znamená „neexportuj mimo tento modul“ — Go toolchain to vynucuje. Proto je API v `internal/api`, ne v kořenovém balíčku.

**Pohovor:** Proč existuje konvence `internal/` a jak se liší od pouhého neexportování symbolů?

---

## 2. `func`

**Vysvětlení:** Funkce jsou first-class. Metody mají *receiver* před jménem.

```go
func NewEngine(...) *Engine { ... }

func (e *Engine) Run(ctx context.Context) { ... } // metoda na *Engine
```

**Srovnání:** Podobné C# metodám / TS metodám ve třídě, ale Go nemá třídy. Receiver může být hodnota (`e Engine`) nebo pointer (`e *Engine`). Pointer receiver = můžeš měnit stav a vyhneš se kopírování velké structury.

**V projektu:** `(*Engine).worker`, `(*Store).CreateMonitor`, `(*Hub).Publish`.

**Pohovor:** Kdy zvolíš value receiver a kdy pointer receiver?

---

## 3. Proměnné a `:=`

**Vysvětlení:** `:=` deklaruje a přiřadí uvnitř funkce (type inference). `var` použiješ pro zero value nebo package-level.

```go
workers, _ := strconv.Atoi(envOr("PULSEWATCH_WORKERS", "5"))
var lastQueued = map[string]time.Time{}
```

**Srovnání:** Blízko `const workers = ...` / `let` v JS, ale Go je staticky typovaný. `_` zahodí nechtěnou návratovou hodnotu.

**V projektu:** `loadConfig()` v `main.go`, skoro každý handler.

**Pohovor:** Co je „zero value“ u `string`, `int`, `pointer`, `slice`?

---

## 4. Structs vs TypeScript interfaces / C# classes

**Vysvětlení:** `struct` je konkrétní datový tvar v paměti. Nemá dědičnost. Chování přidáváš metodami.

```go
type Monitor struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}
```

Tagy `` `json:"id"` `` řídí serializaci — jako atributy / dekorátory, ale jsou to jen metadata pro reflexi.

**Srovnání:** TS `interface` je jen typový kontrakt (zmizí po kompilaci). C# `class` má identity a dědičnost. Go struct = spíš C# `record` / value object, ale často se předává přes pointer.

**V projektu:** [`backend/internal/models/models.go`](backend/internal/models/models.go).

**Pohovor:** Jak Go dosáhne polymorfismu bez dědičnosti tříd?

---

## 5. Interfaces a implicitní implementace

**Vysvětlení:** Interface se ne„implementuje“ klíčovým slovem. Pokud typ má požadované metody, splňuje interface automaticky (duck typing + compile-time check).

```go
type HTTPChecker interface {
    Check(ctx context.Context, monitorID, rawURL string) models.CheckResult
}
```

`*Checker` i testovací `rawChecker` / `fakeChecker` fungují jako `HTTPChecker` bez deklarace.

**Srovnání:** TS structural typing. C# vyžaduje `class X : IFoo`. Go je structural a obvykle drží interface **u konzumenta**, ne u implementace („accept interfaces, return structs“).

**V projektu:** `HTTPChecker`, `MonitorSource` v `engine.go` — umožňují testy bez oslabení produkčního SSRF.

**Pohovor:** Proč v Go často nedefinuješ interface, dokud ho nepotřebuješ pro test / DI?

---

## 6. Pointers

**Vysvětlení:** `*T` je ukazatel na hodnotu. `&x` bere adresu. Go nemá pointer aritmetiku.

```go
func (s *Store) GetMonitor(...) (*models.Monitor, error)
enabled *bool // v UpdateMonitorRequest — nil = „neposláno“
```

**Srovnání:** C# reference typy + `Nullable<T>`. V JS je všechno reference kromě primitiv. V Go jsou structy defaultně **hodnoty** (kopírují se), proto API často vrací `*Monitor`.

**V projektu:** partial update přes pointer fields; JSON omitempty.

**Pohovor:** Jak odlišíš v JSON API „pole chybí“ vs „pole je explicitně null/false“?

---

## 7. Slices a maps

**Vysvětlení:**

- Slice = dynamické pole (`[]Monitor`) nad polem + length/capacity.
- Map = hash mapa (`map[string]time.Time`).

```go
jobs := make(chan checkJob, workers*4)
lastQueued := make(map[string]time.Time)
```

**Srovnání:** TS `Array` / `Map` / plain object. Slice append může realokovat — pozor na sdílení underlying array.

**V projektu:** fronta jobů, `lastQueued` ve scheduleru, seznam subscriberů v hubu.

**Pohovor:** Co se stane, když concurrent goroutines zapisují do mapy bez mutexu?

---

## 8. Multiple return values

**Vysvětlení:** Funkce běžně vrací `(value, error)`.

```go
m, err := s.store.CreateMonitor(...)
if err != nil { ... }
```

**Srovnání:** Místo výjimek. Blízko Rust `Result`, nebo C# `TryParse`. V JS bys často throw/catch nebo `{ ok, data }`.

**V projektu:** všude ve storage a API.

**Pohovor:** Kdy vracet sentinel error (`ErrNotFound`) a kdy `fmt.Errorf("%w")` wrapping?

---

## 9. Error handling s `error`

**Vysvětlení:** `error` je interface s metodou `Error() string`. Idiom: vrať chybu nahoru, loguj na hranici (HTTP handler / main).

```go
if errors.Is(err, storage.ErrNotFound) {
    writeError(w, http.StatusNotFound, "monitor not found")
}
```

**Srovnání:** Žádný checked exceptions chaos. Explicitní větvení je verbose, ale čitelné v code review.

**V projektu:** handlery mapují domain errory na HTTP statusy.

**Pohovor:** Rozdíl mezi `errors.Is` a `errors.As`?

---

## 10. `defer`

**Vysvětlení:** `defer` naplánuje volání na konec funkce (LIFO). Ideální na `Close()`, `Unlock()`, `cancel()`.

```go
defer resp.Body.Close()
defer cancel()
defer rows.Close()
```

**Srovnání:** C# `using` / `finally`. JS `try/finally`. Go `defer` je lexikálně u resource acquisition — méně zapomenutých close.

**V projektu:** checker, storage rows, SSE unsubscribe, HTTP shutdown timeout.

**Pohovor:** Co vrátí `defer` s named return values a proč na tom záleží?

---

## 11. Goroutines

**Vysvětlení:** `go f()` spustí f souběžně. Jsou levné (tisíce OK). Nejsou threadami 1:1 — runtime je multiplexuje na OS thready (M:N).

```go
go e.worker(ctx, i)
go engine.Run(ctx)
```

**Srovnání:** JS má event loop (1 thread + async). C# `Task.Run` / thread pool. Go goroutine = default jednotka concurrency.

**V projektu:** worker pool, HTTP server, monitoring engine vedle sebe v `main`.

**Pohovor:** Co je „goroutine leak“ a jak ho poznáš?

---

## 12. Channels

**Vysvětlení:** Channel je typovaný pipe mezi goroutines. Buffered channel oddělí producenta od konzumenta.

```go
jobs: make(chan checkJob, workers*4)
e.jobs <- job   // send
job := <-e.jobs // receive
for job := range e.jobs { ... } // dokud se neclose
```

**Srovnání:** Blízko async queue / BlockingCollection. V Node bys měl EventEmitter nebo Bull queue; tady je to v jazyce.

**V projektu:** job queue worker poolu; SSE subscriber buffery.

**Pohovor:** Co se stane při send do closed channel? Co při receive z closed?

---

## 13. `select`

**Vysvětlení:** `select` čeká na více channel operací najednou (jako `Promise.race` + multiplex).

```go
select {
case <-ctx.Done():
    return
case now := <-ticker.C:
    // schedule
case ch <- payload:
default:
    // drop slow subscriber
}
```

`default` = non-blocking větev.

**Srovnání:** JS nemá přímý ekvivalent; C# `Task.WhenAny` je blízko.

**V projektu:** scheduler, SSE loop, non-blocking publish.

**Pohovor:** Jak uděláš timeout na receive z channelu?

---

## 14. `sync.WaitGroup`

**Vysvětlení:** Počítadlo „kolik goroutines ještě běží“. `Add` / `Done` / `Wait`.

```go
e.wg.Add(1)
go e.worker(ctx, i)
// ...
e.wg.Wait()
```

**Srovnání:** `Task.WhenAll`, `Promise.all`, CountDownLatch.

**V projektu:** engine čeká na workery po `close(jobs)`.

**Pohovor:** Proč volat `Add` před `go`, ne uvnitř goroutine?

---

## 15. `sync.Mutex` a race conditions

**Vysvětlení:** Sdílená paměť bez synchronizace = data race. Mutex chrání kritickou sekci. `sync.Map` je specializovaná concurrent mapa.

```go
h.mu.Lock()
h.subs[ch] = struct{}{}
h.mu.Unlock()
```

**Srovnání:** C# `lock`, JS je single-threaded (pokud ne Worker). Go tě nutí myslet na race od začátku — proto `go test -race`.

**V projektu:** Event Hub mutex; `inflight sync.Map` proti duplicitním checkům.

**Pohovor:** Jaký je rozdíl mezi race condition a data race? Proč spouštět `-race` v CI?

---

## 16. `context.Context`

**Vysvětlení:** Context nese deadline/cancellation/request-scoped values napříč API. Zrušení se propaguje.

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
checkCtx, cancel := context.WithTimeout(ctx, DefaultHTTPTimeout+time.Second)
defer cancel()
```

**Srovnání:** `AbortController` v JS/fetch. `CancellationToken` v C#. V Go je Context **první argument** skoro všude.

**V projektu:** graceful shutdown, HTTP request ctx u SSE, timeout u checků.

**Pohovor:** Proč bys do Contextu neměl dávat volitelnou business konfiguraci jako „userID optional bag“ bez rozmyslu?

---

## 17. Go modules

**Vysvětlení:** `go.mod` definuje module path a závislosti. `go.sum` pinuje checksumy.

```go
module github.com/pulsewatch/pulsewatch/backend
```

**Srovnání:** `package.json` + lockfile. Příkazy: `go get`, `go mod tidy`, `go test ./...`.

**V projektu:** jediná externí DB dependency je `modernc.org/sqlite` (+ `google/uuid`). Zbytek stdlib.

**Pohovor:** Co dělá `go mod tidy` a proč commitovat `go.sum`?

---

## 18. HTTP handlers

**Vysvětlení:** Handler je `func(http.ResponseWriter, *http.Request)`. Od Go 1.22 má ServeMux method+path patterns.

```go
mux.HandleFunc("POST /api/monitors/{id}/check", s.handleCheckMonitor)
id := r.PathValue("id")
```

**Srovnání:** Express middleware / ASP.NET controllers. Go stdlib je záměrně nízkoúrovňový — žádný built-in DI container.

**V projektu:** [`handlers.go`](backend/internal/api/handlers.go), CORS + logging middleware.

**Pohovor:** Jak správně nastavit `WriteTimeout` když máš SSE endpointy?

---

## 19. Database access

**Vysvětlení:** `database/sql` je generické API nad driverm. Prepared statements přes `?` placeholdery. SQLite + foreign keys + indexy.

```go
db.ExecContext(ctx, `INSERT INTO monitors (...) VALUES (?, ?, ...)`, ...)
```

**Srovnání:** spíš než ORM (EF/Prisma) je to blízké ručnímu SQL / `pg` v Node. Explicitní scany do structs.

**V projektu:** [`storage/sqlite.go`](backend/internal/storage/sqlite.go), historie limit 500 řádků/monitor.

**Pohovor:** Proč `SetMaxOpenConns(1)` u SQLite v tomto projektu?

---

## 20. Unit tests

**Vysvětlení:** Soubory `*_test.go`, balíček stejný nebo `foo_test` (external test). `httptest` = fake HTTP server v procesu.

```go
srv := httptest.NewServer(http.HandlerFunc(...))
defer srv.Close()
```

**Srovnání:** Jest/Vitest + msw; xUnit + TestServer. Go má testing ve stdlib.

**V projektu:** API CRUD, SSRF reject, timeout, worker pool, cancellation. Produkční dialer se v testech neobaluje „vypnutím SSRF“, ale alternativní `HTTPChecker`.

**Pohovor:** Jak otestuješ graceful cancellation bez flaky sleepů?

---

## 21. Graceful shutdown

**Vysvětlení:** Na signal zruš context → engine přestane schedulovat → close jobs → WaitGroup → `http.Server.Shutdown`.

```go
ctx, stop := signal.NotifyContext(...)
go engine.Run(ctx)
<-ctx.Done()
httpServer.Shutdown(shutdownCtx)
```

**Srovnání:** Node `SIGTERM` handler; Kestrel graceful stop. Důležité v kontejnerech (Kubernetes/Docker stop).

**V projektu:** `main.go` end-to-end.

**Pohovor:** Co se stane s in-flight HTTP checkem při shutdown — a je to OK pro monitoring use-case?

---

# Jak prezentovat PulseWatch na pohovoru

## Celková architektura (30 sekund)

„PulseWatch je monorepo: Go API se SQLite a React dashboard. Uživatel přidá veřejnou URL, scheduler každou sekundu najde due monitory, pošle je do bounded worker poolu přes channel, workery udělají reálný HTTP GET se SSRF ochranou, výsledek uloží a pushnou přes SSE do UI, které patchuje TanStack Query cache.“

## Proč Go na monitoring workers

- Goroutines jsou levné → snadný pool bez těžkého frameworku.
- Channels dávají přirozený back-pressure.
- Stdlib `net/http` + `context` stačí na timeouts a cancel.
- Statická typovost a `go test -race` sedí na concurrent kód.

## Jak spolu hrají goroutines a channels

Scheduler = producer (`jobs <-`). Workery = consumers (`range jobs`). Buffered channel vyhlazuje špičky. Při plném bufferu tick dropneme místo deadlocku.

## Jak je concurrency omezená

1. Fixní počet workerů (default 5).  
2. Buffer job channelu.  
3. `inflight` mapa — jeden check na monitor.  
4. HTTP timeout 5s.  
5. SSE publish s `default` → slow UI neblokuje checky.

## Jak se řeší chyby

Explicitní `error` return values. Validace URL → 400. Not found → 404. Interní chyby logujeme (`slog`) a klientovi dáme obecnou zprávu. Check failure **není** API error — je to validní `CheckResult` s `is_up=false`.

## Jak React mluví s Go

- REST pro CRUD / history / stats (TanStack Query).  
- SSE pro live eventy (`check_result`, `monitor_update`, `monitor_delete`).  
- Vite proxy v dev; v Dockeru nginx proxy na backend.

## Tradeoffs, které umím obhájit

| Rozhodnutí | Proč | Tradeoff |
| --- | --- | --- |
| SQLite | Zero ops, portfolio-friendly | Ne multi-writer horizontálně |
| Žádný Redis | Zbytečný pro single-node | Fan-out jen in-process |
| Redirects off | SSRF | Neověří finální destination za 302 |
| Stdlib mux | Méně deps | Méně „baterií“ než Gin/Echo |
| History cap 500 | Kontrola růstu DB | Starší data zmizí |
| Žádný auth | Scope pohovoru | Nesmí na public internet bez ochrany |

## Co říct upřímně o Go

„V Go jsem nový, ale umím concurrency model vysvětlit z tohoto projektu: goroutines, channels, select, context, race detector. Opírám se o stdlib a idiomy (errors, defer, small interfaces). V JS/TS/React/C#/DevOps mám praxi — Go se učím cíleně přes produkčně inspirovaný kód, ne tutorial hello-world.“

Dobrá follow-up věta: „Kdybych to měl škálovat, oddělil bych checker do worker služby, přidal Postgres a frontu (NATS/SQS) a auth na API.“

---

## Pět nejdůležitějších konceptů před pohovorem

1. **Goroutines + channels + select** (worker pool v `engine.go`)  
2. **`context.Context`** (timeouts + shutdown)  
3. **Interfaces / implicitní implementace** (testovatelný `HTTPChecker`)  
4. **Error handling + `defer`** (idiomatický Go styl)  
5. **Data races / `sync` / `-race`** (Hub mutex, inflight map)

Procvič si nahlas: nakresli na tabuli scheduler → channel → workers → SQLite → SSE → React.
