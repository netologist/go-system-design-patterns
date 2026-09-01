# Production Backend Engineering Patterns — Go

> Production-grade backend servislerinde kullanılan pattern, principle ve “ince ayar” kataloğu.
>
> Bu doküman özellikle Go backend servisleri düşünülerek hazırlanmıştır. Her başlık altında pattern'in **ne olduğu**, **hangi problemi çözdüğü** ve gerektiğinde kısa örnekler bulunur.

---

## İçindekiler

1. [Process & Lifecycle](#1-process--lifecycle)
2. [Configuration](#2-configuration)
3. [Dependency Injection](#3-dependency-injection)
4. [Repository & Persistence](#4-repository--persistence)
5. [Context](#5-context)
6. [Error Handling](#6-error-handling)
7. [HTTP Server Hardening](#7-http-server-hardening)
8. [HTTP Client & Downstream](#8-http-client--downstream)
9. [Concurrency](#9-concurrency)
10. [API Design](#10-api-design)
11. [Middleware](#11-middleware)
12. [Authentication & Authorization](#12-authentication--authorization)
13. [Database & Transaction İncelikleri](#13-database--transaction-İncelikleri)
14. [Caching](#14-caching)
15. [Messaging & Queue](#15-messaging--queue)
16. [Distributed Systems](#16-distributed-systems)
17. [Observability](#17-observability)
18. [Testing](#18-testing)
19. [Security Hardening](#19-security-hardening)
20. [Reliability Patterns](#20-reliability-patterns)
21. [Deployment & Kubernetes](#21-deployment--kubernetes)
22. [Resource Management](#22-resource-management)
23. [Data Integrity](#23-data-integrity)
24. [Background Jobs](#24-background-jobs)
25. [Code Organization](#25-code-organization)
26. [İleri Seviye İnce Ayarlar](#26-ileri-seviye-ince-ayarlar)
27. [Büyük Resim: Pattern'lerin Ortak Prensipleri](#27-büyük-resim-patternlerin-ortak-prensipleri)
28. [Özet: Hangi Pattern Hangi Problemi Çözer?](#28-özet-hangi-pattern-hangi-problemi-çözer)

---

# 1. Process & Lifecycle

Bir servisin sadece request işlerken değil, **başlarken, sinyal alırken ve kapanırken** nasıl davranacağını belirleyen pattern'lerdir.

| Pattern | Ne? | Neyi çözer? |
|---|---|---|
| **Graceful Shutdown** | `SIGTERM`/`SIGINT` sonrası server'ı kontrollü kapatma | Deploy/restart sırasında aktif request'lerin yarım kalmasını ve veri kaybını azaltır |
| **Graceful Startup** | Servisin dependency'leri ve kritik kaynakları kontrollü biçimde hazırlaması | Servisin daha hazır olmadan trafik almasını önler |
| **Fail-Fast Config** | Config'i startup'ta parse/validate etmek | Eksik veya yanlış env/config'i saatler sonra değil ilk saniyede yakalar |
| **Dependency Validation** | DB/Redis/Kafka gibi kritik dependency'lerin başlangıçta doğrulanması | Bozuk deployment'ın “çalışıyor gibi” görünmesini önler |
| **Readiness Probe** | Trafik almaya hazır olup olmadığını bildiren health endpoint/probe | K8s'in hazır olmayan pod'a trafik göndermesini önler |
| **Liveness Probe** | Process'in sağlıklı şekilde çalışıp çalışmadığını gösterir | Gerçekten stuck olan instance'ın yeniden başlatılmasını sağlar |
| **Startup Probe** | Başlangıç sürecine özel probe | Yavaş başlayan uygulamaların erken öldürülmesini önler |
| **Signal Handling** | OS sinyallerini kontrollü ele almak | Shutdown ve lifecycle davranışını deterministik hale getirir |
| **Shutdown Deadline** | Kapanış için maksimum süre belirlemek | Shutdown'ın sonsuza kadar sürmesini engeller |
| **Single Responsibility `main`** | `main`'i bootstrap/composition ile sınırlamak | `main.go`'nun business logic çöplüğüne dönüşmesini önler |
| **Dependency Composition Root** | Dependency'leri tek bir bootstrap noktasında oluşturmak | Wiring'in dağılmasını ve global state ihtiyacını azaltır |

### Graceful Shutdown

Tipik akış:

```text
K8s
  |
  | SIGTERM
  v
Application
  |
  +--> yeni request kabulünü durdur
  |
  +--> aktif request'leri tamamla
  |
  +--> worker'ları durdur
  |
  +--> queue consumer'larını kapat
  |
  +--> DB/HTTP kaynaklarını kapat
  |
  v
Exit
```

Buradaki kritik fikir:

> Shutdown bir `os.Exit()` değil, kontrollü bir lifecycle aşamasıdır.

---

# 2. Configuration

Configuration pattern'leri uygulamanın **hangi şartlarda çalışabileceğini** daha baştan garanti altına alır.

| Pattern | Ne? | Neyi çözer? |
|---|---|---|
| **Fail-Fast Config** | Env/config'i startup'ta validate etmek | Yanlış config ile çalışan servisleri önler |
| **Typed Configuration** | String env değerlerini typed struct'a dönüştürmek | Her yerde `os.Getenv` ve manuel parse dağınıklığını önler |
| **Config Validation** | Port, URL, timeout, enum vb. doğrulama | Geçersiz config'in runtime'da patlamasını önler |
| **Default vs Required** | Default değer ile zorunlu değeri ayırmak | Yanlışlıkla kritik config'e sessiz default verilmesini önler |
| **Environment-specific Config** | Ortama göre config yönetimi | Dev/staging/prod farklarının kontrollü olmasını sağlar |
| **Secret Separation** | Secret'ları normal config'ten ayırmak | Secret exposure riskini azaltır |
| **Immutable Runtime Config** | Config'i startup sonrası değiştirmemek | Runtime state'in öngörülemez şekilde değişmesini önler |
| **Config Sanitization** | Config loglanırken secret'ları maskelemek | Token/password gibi değerlerin loglara sızmasını önler |

Örnek:

```go
type Config struct {
    Port        int
    DatabaseURL string
    HTTPTimeout time.Duration
}
```

Startup:

```go
cfg, err := LoadConfig()
if err != nil {
    log.Fatal(err)
}
```

Amaç:

```text
Bad:
service starts
    |
    +--> 3 hours later
    +--> DB connection error
    +--> investigation
    +--> "hangi env eksik?"

Good:
service starts
    |
    +--> config validation
    |
    +--> invalid -> fail immediately
```

---

# 3. Dependency Injection

Dependency Injection (DI), bir component'in ihtiyaç duyduğu dependency'leri **kendi içinde yaratmak yerine dışarıdan almasıdır**.

| Pattern | Ne? | Neyi çözer? |
|---|---|---|
| **Constructor Injection** | Dependency'leri constructor ile geçirmek | Dependency'leri açık ve görünür yapar |
| **Repository Interface** | DB davranışını interface ile soyutlamak | Business logic'in gerçek DB'ye bağımlılığını azaltır |
| **Service Interface** | Business logic kontratını ayırmak | HTTP katmanından bağımsız test/organizasyon sağlar |
| **Dependency Inversion** | Üst seviye logic'in concrete infrastructure'a bağlanmaması | Coupling'i azaltır |
| **Composition Root** | Tüm dependency wiring'ini tek yerde yapmak | Uygulamanın nasıl kurulduğunu görünür kılar |
| **Interface at Consumer** | Interface'i kullanan tarafın tanımlaması | Gereksiz büyük interface'leri önler |
| **Fake / Stub / Mock** | Testte gerçek dependency yerine test double kullanmak | Unit testleri hızlı ve deterministik yapar |
| **Functional Options** | Optional config/dependency'leri option fonksiyonlarıyla geçirmek | Constructor parametre patlamasını azaltır |
| **Explicit Dependencies** | Dependency'leri açıkça geçirmek | Global singleton ve gizli state'i azaltır |

Tipik katman:

```text
HTTP
  |
  v
Handler
  |
  v
Service
  |
  +--> Repository
  |
  +--> Downstream Client
```

Handler DB'nin nasıl çalıştığını bilmez.

Service HTTP response yazmaz.

Repository HTTP request bilmez.

Bu sınırlar test edilebilirliği ve değiştirilebilirliği ciddi şekilde artırır.

### Go'da önemli nüans

Her struct için interface yazmak doğru değildir.

Daha iyi yaklaşım:

> Interface'i, onu kullanan consumer'ın ihtiyaç duyduğu küçük davranış için tanımla.

---

# 4. Repository & Persistence

Repository katmanı persistence detaylarını business logic'ten ayırır.

| Pattern | Ne? | Neyi çözer? |
|---|---|---|
| **Repository Pattern** | DB erişimini abstraction arkasına almak | Service/business logic'in DB detaylarına bağımlılığını azaltır |
| **Unit of Work** | Birden fazla işlemi tek transaction unit'i olarak yönetmek | Çok adımlı değişikliklerin birlikte commit/rollback edilmesini sağlar |
| **Transaction Boundary** | Transaction'ın nerede başlayıp bittiğini belirlemek | Gereksiz uzun transaction ve yanlış consistency modelini önler |
| **Context-aware Queries** | DB operation'larına `context.Context` vermek | Request cancellation/deadline'ını DB'ye taşır |
| **Query Timeout** | DB sorgusuna süre sınırı koymak | Uzayan sorguların kaynakları sonsuza kadar tutmasını önler |
| **Connection Pool Limits** | DB connection sayısını sınırlamak | DB exhaustion ve connection storm'u azaltır |
| **Prepared Statements** | Tekrarlanan query'leri prepare etmek | Bazı workload'larda parsing/plan maliyetini azaltabilir |
| **Pagination** | Sonuçları sayfalara bölmek | Devasa result set ve memory kullanımını önler |
| **Keyset Pagination** | Cursor/last-seen key ile ilerlemek | Büyük tablolarda offset'in performans problemlerini azaltır |
| **Optimistic Locking** | Version/timestamp kontrolüyle concurrent update yönetmek | Lost update'i önler |
| **Idempotent Write** | Aynı operation tekrarlandığında aynı sonucu korumak | Retry nedeniyle duplicate kayıtları önler |
| **Soft Delete** | Kaydı fiziksel silmek yerine işaretlemek | Audit/recovery gibi ihtiyaçları karşılar |

Önemli prensip:

> Connection pool, queue, goroutine veya transaction gibi kaynakların **maksimum sınırı** düşünülmelidir.

---

# 5. Context

Go'da `context.Context`, request lifecycle, cancellation ve deadline bilgisini dependency zincirinde taşımak için kullanılır.

| Pattern | Ne? | Neyi çözer? |
|---|---|---|
| **Context Propagation** | Context'i handler → service → repository/client boyunca geçirmek | Request lifecycle'ını alt katmanlara taşır |
| **Context-Scoped DB Timeout** | DB query'yi context deadline'ına bağlamak | Yavaş query'nin connection pool'u kilitlemesini azaltır |
| **Context-aware HTTP Request** | HTTP request'i context ile yapmak | Client disconnect/timeout olduğunda downstream çağrıyı iptal eder |
| **Context Cancellation** | `ctx.Done()` üzerinden işi durdurmak | Artık ihtiyaç kalmayan işin devam etmesini önler |
| **Deadline Propagation** | Toplam request süresini dependency'lere taşımak | Bir request'in downstream zincirinde sonsuza kadar yaşamasını önler |
| **Context Values Sparingly** | Context value kullanımını sınırlamak | Context'in gizli global dependency container'a dönüşmesini önler |

Örnek:

```go
ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
defer cancel()

err := repo.GetUser(ctx, id)
```

Repository:

```go
row := db.QueryRowContext(ctx, query, id)
```

Böylece:

```text
HTTP request
    |
    v
Service
    |
    v
Repository
    |
    v
DB
```

aynı cancellation/deadline zincirinde kalır.

---

# 6. Error Handling

Production error handling'de amaç sadece “error dönmek” değil, **hatanın anlamını kaybetmeden katmanlar arasında taşımaktır**.

| Pattern | Ne? | Neyi çözer? |
|---|---|---|
| **Sentinel Errors** | Tanımlı hata değerleri | `err.Error()` string karşılaştırmasını ortadan kaldırır |
| **Typed Errors** | Hata tipleri | Structured error bilgisi taşır |
| **Error Wrapping** | `%w` ile alt hatayı context ile sarmak | Root cause'u kaybetmeden anlam ekler |
| **`errors.Is`** | Error identity kontrolü | Wrapped error'larda güvenilir sınıflandırma sağlar |
| **`errors.As`** | Belirli error type'ını çıkarmak | Typed error bilgisine ulaşmayı sağlar |
| **Central Error Mapping** | Domain/service error → HTTP status mapping | Handler'larda tekrar eden mapping logic'i azaltır |
| **Internal vs Public Errors** | İç hata ile client'a gösterilecek hatayı ayırmak | DB/vendor detaylarının sızmasını önler |
| **Error Taxonomy** | Not found, conflict, validation, internal vb. sınıflar | Tutarlı davranış sağlar |
| **Structured Error Response** | Standart JSON error formatı | Client'ların hataları tutarlı işlemesini sağlar |
| **Error Context** | Hatanın hangi operation'da oluştuğunu belirtmek | Debugging'i kolaylaştırır |
| **No String Matching** | `err.Error() == "not found"` kullanmamak | Mesaj değişikliğinde kırılan logic'i önler |

Örnek:

```go
var ErrNotFound = errors.New("resource not found")
```

Service:

```go
return ErrNotFound
```

Handler:

```go
if errors.Is(err, ErrNotFound) {
    writeError(w, http.StatusNotFound)
    return
}
```

Bunun avantajı:

```text
DB error
   |
   v
Repository
   |
   v
Service: wraps / classifies
   |
   v
Handler: maps to HTTP
   |
   v
404 / 409 / 422 / 500
```

Client'a şunu vermemek gerekir:

```json
{
  "error": "pq: password authentication failed for user..."
}
```

İç detay loglarda kalabilir; public response kontrollü olur.

---

# 7. HTTP Server Hardening

HTTP endpoint'lerinin sadece “doğru request'te çalışması” yetmez. Kötü, yanlış veya aşırı büyük request'lere karşı da dayanıklı olması gerekir.

| Pattern | Ne? | Neyi çözer? |
|---|---|---|
| **Request Body Limit** | Body boyutunu sınırlamak | Memory exhaustion / DoS riskini azaltır |
| **Strict JSON Decode** | Unknown JSON field'larını reddetmek | Sessiz client bug'larını yakalar |
| **Single JSON Value** | Body'de tek JSON document kabul etmek | Fazladan JSON değerlerinin sessizce yutulmasını önler |
| **Content-Type Validation** | Beklenen media type'ı doğrulamak | Yanlış body formatlarını erkenden reddeder |
| **Request Timeout** | Handler için maksimum süre | Hanging request'leri sınırlar |
| **Read Header Timeout** | Header okuma süresi | Slowloris benzeri kaynak tüketimini azaltır |
| **Write Timeout** | Response yazma süresi | Slow client'ın server resource'larını tutmasını azaltır |
| **Idle Timeout** | Idle connection süresi | Kullanılmayan bağlantıların sonsuza kadar tutulmasını önler |
| **Max Header Size** | Header boyutunu sınırlamak | Aşırı büyük header saldırılarını azaltır |
| **Request Size Limits** | Endpoint bazında input limitleri | Resource kullanımını kontrol eder |
| **Panic Recovery Middleware** | Handler panic'lerini yakalamak | Tek request panic'inin process'i düşürmesini önler |
| **Request ID** | Her request'e ID vermek | Log/tracing correlation sağlar |
| **Correlation ID** | Dağıtık çağrılarda ortak ID taşımak | Bir operation'ın birden fazla serviste takip edilmesini sağlar |

Strict JSON:

```go
decoder := json.NewDecoder(r.Body)
decoder.DisallowUnknownFields()

if err := decoder.Decode(&req); err != nil {
    // 400
}
```

Body limit:

```go
r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
```

Böylece:

```json
{
  "emali": "test@example.com"
}
```

gibi typo'lar sessizce yok sayılmak yerine yakalanabilir.

---

# 8. HTTP Client & Downstream

Bir backend servisinin başka servislere yaptığı HTTP çağrılarında **timeout, connection reuse ve failure isolation** kritik önemdedir.

| Pattern | Ne? | Neyi çözer? |
|---|---|---|
| **Reused `http.Client`** | Client'ı request başına oluşturmamak | Connection pool/keep-alive kullanımını korur |
| **Explicit HTTP Timeout** | Client'a timeout vermek | Sonsuz beklemeyi önler |
| **Context-aware Requests** | Request'e context bağlamak | Cancellation/deadline propagation sağlar |
| **Connection Pooling** | TCP/TLS connection'larını yeniden kullanmak | Connection setup maliyetini azaltır |
| **MaxIdleConns** | Idle connection sayısını kontrol etmek | Connection resource kullanımını sınırlar |
| **MaxConnsPerHost** | Host başına connection üst sınırı | Downstream'e connection flood'u önler |
| **IdleConnTimeout** | Idle connection yaşam süresi | Eski bağlantıları temizler |
| **Retry with Backoff** | Geçici hatalarda kontrollü retry | Transient network failure'lara dayanıklılık sağlar |
| **Exponential Backoff** | Retry aralığını artırmak | Sürekli retry storm'u azaltır |
| **Jitter** | Retry süresine random sapma eklemek | Binlerce instance'ın aynı anda retry etmesini önler |
| **Retry Budget** | Retry için ayrılmış limit | Retry mekanizmasının sistemi daha da bozmasını önler |
| **Circuit Breaker** | Başarısız dependency'yi geçici olarak devre dışı bırakmak | Cascade failure'ı azaltır |
| **Bulkhead** | Dependency/workload kapasitesini izole etmek | Bir dependency'nin diğerlerini boğmasını önler |
| **Response Body Close** | Body'yi kapatmak | Connection reuse ve resource cleanup için gereklidir |
| **Status Code Validation** | HTTP status'u açıkça kontrol etmek | `500` response'unun başarılı response gibi işlenmesini önler |
| **Response Size Limit** | Downstream response boyutunu sınırlamak | Memory exhaustion riskini azaltır |

Kötü:

```go
resp, err := http.Get(url)
```

Daha kontrollü:

```go
client := &http.Client{
    Timeout: 5 * time.Second,
}
```

Bu client application lifetime boyunca reuse edilebilir.

### Retry konusunda kritik nokta

Her şeyi retry etmek doğru değildir.

Örneğin:

```text
GET /user
```

çoğu durumda retry edilebilir.

Ama:

```text
POST /charge-card
```

idempotency olmadan körlemesine retry edilirse duplicate charge oluşturabilir.

---

# 9. Concurrency

Concurrency pattern'leri sistemin aynı anda ne kadar iş yapabileceğini ve fazla yük altında nasıl davranacağını kontrol eder.

| Pattern | Ne? | Neyi çözer? |
|---|---|---|
| **Bounded Worker Pool** | Sabit/bounded worker sayısı | Goroutine explosion ve uncontrolled concurrency'yi önler |
| **Semaphore** | Aynı anda çalışan operation sayısını sınırlayan kapasite mekanizması | Belirli bir resource/operation için concurrency limit koyar |
| **Rate Limiter** | Birim zamanda izin verilen işlem sayısını sınırlar | Traffic burst ve downstream overload'u azaltır |
| **Backpressure** | Consumer yetişemediğinde producer'ı yavaşlatmak/reddetmek | Queue/memory büyümesini kontrol eder |
| **Bounded Queue** | Queue capacity'sini sınırlamak | Sınırsız memory growth'u önler |
| **Fan-out / Fan-in** | İşleri paralel dağıtıp sonuçları birleştirmek | Paralel processing'i organize eder |
| **`errgroup`** | Concurrent goroutine'leri birlikte yönetmek | Error propagation ve cancellation sağlar |
| **WaitGroup** | Goroutine'lerin tamamlanmasını beklemek | Goroutine lifecycle'ını takip eder |
| **Singleflight** | Aynı işi concurrent çağrılarda bir kez çalıştırmak | Duplicate expensive work'ü azaltır |
| **Channel Ownership** | Channel'ı kimin açıp kapatacağının net olması | Channel lifecycle bug'larını azaltır |
| **Data Race Prevention** | Shared state erişimini güvenli hale getirmek | Race condition'ları önler |
| **Mutex Discipline** | Lock scope'unu küçük ve net tutmak | Gereksiz contention'i azaltır |
| **Atomic Operations** | Basit shared state için atomic işlemler | Bazı durumlarda lock maliyetini azaltır |

### Worker Pool vs Semaphore

Bunlar aynı değildir.

**Semaphore:**

> “Aynı anda maksimum kaç operation çalışabilir?”

```text
1000 goroutine
     |
     v
 semaphore(5)
     |
     v
max 5 active operations
```

**Worker Pool:**

> “Bu işleri kaç worker işleyecek?”

```text
1000 jobs
   |
   v
queue
   |
   +--> worker 1
   +--> worker 2
   +--> worker 3
   +--> worker 4
   +--> worker 5
```

Worker pool doğal olarak goroutine sayısını da bounded tutar.

### Bulkhead ile ilişkisi

```text
Bounded Worker Pool
    = concurrency/resource bound

Bulkhead
    = workload/failure isolation
```

Ayrı workload'lara ayrı worker pool vermek bulkhead oluşturmanın bir yolu olabilir.

---

# 10. API Design

API tasarım pattern'leri client ile server arasındaki kontratı daha güvenilir hale getirir.

| Pattern | Ne? | Neyi çözer? |
|---|---|---|
| **Idempotency Keys** | Aynı operation'ın unique key ile takip edilmesi | Retry sonucu duplicate operation'ları önler |
| **Pagination** | Büyük collection'ları parçalara bölmek | Büyük response ve DB yükünü azaltır |
| **Filtering** | Server-side filtreleme | Gereksiz veri transferini azaltır |
| **Sorting** | Kontrollü server-side sıralama | Client'ın ihtiyaç duyduğu sıralamayı güvenli biçimde sağlar |
| **Partial Update Semantics** | PATCH davranışını netleştirmek | Update kontratındaki belirsizlikleri azaltır |
| **Optimistic Concurrency** | Version/ETag vb. ile update kontrolü | Stale client'ın yeni veriyi ezmesini önler |
| **Consistent Response Envelope** | Tutarlı response formatı | Client kodunu sadeleştirir |
| **API Versioning** | Breaking change'leri versiyonlamak | Eski client'ların kırılmasını önler |
| **Validation at Boundary** | Input'u API girişinde doğrulamak | Invalid data'nın içeri yayılmasını önler |
| **Explicit DTOs** | API request/response modellerini domain/DB'den ayırmak | Persistence modelinin API'ye sızmasını önler |
| **Input Normalization** | Email/phone vb. değerleri canonical hale getirmek | Aynı verinin farklı formatlarda çoğalmasını azaltır |
| **Output Filtering** | Response'tan hassas/internal field'ları çıkarmak | Data leakage riskini azaltır |

---

# 11. Middleware

Middleware cross-cutting concerns için kullanılır.

| Pattern | Ne? | Neyi çözer? |
|---|---|---|
| **Request ID Middleware** | Request ID üretir/taşır | Log correlation |
| **Logging Middleware** | HTTP access log üretir | Trafik ve hata görünürlüğü |
| **Recovery Middleware** | Panic'i yakalar | Process crash riskini azaltır |
| **Timeout Middleware** | Request deadline uygular | Hanging handler'ları sınırlar |
| **Authentication Middleware** | Kullanıcı kimliğini doğrular | Unauthorized access'i önler |
| **Authorization Middleware** | Yetki kontrolü yapar | Authenticated ama yetkisiz erişimi önler |
| **Rate Limiting Middleware** | Request rate'ini sınırlar | Abuse ve overload'u azaltır |
| **CORS Middleware** | Browser origin policy yönetimi | Cross-origin API kullanımını kontrol eder |
| **Compression Middleware** | Response'u sıkıştırır | Network bandwidth kullanımını azaltır |
| **Metrics Middleware** | HTTP metrics üretir | Latency/error/traffic görünürlüğü |
| **Tracing Middleware** | HTTP trace/span oluşturur | Distributed request tracing |

Prensip:

> Middleware'e her şeyi koyma.

Business logic middleware'e sızarsa katman sınırları bulanıklaşır.

---

# 12. Authentication & Authorization

Authentication “kimsin?”, authorization ise “bunu yapmaya yetkin var mı?” sorusudur.

| Pattern | Ne? | Neyi çözer? |
|---|---|---|
| **Authentication ≠ Authorization** | Kimlik ve yetkiyi ayrı kavramlar olarak ele almak | “Login olmuş” ile “izinli” arasındaki farkı korur |
| **JWT Validation** | Token signature/claims doğrulama | Sahte veya geçersiz token kullanımını önler |
| **Token Expiry** | Token'ın yaşam süresini sınırlamak | Çalınmış token'ın etkisini sınırlar |
| **Refresh Token Rotation** | Refresh token'ları döndürmek | Token replay riskini azaltır |
| **RBAC** | Role-based access control | Role göre yetkilendirme |
| **ABAC** | Attribute-based access control | Daha detaylı policy'ler |
| **Resource Ownership Checks** | Resource sahibini doğrulamak | IDOR benzeri erişim problemlerini önler |
| **Permission Boundary** | Service/resource bazında izin sınırı | Yetkinin yanlış katmana taşmasını önler |
| **Fail-closed Authorization** | Belirsizlikte erişimi reddetmek | Güvenli default sağlar |

---

# 13. Database & Transaction İncelikleri

Bu bölüm correctness ve production stability açısından özellikle önemlidir.

| Pattern | Ne? | Neyi çözer? |
|---|---|---|
| **Transaction Boundary** | Transaction'ın kapsamını doğru belirlemek | Gereksiz lock ve uzun transaction'ları önler |
| **No Network Call in Transaction** | Transaction açıkken HTTP/Kafka vb. beklememek | DB lock'larının network latency ile uzamasını önler |
| **Short Lock Duration** | Lock'ları mümkün olduğunca kısa tutmak | Contention/deadlock riskini azaltır |
| **Deadlock Retry** | Uygun transaction'ları kontrollü retry etmek | Geçici deadlock'lara dayanıklılık sağlar |
| **Connection Pool Sizing** | DB pool'u kontrollü boyutlandırmak | DB'nin aşırı connection ile boğulmasını önler |
| **Statement Timeout** | DB tarafında statement süresini sınırlamak | Uzun sorguların kaynak tüketimini sınırlar |
| **Index Validation** | Query plan/index davranışını doğrulamak | Production'da beklenmeyen full scan'leri azaltır |
| **N+1 Detection** | Tek collection için gereksiz query patlamalarını yakalamak | DB round-trip sayısını azaltır |
| **Batch Insert/Update** | Birden fazla işlemi batch yapmak | DB round-trip maliyetini azaltır |
| **Read/Write Separation** | Okuma/yazma kaynaklarını ayırmak | Scale ve workload isolation sağlar |
| **Replica Lag Awareness** | Read replica gecikmesini hesaba katmak | Yeni yazılan verinin hemen okunamaması problemini yönetir |
| **Optimistic Locking** | Version kontrolü | Concurrent overwrite'i önler |
| **Unique Constraints** | Uniqueness'ı DB seviyesinde enforce etmek | Race condition kaynaklı duplicate'leri önler |
| **Foreign Key Constraints** | Referential integrity | Orphan/inconsistent data'yı önler |
| **Migration Versioning** | Schema değişikliklerini versionlamak | Deployment ve rollback yönetimini kolaylaştırır |
| **Backward-Compatible Migration** | Eski ve yeni kodun aynı schema ile bir süre çalışabilmesi | Zero-downtime deployment'ı kolaylaştırır |

### DB constraint neden önemli?

Kötü:

```text
SELECT exists
    |
    v
if not exists
    |
    v
INSERT
```

İki concurrent request aynı anda `not exists` görebilir.

Daha güvenli:

```text
DB:
UNIQUE(...)
```

Application logic son savunma hattı olabilir ama **DB integrity constraint'i nihai garantidir**.

---

# 14. Caching

Cache doğru kullanıldığında latency ve DB yükünü dramatik biçimde azaltabilir; yanlış kullanıldığında consistency problemleri çıkarabilir.

| Pattern | Ne? | Neyi çözer? |
|---|---|---|
| **Cache-aside** | Önce cache, miss'te DB, sonra cache | En yaygın cache modelidir |
| **TTL** | Cache entry için yaşam süresi | Stale data'yı sınırlar |
| **Cache Invalidation** | Data değişince cache'i temizlemek/güncellemek | Eski verinin uzun süre servis edilmesini azaltır |
| **Cache Stampede Prevention** | Aynı anda çok sayıda miss'i tek fetch'e indirmek | DB'ye ani yük binmesini önler |
| **Singleflight Cache Fill** | Aynı key için tek refill | Duplicate expensive fetch'i önler |
| **Negative Caching** | “Yok” sonucunu da kısa süre cache'lemek | Olmayan resource için sürekli DB query'sini azaltır |
| **Bounded Cache** | Cache boyutuna sınır koymak | Memory explosion'ı önler |
| **Jittered TTL** | TTL'lere random sapma vermek | Binlerce entry'nin aynı anda expire olmasını önler |

---

# 15. Messaging & Queue

Kafka, RabbitMQ, SQS gibi sistemlerde delivery modelini ve failure davranışını açıkça tasarlamak gerekir.

| Pattern | Ne? | Neyi çözer? |
|---|---|---|
| **At-least-once Processing** | Message'ın tekrar gelebileceğini kabul etmek | Delivery reliability sağlar |
| **Idempotent Consumer** | Aynı message tekrar gelse de sonucu bozmamak | Duplicate delivery problemini çözer |
| **Dead Letter Queue** | İşlenemeyen message'ları ayırmak | Poison message'ın sistemi kilitlemesini önler |
| **Retry with Backoff** | Failed message'ı kontrollü tekrar denemek | Transient failure'lara dayanıklılık |
| **Poison Message Handling** | Sürekli başarısız message'ı sınırlamak | Sonsuz retry loop'u önler |
| **Consumer Timeout** | Message processing için süre sınırı | Stuck consumer problemlerini azaltır |
| **Graceful Consumer Shutdown** | Consumer'ı kontrollü kapatmak | Deploy sırasında message kaybını/duplicate'i azaltır |
| **Consumer Concurrency Limit** | Paralel consumer sayısını sınırlandırmak | Downstream overload'u azaltır |
| **Backpressure** | Queue büyümesini kontrol etmek | Consumer'ın kapasitesinin aşılmasını önler |
| **Schema Versioning** | Event schema değişikliklerini yönetmek | Eski/yeni consumer uyumluluğunu sağlar |
| **Outbox Pattern** | DB change ile event publish'i güvenilir şekilde bağlamak | DB başarılı + event kayıp problemini azaltır |
| **Inbox Pattern** | Gelen event'i processed olarak kaydetmek | Duplicate event processing'i önler |
| **Transactional Outbox** | Transaction içinde business data + outbox event yazmak | Atomic persistence sağlar |
| **Message Ordering** | Event sırasını korumak | Sıra bağımlı domain logic'i korur |

---

# 16. Distributed Systems

Distributed system'lerde network güvenilir değildir. Bu yüzden failure normal bir durum olarak tasarlanmalıdır.

| Pattern | Ne? | Neyi çözer? |
|---|---|---|
| **Timeout** | Dependency için maksimum bekleme | Hanging dependency |
| **Retry** | Geçici hatada tekrar deneme | Transient failure |
| **Backoff** | Retry'lar arasında bekleme | Retry storm |
| **Jitter** | Retry zamanını dağıtma | Thundering herd |
| **Circuit Breaker** | Dependency'yi geçici olarak bypass etme | Cascade failure |
| **Bulkhead** | Resource/workload izolasyonu | Bir failure'ın diğer workload'lara yayılması |
| **Rate Limiting** | Traffic sınırı | Overload/abuse |
| **Load Shedding** | Kapasite aşımında bazı işleri erken reddetmek | Total collapse yerine kontrollü degradation |
| **Backpressure** | Producer'ı consumer kapasitesine uydurmak | Queue/memory explosion |
| **Idempotency** | Retry'ları güvenli hale getirmek | Duplicate side effect |
| **Deduplication** | Aynı event/request'i tanıyıp tekrar işlememek | Duplicate processing |
| **Distributed Lock** | Birden fazla instance arasında exclusive operation | Concurrent job duplication |
| **Leader Election** | Bir işi bir instance'ın üstlenmesini sağlamak | Duplicate scheduler/worker execution |
| **Saga** | Distributed transaction yerine step + compensation modeli | Multi-service workflow consistency |
| **Outbox** | DB + event consistency | Lost event |
| **Inbox** | Event processing state'i | Duplicate event |
| **Eventual Consistency** | Anlık yerine zaman içinde tutarlılık | Distributed transaction maliyetini azaltmak |
| **Compensation** | Başarısız workflow step'ini ters işlemle telafi etmek | Distributed rollback ihtiyacını yönetmek |
| **Correlation ID** | Operation ID'sini servisler arasında taşımak | Debugging |
| **Distributed Tracing** | Request'i servisler boyunca trace etmek | Latency/failure lokasyonu |

### Timeout temel prensibi

> Timeout olmayan distributed call, potansiyel olarak kontrolsüz resource retention'dır.

---

# 17. Observability

Production'da amaç sadece sistemi çalıştırmak değil, **neden çalışmadığını hızlı anlayabilmektir**.

## Logging

| Pattern | Neyi çözer? |
|---|---|
| **Structured Logging** | Makine tarafından parse edilebilir loglar |
| **JSON Logs** | Merkezi log sistemleriyle kolay işleme |
| **Request ID** | Tek request'in loglarını bulma |
| **Correlation ID** | Distributed operation'ı takip etme |
| **Error Wrapping** | Hatanın hangi operation'da olduğunu anlama |
| **Log Levels** | Debug/info/warn/error ayrımı |
| **Sensitive Data Redaction** | Password/token/PII sızıntısını azaltma |
| **Log Sampling** | Aşırı log üretimini azaltma |

## Metrics

Temel metric'ler:

```text
request_count
error_rate
latency_p50
latency_p95
latency_p99
db_latency
db_pool_usage
queue_depth
worker_utilization
downstream_error_rate
retry_count
circuit_breaker_state
```

## Tracing

Örnek:

```text
Trace
 |
 +-- HTTP handler
 |
 +-- Service
 |
 +-- DB query
 |
 +-- Payment API
 |
 +-- Kafka publish
```

Mental model:

> **Logs = ne oldu?**  
> **Metrics = ne kadar oldu?**  
> **Traces = nerede oldu?**

---

# 18. Testing

Production confidence'ın temelidir.

| Pattern | Ne? | Neyi çözer? |
|---|---|---|
| **Table-driven Tests** | Test case'lerini data tablosu olarak tanımlamak | Yeni senaryo eklemeyi kolaylaştırır |
| **`httptest`** | Gerçek server açmadan HTTP test etmek | Handler testlerini hızlı ve izole yapar |
| **Repository Fake** | Gerçek DB yerine fake repository kullanmak | Service unit testlerini DB'siz çalıştırır |
| **Service Unit Test** | Business logic'i dependency'lerden izole test etmek | Hızlı regression detection |
| **Integration Test** | Gerçek dependency ile test | Wiring ve gerçek DB davranışını doğrular |
| **Contract Test** | Service'ler arası API kontratını test etmek | Breaking integration değişikliklerini yakalar |
| **Golden Tests** | Beklenen büyük output'u snapshot olarak saklamak | Output regression'larını yakalar |
| **Fuzz Testing** | Çok çeşitli input üretmek | Edge-case ve parser bug'larını bulur |
| **Race Detector** | Concurrent testlerde race aramak | Data race'leri yakalar |
| **Benchmark Tests** | Performance benchmark'ı | Performance regression'ı takip eder |
| **Testcontainers** | Gerçek DB/Redis/Kafka'yı izole container'da çalıştırmak | Gerçeğe yakın integration test sağlar |
| **Property-based Testing** | Invariant'ları test etmek | Çok sayıda input üzerinde genel doğruluk arar |
| **Failure Injection** | Dependency failure simüle etmek | Failure behavior'ı test eder |
| **Timeout Tests** | Hanging dependency senaryoları | Timeout davranışını doğrular |
| **Cancellation Tests** | Context cancellation simüle etmek | Resource cleanup/cancellation davranışını doğrular |

Go araçları:

```bash
go test ./...
go test -race ./...
go test -run TestHandler
go test -fuzz=Fuzz
go test -bench=.
```

### Table-driven örneği

```go
tests := []struct {
    name       string
    input      string
    wantStatus int
}{
    {
        name:       "not found",
        input:      "missing",
        wantStatus: 404,
    },
    {
        name:       "invalid input",
        input:      "",
        wantStatus: 400,
    },
}
```

Yeni scenario eklemek çoğu zaman bir satır/entry eklemek kadar kolaylaşır.

---

# 19. Security Hardening

Security, tek bir middleware değil; sistemin her katmanına yayılan bir disiplindir.

| Pattern | Ne? | Neyi çözer? |
|---|---|---|
| **Input Validation** | Input'u sınırda doğrulamak | Beklenmeyen/zararlı input |
| **Output Encoding** | Output'u uygun context'te encode etmek | Injection riskleri |
| **SQL Parameterization** | Query parametrelerini bind etmek | SQL injection |
| **Request Body Limits** | Body boyutunu sınırlamak | Memory DoS |
| **Header Limits** | Header boyutunu sınırlamak | Resource abuse |
| **Timeout Limits** | İşlemlere süre koymak | Resource exhaustion |
| **Rate Limiting** | Request hızını sınırlamak | Abuse/brute-force/overload |
| **Authentication** | Kimlik doğrulamak | Unauthorized access |
| **Authorization** | Yetki kontrolü | Privilege abuse |
| **Secret Management** | Secret'ları güvenli yerde tutmak | Credential leakage |
| **Password Hashing** | Password'ları hashlemek | Plaintext password riskini önler |
| **Token Rotation** | Token'ları yenilemek | Token theft etkisini sınırlar |
| **CSRF Protection** | Cross-site request saldırılarını azaltmak | Browser session saldırıları |
| **SSRF Protection** | Server'ın saldırganın belirttiği URL'lere erişmesini sınırlandırmak | Internal network erişimi |
| **Path Traversal Prevention** | Dosya path'lerini kontrol etmek | Arbitrary file access |
| **File Upload Limits** | Upload boyut/tipini sınırlamak | Storage/memory abuse |
| **Content-Type Validation** | Dosya/request formatını doğrulamak | Parser abuse |
| **Security Headers** | Browser security header'ları | Browser tabanlı saldırı risklerini azaltır |
| **Audit Logging** | Kritik action'ları kaydetmek | Security investigation |
| **PII Redaction** | Hassas veriyi loglardan çıkarmak | Data leakage |
| **Dependency Scanning** | Dependency vulnerability kontrolü | Bilinen güvenlik açıklarını azaltır |
| **Container Image Scanning** | Image içindeki CVE'leri kontrol etmek | Supply-chain riskini azaltır |
| **Least Privilege** | Minimum gerekli permission | Blast radius'u azaltır |
| **Non-root Container** | Container'ı root olmadan çalıştırmak | Container compromise etkisini azaltır |
| **Read-only Filesystem** | Runtime filesystem'i mümkün olduğunca salt okunur yapmak | Persistence/tampering riskini azaltır |

---

# 20. Reliability Patterns

Reliability, tek tek component'lerin değil **sistemin failure altında davranışının** tasarlanmasıdır.

| Pattern | Ne? | Neyi çözer? |
|---|---|---|
| **Health Checks** | Servisin durumunu dışarı bildirmek | Orchestrator/monitoring visibility |
| **Readiness** | Trafik alabilirlik | Hazır olmayan instance'a traffic gitmesini önler |
| **Liveness** | Process health | Stuck instance recovery |
| **Graceful Shutdown** | Kontrollü kapanma | Deploy/restart sırasında request/message kaybını azaltır |
| **Timeouts** | İşlem süresi sınırı | Hanging dependency |
| **Retries** | Geçici failure recovery | Transient errors |
| **Circuit Breaker** | Failure sonrası dependency bypass | Cascade failure |
| **Bulkhead** | Kaynak/workload izolasyonu | Failure propagation |
| **Rate Limit** | Trafik sınırı | Overload |
| **Load Shedding** | Aşırı yükte request/job reddetme | Total collapse |
| **Backpressure** | Producer'ı yavaşlatma | Unbounded queue |
| **Idempotency** | Retry-safe operation | Duplicate side effect |
| **Dead Letter Queue** | Failed message isolation | Poison message |
| **Fallback** | Dependency yokken alternatif cevap | Availability |
| **Degraded Mode** | Servisin bazı özelliklerle çalışmaya devam etmesi | Partial failure altında availability |

Önemli düşünce:

> Bazen kontrollü `429`/`503`, bütün sistemin timeout olup çökmesinden çok daha iyi bir sonuçtur.

---

# 21. Deployment & Kubernetes

Production deployment'ta application code ile orchestrator'ın lifecycle davranışı birlikte düşünülmelidir.

| Pattern | Ne? | Neyi çözer? |
|---|---|---|
| **`SIGTERM` Handling** | K8s termination sinyalini ele almak | Graceful deployment |
| **`terminationGracePeriodSeconds`** | Pod kapanış süresi | Request/worker cleanup için zaman tanır |
| **`preStop`** | Shutdown öncesi lifecycle hook | Kontrollü drain işlemleri |
| **Readiness Probe** | Pod'un traffic alabilirliğini bildirmek | Bad pod'a traffic gitmesini önler |
| **Liveness Probe** | Pod'un canlılığını bildirmek | Stuck pod recovery |
| **Startup Probe** | Startup aşamasını ayrı değerlendirmek | Slow startup |
| **Rolling Deployment** | Eski/yeni pod'ları kademeli değiştirmek | Zero/minimal downtime |
| **Pod Disruption Budget** | Aynı anda ne kadar pod kaybedilebileceğini sınırlamak | Availability |
| **Resource Requests** | Scheduler'a gereken kaynak bilgisini vermek | Daha doğru scheduling |
| **Resource Limits** | Container resource ceiling | Noisy neighbor/resource exhaustion |
| **HPA** | Load'a göre replica sayısını değiştirmek | Capacity scaling |
| **Graceful Connection Draining** | Traffic kesilirken mevcut connection/request'leri yönetmek | Deploy sırasında request loss |
| **Backward-compatible Migrations** | Eski ve yeni code/schema uyumluluğu | Zero-downtime schema changes |
| **Canary Deployment** | Yeni sürümü küçük traffic ile denemek | Riskli deploy'ları sınırlamak |
| **Blue/Green Deployment** | İki deployment arasında trafik geçirmek | Hızlı rollback |

### Expand/Contract Migration

Örneğin bir column'u değiştireceksen:

```text
1. Yeni column'u ekle
2. Kod hem eski hem yeni yapıyı desteklesin
3. Data'yı migrate et
4. Read/write'u yeni yapıya geçir
5. Eski column'u daha sonra kaldır
```

Bu yaklaşım rolling deployment sırasında eski ve yeni pod'ların birlikte çalışmasına yardımcı olur.

---

# 22. Resource Management

Çok underrated ama production stability için temel bir konudur.

Her resource için şu sorular sorulmalıdır:

> **Kim açıyor?**
>
> **Kim kapatıyor?**
>
> **Ne kadar süre tutuluyor?**
>
> **Maksimum kaç tane olabilir?**
>
> **Timeout nedir?**

Kontrol edilmesi gereken resource'lar:

```text
DB connections
HTTP connections
File descriptors
Goroutines
Channels
Memory buffers
Queue entries
Cache entries
Worker slots
```

Örnek:

```go
resp, err := client.Do(req)
if err != nil {
    return err
}
defer resp.Body.Close()
```

Buradaki `Close()` küçük görünür ama resource lifecycle'ın parçasıdır.

Ana prensip:

> **Her resource'un ownership'i ve upper bound'u belli olmalı.**

---

# 23. Data Integrity

Özellikle order, payment, inventory gibi domain'lerde correctness availability kadar önemlidir.

| Pattern | Ne? | Neyi çözer? |
|---|---|---|
| **Idempotency Key** | Operation'ı unique key ile tanımlamak | Duplicate payment/order vb. |
| **Unique Constraints** | DB'de uniqueness garanti etmek | Race condition kaynaklı duplicate |
| **Transactions** | Birden fazla DB değişikliğini atomic yapmak | Partial write |
| **Optimistic Locking** | Version kontrolü | Lost update |
| **Pessimistic Locking** | DB lock ile concurrent erişimi kontrol etmek | Bazı race senaryoları |
| **Atomic Updates** | DB'de tek atomic statement | Read-modify-write race'lerini azaltır |
| **State Machines** | Geçerli state transition'larını tanımlamak | Invalid state transition |
| **Invariants** | Domain'de her zaman doğru olması gereken kurallar | Data corruption |
| **Audit Trail** | Kritik değişikliklerin geçmişini tutmak | Investigation/compliance |
| **Immutable Events** | Geçmiş event'leri değiştirmemek | Event history consistency |
| **Outbox** | DB + event consistency | Lost event |
| **Deduplication** | Duplicate operation/event'i tanımak | Reprocessing |
| **Reconciliation Jobs** | Sistemler arası farkları periyodik kontrol etmek | Uzun vadeli consistency problemleri |

### State Machine

Örneğin payment:

```text
PENDING
   |
   v
PROCESSING
   |
   +------> SUCCEEDED
   |
   +------> FAILED
```

Rastgele boolean'lar:

```text
isPaid
isProcessing
hasFailed
```

yerine explicit state kullanmak çoğu zaman daha güvenlidir.

---

# 24. Background Jobs

Background job sistemlerinde request lifecycle dışındaki işler için de aynı reliability prensipleri uygulanmalıdır.

| Pattern | Ne? | Neyi çözer? |
|---|---|---|
| **Bounded Worker Pool** | Sabit worker sayısı | Goroutine explosion |
| **Context Cancellation** | Job'u iptal edebilmek | Gereksiz işin devamını önler |
| **Retry** | Geçici failure sonrası tekrar deneme | Transient error |
| **Backoff** | Retry'ı zamana yayma | Retry storm |
| **Job Timeout** | Tek job için maksimum süre | Stuck job |
| **Dead Letter Handling** | Sürekli başarısız job'ları ayırmak | Poison job |
| **Job Idempotency** | Aynı job tekrar gelse de güvenli olmak | Duplicate execution |
| **Job Deduplication** | Aynı işi tekrar queue'ya koymamak | Duplicate work |
| **Graceful Shutdown** | Worker'ları kontrollü kapatmak | Deploy sırasında yarım iş |
| **Queue Visibility Timeout** | Job'un tekrar alınabileceği süreyi yönetmek | Stuck/duplicate processing |
| **Job Heartbeat** | Uzun job'un alive olduğunu bildirmek | False timeout |
| **Max Attempts** | Retry sayısını sınırlamak | Infinite retry |
| **Scheduled Retry** | Retry'ı belirli zamanda yapmak | Kontrollü recovery |
| **Poison Job Detection** | Sürekli başarısız işleri işaretlemek | Worker starvation |
| **Job Metrics** | Queue depth, latency, failure ölçmek | Operasyonel visibility |

---

# 25. Code Organization

Amaç “klasörleri güzel göstermek” değil, **dependency direction ve responsibility sınırlarını korumaktır**.

Tipik bir yapı:

```text
cmd/
    api/
        main.go

internal/
    config/
    domain/
    handler/
    service/
    repository/
    client/
    middleware/
    worker/
```

Tipik dependency direction:

```text
HTTP
 |
 v
Handler
 |
 v
Service
 |
 +----> Repository
 |
 +----> Downstream Client
 |
 v
Infrastructure
```

Temel kurallar:

- Handler DB detaylarını bilmemeli.
- Service `http.ResponseWriter` bilmemeli.
- Repository HTTP request bilmemeli.
- Domain layer framework'e mümkün olduğunca az bağlı olmalı.
- Dependency wiring composition root'ta yapılmalı.
- Global mutable state minimumda tutulmalı.

Ama klasör yapısını dogmaya dönüştürmemek gerekir.

> Mimari sınırların amacı klasörleri çoğaltmak değil, **değişikliklerin blast radius'unu küçültmektir.**

---

# 26. İleri Seviye İnce Ayarlar

Bu bölümde pattern'lerin daha ileri kombinasyonları var.

## 26.1 Singleflight

Aynı expensive operation aynı anda çok kez isteniyorsa tek execution yapıp sonucu paylaşır.

```text
GET /user/123
GET /user/123
GET /user/123
GET /user/123

        |
        v

     singleflight

        |
        v

   DB query x1
```

Çözüm:

> Cache miss veya expensive initialization sırasında duplicate concurrent work'ü azaltır.

---

## 26.2 Bulkhead

Bir workload'un başka workload'un kapasitesini tüketmesini engeller.

Kötü:

```text
                Shared Pool
                    |
       +------------+------------+
       |            |            |
    Payment        Email        KYC
```

Payment bütün pool'u tüketebilir.

Bulkhead:

```text
Payment ----> 10 workers
Email   ---->  5 workers
KYC     ---->  5 workers
```

Çözüm:

> Failure isolation ve resource isolation.

---

## 26.3 Load Shedding

Sistem kapasitesini aştığında tüm request'leri yavaşlatmak yerine bazılarını erken reddetmek.

```text
capacity = 1000 req/s
incoming = 1200 req/s

        |
        v

controlled 429/503
```

Çözüm:

> Total collapse yerine kontrollü degradation.

---

## 26.4 Jitter

Retry/backoff sürelerine random sapma eklemek.

Kötü:

```text
instance A -> retry at 10s
instance B -> retry at 10s
instance C -> retry at 10s
instance D -> retry at 10s
```

Daha iyi:

```text
A -> 9.4s
B -> 10.7s
C -> 8.9s
D -> 11.3s
```

Çözüm:

> Thundering herd / synchronized retry storm.

---

## 26.5 Idempotency

Özellikle side-effect yaratan endpoint'lerde önemlidir.

```text
POST /payments

request
   |
   v
payment created
   |
   X response lost
   |
   v
client retries
```

Idempotency olmadan:

```text
payment #1
payment #2  <-- duplicate
```

Idempotency ile:

```text
idempotency-key = abc123

request #1 -> payment #1
request #2 -> existing result #1
```

Çözüm:

> Network retry'larının duplicate side effect üretmesini önlemek.

---

## 26.6 Outbox Pattern

DB update ile event publish'in ayrışmasından doğan consistency problemini azaltır.

```text
BEGIN
    INSERT order
    INSERT outbox_event
COMMIT

        |
        v

outbox publisher
        |
        v

Kafka
```

Böylece:

```text
DB success
Kafka failure
```

olduğunda event kaybolmaz; outbox'tan tekrar publish edilebilir.

Çözüm:

> Database state ile event publication arasındaki dual-write problemini yönetmek.

---

# 27. Büyük Resim: Pattern'lerin Ortak Prensipleri

Bu pattern'lerin çoğu aslında birkaç temel engineering prensibinin farklı uygulamalarıdır.

---

## 27.1 Unbounded → Bounded

Kontrol edilmesi gereken kaynaklar:

```text
goroutines
connections
memory
queues
retries
requests
workers
cache
```

Temel soru:

> **Bu resource'un maksimumu nedir?**

Örnek:

```text
50,000 request
      |
      X
50,000 goroutine

yerine

50,000 request
      |
      v
bounded queue
      |
      v
20 workers
```

---

## 27.2 Implicit → Explicit

Kötü:

```go
http.Get(url)
```

Daha kontrollü:

```go
client.Do(req)
```

Kötü:

```go
os.Getenv("PORT")
```

Her yerde tekrar tekrar okumak yerine:

```go
cfg.Port
```

Kötü:

```go
err.Error() == "not found"
```

Daha doğru:

```go
errors.Is(err, ErrNotFound)
```

Pattern'in ortak fikri:

> Önemli davranışı framework/default davranışına bırakmak yerine açıkça tanımla.

---

## 27.3 Uncontrolled → Timeout / Deadline

Şu kaynakların tamamında timeout düşün:

```text
HTTP
DB
Redis
Kafka
Worker
Queue
Shutdown
External API
```

Soru:

> **Bu operation maksimum ne kadar bekleyebilir?**

---

## 27.4 Implicit Failure → Explicit Failure

Failure'ı tasarımın parçası yap:

```text
timeout
retry
backoff
circuit breaker
dead letter queue
fallback
load shedding
health check
```

---

## 27.5 Coupled → Decoupled

Kötü:

```text
Handler ---> DB
```

Daha iyi:

```text
Handler
   |
   v
Service
   |
   v
Repository
   |
   v
DB
```

Amaç:

> Bir katmanın diğerinin implementation detaylarını bilmemesi.

---

## 27.6 Unobservable → Observable

Her kritik operation için:

```text
Logs
Metrics
Traces
```

olması teşhis süresini ciddi biçimde azaltır.

---

## 27.7 Non-idempotent → Retry-safe

Distributed systems'te:

```text
request
response
network failure
retry
```

normaldir.

Bu yüzden özellikle side-effect yaratan operation'larda:

```text
idempotency
deduplication
unique constraints
state machines
```

düşünülmelidir.

---

## 27.8 Failure Isolation

Sadece failure'ı önlemeye çalışma.

Failure'ın **nereye kadar yayılabileceğini** de sınırla.

```text
Payment failure
      |
      X
      |
      +---- Email continues
      |
      +---- KYC continues
      |
      +---- Reporting continues
```

Bu düşüncenin araçları:

```text
Bulkhead
Circuit Breaker
Bounded Pools
Separate Connection Pools
Separate Queues
Timeouts
Load Shedding
```

---

# 28. Özet: Hangi Pattern Hangi Problemi Çözer?

| Problem | İlk bakılacak pattern'ler |
|---|---|
| Deploy sırasında request kesiliyor | Graceful Shutdown, readiness, connection draining |
| Yanlış env 3 saat sonra fark ediliyor | Fail-Fast Config, Config Validation |
| Handler DB'ye çok bağlı | Repository Interface, DI |
| Unit test için gerçek DB gerekiyor | Repository Fake, DI |
| `err.Error()` string karşılaştırmaları | Sentinel Errors, `errors.Is`, typed errors |
| DB error client'a sızıyor | Central Error Mapping, Public/Internal Error ayrımı |
| Devasa JSON body geliyor | Body Limit |
| Client yanlış JSON field gönderiyor | `DisallowUnknownFields` |
| Handler testleri büyüyor | Table-driven tests, `httptest` |
| Downstream çağrı sonsuza kadar bekliyor | HTTP Client Timeout, Context |
| Her request yeni HTTP connection açıyor | Reused `http.Client`, Transport pooling |
| 50k job → 50k goroutine | Bounded Worker Pool |
| Aynı anda sadece N operation çalışmalı | Semaphore |
| Payment workload Email'i boğuyor | Bulkhead |
| Retry'lar sistemi daha da bozuyor | Backoff, Jitter, Retry Budget |
| Downstream tamamen çöktü | Circuit Breaker |
| Queue sonsuza kadar büyüyor | Bounded Queue, Backpressure |
| Sistem aşırı yükte tamamen ölüyor | Load Shedding |
| Aynı DB fetch 100 kere yapılıyor | Singleflight |
| Aynı payment iki kere oluşuyor | Idempotency |
| DB update oldu event kayboldu | Outbox |
| Event iki kere işlendi | Idempotent Consumer, Inbox, Deduplication |
| Concurrent update veri eziyor | Optimistic Locking |
| Duplicate kayıt oluşuyor | Unique Constraint, Idempotency |
| Büyük DB result set | Pagination, Keyset Pagination |
| DB connection pool tükeniyor | Query Timeout, Pool Limits |
| N+1 query | N+1 Detection, eager/batch query |
| Production'da “neden yavaş?” bilinmiyor | Metrics, Tracing, Structured Logging |
| Panic process'i düşürüyor | Recovery Middleware |
| Secret loglarda görünüyor | Redaction, Secret Separation |
| Malicious upload memory/storage tüketiyor | Body/File Size Limit |
| Scheduler her pod'da aynı job'u çalıştırıyor | Leader Election, Distributed Lock |
| Background job deploy sırasında yarım kalıyor | Graceful Worker Shutdown, Job Idempotency |
| Schema deployment sırasında eski pod kırılıyor | Expand/Contract Migration |
| Bir dependency failure'ı bütün sistemi etkiliyor | Bulkhead, Circuit Breaker, Timeouts |
| Goroutine/resource leak | Context Cancellation, bounded resources, ownership |
| Sistem failure altında nasıl davranacak bilinmiyor | Reliability Engineering + Failure Injection |

---

# Final Mental Model

Production backend engineering'i tek tek pattern ezberlemekten ziyade şu sorularla düşünmek daha değerlidir:

```text
1. Bu işlem ne kadar sürebilir?
        -> Timeout / Deadline

2. Aynı anda kaç tane çalışabilir?
        -> Semaphore / Worker Pool

3. Bu resource'un maksimumu nedir?
        -> Bounded Resource

4. Bu dependency çökerse ne olur?
        -> Circuit Breaker / Bulkhead / Fallback

5. Retry gelirse ne olur?
        -> Idempotency / Deduplication

6. Process kapanırsa ne olur?
        -> Graceful Shutdown

7. Config yanlışsa ne olur?
        -> Fail-Fast Validation

8. Client kötü/yanlış input gönderirse?
        -> Validation / Body Limit / Strict Decode

9. DB yavaşlarsa?
        -> Context Deadline / Query Timeout / Pool Limits

10. Aynı anda 50k iş gelirse?
        -> Queue / Backpressure / Worker Pool

11. Bir şey bozulduğunda nasıl anlayacağım?
        -> Logs / Metrics / Traces

12. Bunu gerçek dependency olmadan test edebilir miyim?
        -> DI / Interfaces / Fakes

13. Deploy sırasında eski ve yeni versiyon birlikte çalışabilir mi?
        -> Backward Compatibility / Expand-Contract

14. Failure bir component'ten diğerine yayılabilir mi?
        -> Bulkhead / Resource Isolation

15. Aynı operation iki kez çalışırsa ne olur?
        -> Idempotency / Unique Constraints / State Machine
```

## En önemli fikir

Bütün bu pattern'lerin arkasında birkaç büyük prensip var:

```text
                PRODUCTION ENGINEERING
                         |
        +----------------+----------------+
        |                |                |
     BOUNDED          EXPLICIT          ISOLATED
        |                |                |
   resources          behavior         failures
        |                |                |
        +----------------+----------------+
                         |
                  FAILURE-AWARE
                         |
        +----------------+----------------+
        |                |                |
     Timeout          Retry-safe      Observable
        |                |                |
        +----------------+----------------+
                         |
                    TESTABLE
                         |
                    MAINTAINABLE
```

> **Senior production engineering çoğu zaman “happy path'i nasıl çalıştırırım?” sorusundan ziyade “resource biterse, dependency ölürse, network timeout olursa, request retry edilirse, process kapanırsa veya traffic kapasiteyi aşarsa sistem nasıl davranacak?” sorusunu cevaplamaktır.**
