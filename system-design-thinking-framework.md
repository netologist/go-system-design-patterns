# System Design — Deep Concepts & Problem-Solving Framework

> Amaç: System Design sorularını ezberlenmiş mimari diyagramlarla değil, **ölçülebilir gereksinimler → bottleneck → trade-off → failure mode → tasarım → doğrulama** zinciriyle çözebilmek.

Bu doküman iki şeyi birlikte öğretir:

1. **Kavram haritası:** Availability, scalability, durability, latency, consistency, throughput, reliability, resilience, partition tolerance ve diğer temel kavramları gerçekten anlamak.
2. **Düşünme sistemi:** Bilmediğin bir system design sorusuna bile sistematik biçimde yaklaşabilmek.

---

# 0. Büyük Resim

Bir distributed system'i şu beş soruyla düşün:

```text
1. Ne kadar traffic var?
2. Ne kadar hızlı cevap vermeliyiz?
3. Ne kadar veri kaybetmeyi göze alabiliriz?
4. Hangi failure'larda çalışmaya devam etmeliyiz?
5. Hangi doğruluk / consistency seviyesine ihtiyacımız var?
```

Bunların üzerine:

```text
                SYSTEM
                   |
       +-----------+-----------+
       |           |           |
    Compute      Storage     Network
       |           |           |
       +-----------+-----------+
                   |
            Distributed State
                   |
       +-----------+-----------+
       |           |           |
 Consistency   Availability  Durability
       |           |           |
       +-----------+-----------+
                   |
              Trade-offs
                   |
          Failure + Recovery
                   |
             Observability
                   |
                Cost
```

System Design'ın özü:

> **Bir sistemin doğru çalışmasını değil, yük altında, network problemi varken, dependency çökerken, veri büyürken ve deploy yapılırken de kabul edilebilir davranmasını tasarlamak.**

---

# 1. Önce Bir Ayrım: Functional vs Non-Functional Requirements

Her system design sorusunun başlangıcında iki farklı requirement sınıfı vardır.

## 1.1 Functional Requirements

Sistem ne yapıyor?

Örnek:

```text
Twitter benzeri sistem:

- Kullanıcı tweet oluşturabilir.
- Kullanıcı tweet okuyabilir.
- Kullanıcı takip edebilir.
- Timeline görebilir.
```

Bunlar feature'dır.

## 1.2 Non-Functional Requirements

Sistem nasıl davranmalı?

Örnek:

```text
- 100M kullanıcı
- 10M DAU
- 100K requests/sec
- p99 < 200 ms
- 99.99% availability
- veri kaybı kabul edilmiyor
```

Bunlar architecture'ı belirler.

### Kritik prensip

Functional requirement:

> "Ne yapıyoruz?"

Non-functional requirement:

> "Ne kadar iyi yapmak zorundayız?"

System design kararlarının büyük kısmını ikinci grup belirler.

---

# 2. System Design'ın Temel Quality Attributes Haritası

En önemli kavramlar:

| Kavram | Temel soru |
|---|---|
| Availability | Sistem ne kadar süre kullanılabilir? |
| Reliability | Sistem doğru şekilde çalışmaya ne kadar güvenilir? |
| Scalability | Yük arttığında sistem büyüyebilir mi? |
| Elasticity | Kaynaklar değişen yüke otomatik uyum sağlayabiliyor mu? |
| Latency | Bir operation ne kadar sürede tamamlanıyor? |
| Throughput | Birim zamanda ne kadar iş yapıyoruz? |
| Durability | Yazılmış veri kayboluyor mu? |
| Consistency | Kullanıcılar hangi state'i görüyor? |
| Freshness | Veri ne kadar güncel? |
| Fault Tolerance | Component failure olduğunda çalışmaya devam ediyor mu? |
| Resilience | Failure sonrası toparlanabiliyor mu? |
| Recoverability | Failure sonrası ne kadar hızlı geri geliyor? |
| Partition Tolerance | Network partition sırasında sistem nasıl davranıyor? |
| Maintainability | Sistemi değiştirmek ne kadar kolay? |
| Operability | Sistemi production'da işletmek ne kadar kolay? |
| Security | Sistem kötü niyetli davranışlara karşı ne kadar güvenli? |
| Cost Efficiency | Bu davranış ne kadar maliyetli? |

Bu kavramların hepsini ayrı ayrı değil, **birbirleriyle ilişkili** düşünmelisin.

---

# 3. Availability

## 3.1 Nedir?

Availability:

> Sistem gerektiğinde request kabul edip kullanılabilir bir response verebiliyor mu?

Basit formül:

```text
Availability =
Uptime / Total Time
```

Örneğin:

```text
99%
99.9%
99.99%
99.999%
```

### "Nine" mantığı

Kabaca yıllık downtime:

| Availability | Yıllık downtime |
|---|---:|
| 99% | ~3.65 gün |
| 99.9% | ~8.76 saat |
| 99.99% | ~52.6 dakika |
| 99.999% | ~5.26 dakika |

Bir dokuz daha eklemek çok pahalı olabilir.

---

## 3.2 Availability nasıl artırılır?

Tek instance:

```text
Client
  |
  v
Server
```

Server öldü:

```text
Client
  X
Server
```

Redundancy:

```text
             +--> Server A
Client --> LB
             +--> Server B
```

Server A öldü:

```text
             +--> X
Client --> LB
             +--> Server B
```

Daha ileri:

```text
Region A
  +-- AZ1
  +-- AZ2
  +-- AZ3

Region B
  +-- AZ1
  +-- AZ2
  +-- AZ3
```

---

## 3.3 Availability = sadece replica sayısı değildir

Şu sistem:

```text
LB
 |
 +--> App A
 +--> App B
```

ama ikisi de:

```text
       |
       v
    DB single
```

DB öldüğünde sistem gider.

Bu yüzden:

> **Availability bir property değil, dependency graph'ın property'sidir.**

---

# 4. Reliability

Availability ile aynı değildir.

Availability:

> "Şu anda ulaşabiliyor muyum?"

Reliability:

> "Sistem doğru işi doğru şekilde yapıyor mu?"

Örneğin:

```text
HTTP 200
```

alıyorsun ama ödeme iki kere çekiliyor.

Sistem available olabilir.

Ama reliable değildir.

### Reliability örnekleri

- Duplicate payment oluşmaması
- Message'ın kaybolmaması
- Order'ın yanlış state'e geçmemesi
- Transaction'ın atomic olması

---

# 5. Scalability

Scalability:

> Workload arttığında sistemi büyüterek kabul edilebilir performance'ı koruyabilme yeteneği.

## 5.1 Vertical Scaling

```text
4 CPU / 16 GB
      ↓
32 CPU / 128 GB
```

Avantaj:

- Basit
- Daha az distributed complexity

Dezavantaj:

- Hardware limit
- Single-machine failure
- Büyük makineler pahalı

---

## 5.2 Horizontal Scaling

```text
1 server
   ↓
10 servers
   ↓
100 servers
```

Load balancer:

```text
             +--> App 1
             +--> App 2
Client --> LB+--> App 3
             +--> App 4
```

Avantaj:

- Büyük kapasite
- Failure isolation
- Elastic scaling

Dezavantaj:

- Distributed state
- Coordination
- Network calls
- Consistency problemleri

---

# 6. Elasticity

Scalability:

> "Büyüyebilir miyim?"

Elasticity:

> "Traffic arttığında veya azaldığında kaynakları otomatik ve verimli şekilde ayarlayabiliyor muyum?"

Örneğin:

```text
02:00 -> 5 pods
12:00 -> 50 pods
18:00 -> 100 pods
03:00 -> 5 pods
```

Elasticity özellikle variable workload için önemlidir.

---

# 7. Latency

Latency:

> Bir operation'ın başlaması ile tamamlanması arasındaki süre.

Örnek:

```text
Request -> 80 ms -> Response
```

Ama ortalama latency tek başına yeterli değildir.

---

# 8. Percentiles: p50, p95, p99, p99.9

Diyelim 1 milyon request var.

```text
p50 = 50 ms
p95 = 120 ms
p99 = 500 ms
p99.9 = 2 sec
```

p99:

> Request'lerin %99'u 500 ms'den hızlı.

Ama %1'i daha yavaş.

1 milyon request'te:

```text
1% = 10,000 request
```

yani 10.000 kullanıcı 500ms+ bekliyor.

### Neden average tehlikelidir?

```text
99 request = 10 ms
1 request  = 10 sec
```

Average yaklaşık:

```text
109.9 ms
```

gibi görünebilir.

Ama p99:

```text
10 sec
```

Gerçek kullanıcı deneyimini daha iyi gösterir.

---

# 9. Latency Budget

Bir endpoint:

```text
p99 < 300 ms
```

olsun.

Dependency'ler:

```text
API processing = 50ms
DB             = 100ms
Redis          = 20ms
Downstream     = 80ms
Network        = 30ms
```

Toplam:

```text
280ms
```

Artık sadece 20ms budget kaldı.

Bu yüzden:

> Latency requirement dependency budget'larına parçalanmalıdır.

---

# 10. Tail Latency

Distributed systems'de en önemli kavramlardan biridir.

Bir request:

```text
Service A
 |
 +--> B
 +--> C
 +--> D
```

Eğer A cevap vermek için hepsini bekliyorsa:

```text
Latency(A)
≈ max(Latency(B), Latency(C), Latency(D))
```

Bir dependency'nin tail latency'si tüm request'i etkiler.

Daha fazla parallel dependency:

```text
A
+--> B
+--> C
+--> D
+--> E
+--> F
```

tail latency riskini artırır.

---

# 11. Throughput

Throughput:

> Birim zamanda işlenen iş miktarı.

Örneğin:

```text
10,000 requests/sec
```

veya:

```text
50,000 messages/sec
```

Latency ve throughput aynı şey değildir.

### Analogy

Restoran:

```text
Latency = Bir müşterinin yemeğini alması için geçen süre
Throughput = Restoranın saatte servis ettiği müşteri sayısı
```

---

# 12. Little's Law

System Design'da çok faydalı bir relation:

```text
L = λW
```

Burada:

```text
L = system'deki ortalama concurrent work
λ = throughput
W = average latency
```

Örneğin:

```text
λ = 1,000 req/s
W = 200 ms = 0.2 sec
```

ise:

```text
L = 1000 × 0.2
  = 200 concurrent requests
```

Bu relation:

- connection pool
- worker pool
- queue
- concurrency

tasarımında çok işe yarar.

---

# 13. Durability

Durability:

> Başarıyla yazılmış verinin daha sonra kaybolmaması.

Örnek:

```text
Client -> DB: COMMIT
DB -> Client: success
```

Sonra server crash.

Data hâlâ varsa:

```text
durable
```

---

# 14. Durability nasıl sağlanır?

## Replication

```text
Primary
  |
  +--> Replica 1
  +--> Replica 2
```

## WAL

Write-Ahead Log:

```text
Request
  |
  v
WAL
  |
  v
Data files
```

Önce recovery için gerekli log güvenceye alınır.

## Backup

```text
DB
 |
 +--> snapshot
 +--> object storage
```

## Multi-region replication

```text
Region A
   |
   v
Region B
```

---

# 15. RPO ve RTO

Disaster Recovery için iki temel sayı.

## RPO — Recovery Point Objective

> Ne kadar veri kaybetmeyi kabul edebilirim?

Örneğin:

```text
RPO = 5 minutes
```

Disaster olursa son 5 dakikalık data kaybolabilir.

## RTO — Recovery Time Objective

> Sistem ne kadar sürede geri gelmeli?

```text
RTO = 10 minutes
```

---

# 16. RPO/RTO Trade-off

```text
Daha düşük RPO
    ↓
Daha fazla replication
    ↓
Daha fazla network/cost
```

```text
Daha düşük RTO
    ↓
Hot standby
    ↓
Daha fazla infrastructure cost
```

---

# 17. Consistency

Consistency:

> Distributed replicas veya readers farklı zamanlarda state'in hangi versiyonunu görebilir?

Basit sistem:

```text
Write -> DB
Read  -> DB
```

Distributed:

```text
        +--> Replica A
Write --+
        +--> Replica B
        +--> Replica C
```

Replication gecikirse:

```text
A = value 10
B = value 9
C = value 9
```

Bir kullanıcı 10 görürken diğeri 9 görebilir.

---

# 18. Strong Consistency

Write başarılı olduktan sonra subsequent read yeni değeri görür.

```text
WRITE X=10
    |
    v
ACK
    |
    v
READ X
    |
    v
10
```

Avantaj:

- Daha kolay mental model
- Finansal işlemlerde önemli olabilir

Dezavantaj:

- Coordination
- Latency
- Availability düşebilir
- Network partition sırasında zorlaşır

---

# 19. Eventual Consistency

Write sonrası replica'lar bir süre farklı olabilir.

```text
t0: primary = 10
t1: replica = 9
t2: replication
t3: replica = 10
```

Avantaj:

- Daha yüksek availability
- Daha düşük write latency
- Daha az coordination

Dezavantaj:

- Stale reads
- Daha karmaşık application semantics

Örnek kullanım:

- Like count
- View count
- Social feed
- Search index
- Analytics

---

# 20. Read-Your-Writes Consistency

Kullanıcı kendi yazdığı veriyi hemen görmelidir.

Örneğin:

```text
POST /profile
name = Alice

GET /profile
```

Kullanıcı `Alice` yerine eski value görüyorsa kötü UX.

Bunu çözmek için:

- Primary read
- Session stickiness
- Version token
- Read-after-write routing

kullanılabilir.

---

# 21. Monotonic Reads

Bir kullanıcı:

```text
10
```

gördükten sonra:

```text
9
```

görmemeli.

Bu distributed replica routing'de önemli olabilir.

---

# 22. CAP Theorem

CAP:

```text
C = Consistency
A = Availability
P = Partition Tolerance
```

Distributed system network partition yaşayabilir.

Partition sırasında:

```text
Node A  X  Node B
```

iletişim yok.

CAP şunu söyler:

> Partition olduğunda aynı anda hem güçlü consistency hem de availability garantisini tam olarak koruyamazsın.

---

# 23. CAP Yanlış Anlaşılmasın

CAP:

> "Consistency veya Availability'den istediğini seç."

demek değildir.

Daha doğru:

> **Network partition olduğunda C ile A arasında trade-off vardır.**

Çünkü distributed system'de P pratikte göz ardı edilemez.

---

# 24. PACELC

CAP'in ötesinde düşünmek için:

```text
If Partition:
    choose Availability vs Consistency

Else:
    choose Latency vs Consistency
```

Yani:

```text
P -> A/C
Else -> L/C
```

Bu çok önemli bir system design mental modelidir.

---

# 25. Quorum

Replica sayısı:

```text
N = 3
```

Write quorum:

```text
W = 2
```

Read quorum:

```text
R = 2
```

Eğer:

```text
W + R > N
```

ise read ve write quorumlarının kesişmesi garanti edilir.

Örneğin:

```text
N=3
W=2
R=2

2+2 > 3
```

Bu consistency stratejilerinin temel matematiğidir.

---

# 26. Replication

Replication:

> Aynı verinin birden fazla node'da tutulması.

Neden?

- Availability
- Read scaling
- Durability
- Failure tolerance

Ama:

> Replication free değildir.

Getirir:

- Network traffic
- Storage cost
- Replication lag
- Consistency complexity
- Failover complexity

---

# 27. Synchronous vs Asynchronous Replication

## Synchronous

```text
Client
 |
 v
Primary
 | \
 |  \
 v   v
R1   R2
 |
 ACK
```

ACK replication tamamlandıktan sonra gelir.

Daha durable/consistent.

Ama latency yüksek.

## Asynchronous

```text
Client
 |
 v
Primary
 |
 ACK
 |
 +----> Replica
```

Daha hızlı.

Ama primary ACK verdikten sonra crash olursa replica henüz update olmamış olabilir.

---

# 28. Sharding

Sharding:

> Dataset'i birden fazla node'a bölmek.

Örneğin:

```text
user_id 0-999
    -> shard A

1000-1999
    -> shard B
```

veya hash:

```text
shard = hash(user_id) % N
```

---

# 29. Sharding neden yapılır?

- Dataset tek node'a sığmıyor
- Write throughput yetmiyor
- Storage büyüyor
- Hotspot dağıtılmak isteniyor

---

# 30. Sharding Trade-offs

Avantaj:

```text
capacity ↑
write throughput ↑
parallelism ↑
```

Dezavantaj:

```text
cross-shard query
cross-shard transaction
rebalancing
hot shard
operational complexity
```

---

# 31. Consistent Hashing

Normal:

```text
hash(key) % N
```

N değişince çok fazla key taşınabilir.

Örneğin:

```text
N=3
```

den:

```text
N=4
```

'e geçince birçok key'in shard'ı değişir.

Consistent hashing:

```text
        hash ring

       A
    /     \
   D       B
    \     /
       C
```

Node'lar ring üzerinde tutulur.

Key, clockwise yönde ilk node'a atanır.

Node ekleme/çıkarma durumunda sadece belirli bir key range etkilenir.

---

# 32. Virtual Nodes

Consistent hashing'de:

```text
Physical Node A
 -> vnode 1
 -> vnode 2
 -> vnode 3
 -> ...
```

kullanılır.

Amaç:

- Daha iyi dağılım
- Hotspot azaltma
- Node capacity farklarını yönetebilme

---

# 33. Hotspot

Teoride dağılım dengeli olabilir.

Pratikte:

```text
user A = 80% traffic
user B = 0.001%
```

gibi skew olabilir.

Bir shard:

```text
100% CPU
```

diğerleri:

```text
10% CPU
```

olabilir.

Bu:

> **Hot shard / hot key**

problemidir.

Çözümler:

- Better partition key
- Key salting
- Replication
- Request coalescing
- Caching
- Load-aware routing

---

# 34. Caching

Cache:

> Pahalı veya sık kullanılan verinin daha hızlı erişilen bir yerde tutulması.

```text
Client
 |
 v
Cache
 |
 +--> HIT -> response
 |
 MISS
 |
 v
DB
```

---

# 35. Cache Hit Ratio

```text
hit ratio =
cache hits / total requests
```

Örneğin:

```text
900 hits
100 misses
```

= 90%.

Cache hit ratio architecture'ı ciddi etkiler.

---

# 36. Cache Invalidation

Ünlü problem:

> Cache invalidation zor bir problemdir.

Stratejiler:

## Cache Aside

```text
read
 |
cache?
 |
 +--> hit
 |
 +--> miss -> DB -> cache
```

Write:

```text
DB update
   |
delete cache
```

## Write Through

```text
write
 |
cache
 |
DB
```

## Write Behind

```text
write
 |
cache
 |
ACK

later
 |
DB
```

Daha hızlı olabilir ama durability complexity getirir.

---

# 37. Cache Stampede

Cache expire olduğunda:

```text
10,000 requests
      |
      v
cache miss
      |
      +--> DB
      +--> DB
      +--> DB
      ...
```

DB patlayabilir.

Çözümler:

- Request coalescing
- Singleflight
- Locking
- Early refresh
- Jittered TTL
- Stale-while-revalidate

---

# 38. Cache Penetration

Olmayan key'ler sürekli soruluyor:

```text
GET missing-key
GET missing-key
GET missing-key
...
```

Cache'de yok.

DB'ye sürekli gidiyor.

Çözüm:

```text
negative caching
```

veya Bloom filter gibi mekanizmalar.

---

# 39. Cache Avalanche

Birçok key aynı anda expire olur:

```text
10:00
 |
 +--> millions of keys expire
 |
 v
DB traffic spike
```

Çözüm:

```text
TTL + random jitter
```

---

# 40. Bloom Filter

Bloom filter:

> Bir item'ın set içinde bulunmadığını hızlı şekilde söyleyebilen probabilistic data structure.

İki cevap:

```text
Definitely NOT present
Probably present
```

False positive olabilir.

False negative olmamalıdır.

Kullanım:

```text
Request
 |
Bloom Filter
 |
 +--> definitely absent -> reject
 |
 +--> maybe present -> DB
```

---

# 41. Queue

Queue:

> Producer ile consumer arasına asynchronous buffer koyar.

```text
Producer
   |
   v
Queue
   |
   v
Consumer
```

Neden?

- Traffic smoothing
- Decoupling
- Retry
- Async processing
- Backpressure

---

# 42. Queue ile Traffic Smoothing

Traffic:

```text
████████████████████
```

Consumer capacity:

```text
████████
```

Queue:

```text
████████████████
       ↓
consumer slowly drains
```

Queue burst'i absorbe eder.

Ama queue sonsuz değildir.

---

# 43. Queue Backlog

Queue:

```text
incoming = 10k/sec
processing = 5k/sec
```

Backlog:

```text
+5k/sec
```

Uzun vadede sistem çöker.

Bu nedenle:

> Queue scalability problemi çözmez; bazı durumlarda sadece problemi zamana yayar.

---

# 44. Backpressure

Backpressure:

> Downstream kapasitesi yetmediğinde upstream'in hızını sınırlamak.

```text
Producer
   |
   v
Queue
   |
   v
Consumer
```

Consumer yavaşsa producer'ın:

```text
slow down
block
reject
shed load
```

etmesi gerekebilir.

---

# 45. Load Shedding

Sistem kapasitesini aşarsa her request'i kabul etmeye çalışmak yerine bazılarını kontrollü reddetmek.

Örneğin:

```text
CPU > 90%
queue > threshold
```

ise:

```text
429 Too Many Requests
503 Service Unavailable
```

döndürmek.

Amaç:

> Sistemin tamamını öldürmek yerine kontrollü şekilde bir kısmını feda etmek.

---

# 46. Bulkhead Pattern

Bulkhead:

> Bir failure'ın sistemin tamamına yayılmasını engellemek için kaynakları izole etmek.

Örneğin:

```text
              API
               |
       +-------+-------+
       |               |
    Payments         Search
       |               |
    pool A            pool B
```

Search patlarsa Payments'ın worker/connection pool'unu tüketmemeli.

---

# 47. Bounded Worker Pool vs Semaphore vs Bulkhead

Bunları karıştırmamak önemli.

## Semaphore

```text
50,000 goroutine
       |
       v
 semaphore = 20
```

Aynı anda 20 operation çalışır.

Ama 50k goroutine yaratılmış olabilir.

## Worker Pool

```text
50,000 jobs
      |
      v
20 workers
```

Concurrency bounded'dır.

## Bulkhead

Daha geniş bir pattern'dir:

```text
Payment resources
Search resources
Email resources
```

birbirinden izole edilir.

Worker pool veya semaphore bulkhead'in implementation araçlarından biri olabilir.

---

# 48. Rate Limiting

Bir client'ın veya tenant'ın ne kadar traffic gönderebileceğini sınırlar.

Algoritmalar:

## Fixed Window

```text
12:00-12:01
max = 100
```

Basit ama boundary burst problemi vardır.

## Sliding Window

Daha düzgün.

## Token Bucket

Bucket tokenlarla dolar.

Request:

```text
1 token
```

tüketir.

Token yoksa request reject/wait.

Avantaj:

- Burst allowance
- Average rate control

## Leaky Bucket

Sabit çıkış rate'i gibi davranır.

---

# 49. Idempotency

Operation tekrarlandığında sonuç aynı olmalı veya duplicate side effect oluşmamalı.

Örneğin:

```text
POST /payment
Idempotency-Key: abc123
```

Client timeout yaşadı.

Request tekrar gönderildi.

Server:

```text
abc123 -> already processed
```

ise ikinci payment çekilmez.

---

# 50. Idempotency neden önemlidir?

Network güvenilir değildir.

Şu olabilir:

```text
Client -> Server
          |
          | payment succeeds
          |
          X response lost
```

Client:

```text
"retry"
```

diyor.

Server duplicate operation yaparsa:

```text
charge #1
charge #2
```

olabilir.

---

# 51. Exactly Once: Dikkat

"Exactly once delivery" çok sık yanlış kullanılır.

Distributed system'de:

```text
network
retry
crash
timeout
```

yüzünden gerçek dünyada end-to-end exactly-once semantics elde etmek zordur.

Çoğu sistem:

```text
at-least-once delivery
+
idempotent consumer
```

yaklaşımını kullanır.

---

# 52. Delivery Semantics

## At-most-once

```text
send once
```

Duplicate yok.

Ama message kaybolabilir.

## At-least-once

Retry yapılır.

Message kaybolması azaltılır.

Ama duplicate olabilir.

## Exactly-once

Her message effect'i tam bir kez uygulanmış gibi görünür.

Bunu sağlamak ciddi coordination/idempotency gerektirir.

---

# 53. Ordering

Bazı sistemlerde sıra önemlidir.

Örneğin:

```text
balance = 100

withdraw 50
deposit 20
```

işlem sırası değişirse sonuç değişebilir.

Ordering scope tanımlanmalıdır:

```text
global ordering
partition ordering
per-user ordering
per-account ordering
```

Çoğu sistemde global ordering yerine:

> **key bazlı ordering**

daha ölçeklenebilirdir.

---

# 54. Partitioning ve Ordering İlişkisi

Örneğin:

```text
account_id = 123
```

tüm event'leri aynı partition'a koyarsan:

```text
partition 7
```

üzerinde sıralı işleyebilirsin.

Ama:

```text
all accounts -> one partition
```

yaparsan global ordering elde edersin fakat throughput düşer.

Trade-off:

```text
stronger ordering
      ↕
scalability
```

---

# 55. Distributed Transactions

Bir transaction birden fazla service'i kapsıyorsa:

```text
Order Service
Payment Service
Inventory Service
```

tek local DB transaction kullanamazsın.

Yaklaşımlar:

- Two-Phase Commit
- Saga
- Outbox
- Event-driven workflow
- Compensation

---

# 56. Two-Phase Commit

```text
Coordinator
   |
   +--> DB A: prepare
   +--> DB B: prepare
   |
   +--> commit
```

Atomicity güçlüdür.

Ama:

- Coordination cost
- Blocking
- Failure complexity
- Availability impact

---

# 57. Saga

Distributed transaction'ı local transaction'lara bölersin.

```text
Create Order
    |
    v
Reserve Inventory
    |
    v
Charge Payment
    |
    v
Confirm Order
```

Bir adım fail olursa compensation:

```text
Charge Payment
    |
    X
    |
refund
```

Saga:

> Distributed transaction'ı business-level state machine olarak düşünmektir.

---

# 58. Outbox Pattern

En önemli reliability pattern'lerinden biridir.

Problem:

```text
DB transaction
    +
publish event
```

aynı anda atomic değil.

Kötü:

```text
DB commit
  |
  X
Kafka publish fails
```

DB state değişti ama event yok.

Outbox:

```text
BEGIN TRANSACTION

UPDATE orders
INSERT INTO outbox

COMMIT
```

Sonra publisher:

```text
Outbox
   |
   v
Kafka
```

Bu sayede DB state ve event intent aynı transaction'da durable olur.

---

# 59. Inbox / Deduplication

Consumer tarafında:

```text
message_id = abc
```

işlenmiş mi?

```text
processed_messages
```

tablosunda tutulabilir.

Böylece at-least-once delivery:

```text
duplicate message
       |
       v
dedupe
       |
       v
no duplicate side effect
```

---

# 60. Leader Election

Bir distributed system'de bir node coordinator olsun:

```text
Node A
Node B
Node C
```

Leader:

```text
A = leader
B = follower
C = follower
```

A ölür:

```text
B = leader
```

Leader election için consensus/coordination mekanizmaları gerekir.

---

# 61. Consensus

Consensus:

> Birden fazla node'un distributed state konusunda anlaşmasını sağlar.

Önemli algoritmalar:

- Raft
- Paxos

Consensus zor bir problemdir çünkü:

- Node crash
- Network delay
- Network partition
- Message loss
- Duplicate message
- Reordering

olabilir.

---

# 62. Raft Mental Model

Raft'ta:

```text
Leader
  |
  +--> Follower
  +--> Follower
```

Leader log entry alır:

```text
set x=10
```

followers'a replicate eder.

Majority:

```text
3 node -> 2
5 node -> 3
7 node -> 4
```

commit için önemlidir.

---

# 63. Failure Detector

Distributed system'de:

> Bir node'un gerçekten öldüğünü mü, yoksa sadece network'te gecikme olduğunu mu biliyorsun?

Çoğu durumda kesin olarak bilemezsin.

Bu nedenle:

```text
timeout
heartbeat
lease
health check
```

gibi failure detector mekanizmaları kullanılır.

Ama timeout:

> "Kesin öldü"

demek değildir.

Sadece:

> "Beklediğim sürede cevap vermedi"

demektir.

Bu ayrım çok önemlidir.

---

# 64. Split Brain

İki node kendini leader sanarsa:

```text
Leader A
Leader B
```

ikisi de write kabul edebilir.

Bu:

> split brain

problemidir.

Çözümler:

- Quorum
- Fencing
- Leases
- Consensus
- Epoch/term number

---

# 65. Fencing Token

Her leader'a monoton artan token verilir:

```text
Leader A -> token 10
Leader B -> token 11
```

Storage:

```text
token < 11
```

olan eski leader'ın write'ını reddeder.

Bu stale leader problemine karşı çok güçlü bir tekniktir.

---

# 66. Time ve Distributed Systems

Saatlere güvenmek tehlikelidir.

Node A:

```text
10:00:00
```

Node B:

```text
09:59:59
```

clock skew olabilir.

Bu yüzden ordering için sadece wall-clock timestamp'e güvenmek risklidir.

Alternatifler:

- Logical clocks
- Lamport clocks
- Vector clocks
- Sequence numbers
- Consensus terms

---

# 67. Lamport Clock

Basit mental model:

```text
counter = logical time
```

Event olduğunda artır.

Message gönderirken timestamp taşı.

Receive ederken:

```text
counter = max(local, received) + 1
```

Ama Lamport clock:

> Causality hakkında partial information verir; tüm event'lerin gerçek global sırasını vermez.

---

# 68. Vector Clock

Her node kendi counter'ını tutar:

```text
A = [2,0,0]
B = [2,3,0]
C = [2,3,1]
```

Concurrent event'leri tespit etmeye yardımcı olur.

Ama node sayısı arttıkça metadata büyür.

---

# 69. Networking Gerçeği

Distributed system'in en temel varsayımı:

> Network güvenilir değildir.

Şunlar olabilir:

- Packet loss
- Delay
- Duplication
- Reordering
- Connection reset
- Partial failure
- DNS failure
- TLS failure
- Load balancer failure

Bu nedenle network call:

```text
function call()
```

gibi değil:

```text
remote operation with uncertainty
```

gibi düşünülmelidir.

---

# 70. Timeout

Her network dependency'nin timeout'u olmalıdır.

Kötü:

```text
wait forever
```

İyi:

```text
deadline = 500ms
```

Timeout:

> Failure handling mekanizmasının temelidir.

Ama timeout çok kısa olursa:

```text
false timeout
retry storm
```

oluşturabilir.

---

# 71. Retry

Transient failure'da tekrar denemek.

Ama:

> Retry bir amplification mechanism'dir.

100 request fail oldu.

Her biri 3 retry:

```text
100 -> 300 requests
```

Downstream zaten zor durumdaysa daha da kötüleşebilir.

---

# 72. Exponential Backoff

Retry aralıkları:

```text
100ms
200ms
400ms
800ms
1600ms
```

gibi artar.

---

# 73. Jitter

Aynı anda başlayan request'ler aynı retry schedule kullanırsa:

```text
100ms -> everyone retries
200ms -> everyone retries
400ms -> everyone retries
```

thundering herd oluşabilir.

Jitter:

```text
base delay + random component
```

kullanır.

---

# 74. Circuit Breaker

Dependency sürekli fail ediyorsa her request'i ona göndermeye devam etme.

State:

```text
CLOSED
  |
  | failures
  v
OPEN
  |
  | wait
  v
HALF-OPEN
```

OPEN:

```text
fail fast
```

HALF-OPEN:

```text
few test requests
```

başarılıysa CLOSED.

---

# 75. Retry + Circuit Breaker

Birlikte düşünülmelidir.

```text
Request
   |
 timeout
   |
 retry with backoff
   |
 repeated failure
   |
 circuit opens
   |
 fail fast
```

Ama yanlış configuration:

```text
10 retries
+
5 services
+
fan-out
```

çok büyük amplification yaratabilir.

---

# 76. Hedged Requests

Tail latency çok yüksekse aynı request'i bir süre sonra başka replica'ya da gönderebilirsin.

```text
request
 |
 +--> replica A
 |
 +-- after 50ms --> replica B
```

İlk cevap kazanır.

Ama:

> Extra traffic üretir.

Bu nedenle sadece tail latency problemi varsa düşünülmelidir.

---

# 77. Graceful Degradation

Dependency yoksa bütün endpoint'i öldürmek yerine daha az özellik sun.

Örneğin:

```text
Recommendation service DOWN
        |
        v
Feed still works
        |
        v
recommendations omitted
```

Bu:

> Availability vs feature completeness

trade-off'udur.

---

# 78. Fail Open vs Fail Closed

Security/validation context'inde:

## Fail Closed

Failure durumunda erişimi reddet.

```text
auth service down
   |
   v
deny
```

Daha güvenli.

## Fail Open

Failure durumunda izin ver.

```text
auth service down
   |
   v
allow
```

Availability daha yüksek olabilir ama security risklidir.

Hangi seçimin doğru olduğu domain'e bağlıdır.

---

# 79. Health Checks

İki önemli tür:

## Liveness

> Process yaşıyor mu?

## Readiness

> Traffic almaya hazır mı?

Bir service process olarak canlı olabilir:

```text
liveness = OK
```

ama DB connection yoksa:

```text
readiness = FAIL
```

Bu distinction deployment'ta kritiktir.

---

# 80. Autoscaling

Scaling signal'ları:

- CPU
- Memory
- Requests/sec
- Queue depth
- Latency
- Custom business metrics

CPU her zaman doğru signal değildir.

Örneğin async worker:

```text
CPU = 20%
queue = 1,000,000
```

burada CPU'ya bakarak scale etmek anlamsız olabilir.

Queue depth daha iyi signal olabilir.

---

# 81. Queue-Based Autoscaling

```text
queue depth
    |
    v
desired workers
```

Örneğin:

```text
0-100 jobs    -> 2 workers
100-1000      -> 10 workers
1000-10000    -> 50 workers
```

Ama maksimum worker sınırı olmalıdır.

Yoksa downstream overload oluşabilir.

---

# 82. Connection Pool

DB connection pool:

```text
App
 |
 +--> connection 1
 +--> connection 2
 +--> ...
 +--> connection N
```

Pool çok küçükse:

```text
requests wait
```

Pool çok büyükse:

```text
DB overload
```

Yani:

> Daha fazla connection her zaman daha fazla throughput değildir.

---

# 83. Queueing ve Saturation

Bir resource:

```text
CPU
DB
connection pool
worker pool
network
```

saturation'a yaklaştıkça latency genellikle dramatik şekilde artar.

Mental model:

```text
utilization ↑
       |
       v
queueing ↑
       |
       v
latency ↑↑
```

Bu yüzden %100 utilization hedefi çoğu latency-sensitive sistemde kötü fikirdir.

---

# 84. Resource Isolation

Aynı resource pool'u her workload paylaşırsa:

```text
critical traffic
+
bulk traffic
```

bulk traffic critical traffic'i boğabilir.

Çözüm:

```text
critical pool
bulk pool
```

Bu bulkhead düşüncesidir.

---

# 85. API Design

System design'da API sadece endpoint listesi değildir.

Düşün:

- Resource model
- Idempotency
- Pagination
- Filtering
- Sorting
- Versioning
- Error model
- Rate limiting
- Authentication
- Authorization
- Timeout semantics
- Retry semantics

---

# 86. Pagination

Offset pagination:

```text
?page=100
```

basittir.

Ama dataset değişirken:

- duplicate
- missing items
- large offset cost

problemleri olabilir.

Cursor pagination:

```text
?cursor=abc
```

daha stable olabilir.

Özellikle büyük datasetlerde cursor pagination tercih edilir.

---

# 87. Offset vs Cursor

| | Offset | Cursor |
|---|---|---|
| Basitlik | + | - |
| Stable pagination | - | + |
| Large dataset | - | + |
| Random page | + | - |
| Mutable data | sorunlu olabilir | daha iyi |

---

# 88. Database Index

Index:

> Query'nin ihtiyaç duyduğu row'ları daha hızlı bulmasını sağlayan data structure.

Ama index bedava değildir.

Read:

```text
faster
```

Write:

```text
more expensive
```

Storage:

```text
more
```

Bu yüzden:

> Her column'a index koymak iyi değildir.

---

# 89. B-Tree Mental Model

Çoğu relational DB index'i için:

```text
          root
        /      \
      node     node
     /  \      /  \
   data data data data
```

Sorted access sağlar.

İyi olduğu şeyler:

- Equality
- Range
- Ordering

---

# 90. Composite Index

Query:

```sql
WHERE tenant_id = ?
AND created_at > ?
ORDER BY created_at
```

için:

```text
(tenant_id, created_at)
```

index'i çok faydalı olabilir.

Column order önemlidir.

---

# 91. N+1 Query

Kötü:

```text
SELECT users
```

sonra her user için:

```text
SELECT orders WHERE user_id = ?
```

100 user:

```text
101 queries
```

Çözümler:

- Join
- Batch query
- Preload
- DataLoader
- Denormalization

---

# 92. Normalization vs Denormalization

Normalization:

```text
less duplication
more joins
```

Denormalization:

```text
more duplication
faster reads
```

Trade-off:

```text
write complexity
      ↕
read performance
```

---

# 93. SQL vs NoSQL

SQL:

Avantaj:

- Strong schema
- Transactions
- Joins
- Mature query language

NoSQL:

Avantaj:

- Flexible schema
- Horizontal scale için bazı workload'larda daha kolay
- Access-pattern driven design

Yanlış soru:

> "Hangisi daha iyi?"

Doğru soru:

> "Bu workload'un access pattern'leri ve consistency ihtiyaçları için hangisi daha uygun?"

---

# 94. Single Source of Truth

Bir state için birden fazla authoritative system varsa:

```text
DB A = 10
DB B = 11
Cache = 9
Search = 10
```

hangisi doğru?

Architecture'da:

> Source of truth açıkça tanımlanmalıdır.

Örneğin:

```text
Postgres = source of truth
Redis = cache
Elasticsearch = read model/index
Kafka = event transport
```

---

# 95. CQRS

Command Query Responsibility Segregation:

```text
Writes
  |
  v
Write Model

Events
  |
  v

Read Model
  |
  v
Queries
```

Read-heavy workloadlarda faydalı olabilir.

Ama:

- Eventual consistency
- More storage
- More operational complexity

getirir.

---

# 96. Event-Driven Architecture

```text
Service A
   |
 publish
   v
Event Bus
   |
 +---+---+
 v       v
B       C
```

Avantaj:

- Loose coupling
- Async processing
- Independent consumers
- Replay possibilities

Dezavantaj:

- Debugging
- Ordering
- Duplicate events
- Eventual consistency
- Schema evolution

---

# 97. Event Schema Evolution

Event:

```json
{
  "user_id": "123",
  "email": "a@b.com"
}
```

sonra:

```json
{
  "user_id": "123",
  "email": "a@b.com",
  "country": "TR"
}
```

Consumer eski schema ile çalışmaya devam edebilmeli.

Bu nedenle:

> Backward compatibility çok önemlidir.

---

# 98. At-Least-Once + Idempotency

Çok yaygın production pattern:

```text
Producer
   |
   v
Queue
   |
   v
Consumer
   |
process
   |
ack
```

Consumer process ettikten önce crash olursa:

```text
message redelivered
```

Bu yüzden:

```text
message_id
    |
deduplication
    |
idempotent side effect
```

gerekebilir.

---

# 99. Distributed Lock

Bir resource üzerinde tek owner gerekebilir.

Ama distributed lock risklidir:

- Lease expiration
- Clock issues
- Network partition
- Stale owner
- Lock holder crash

Daha güvenli yaklaşım çoğu zaman:

```text
lease + fencing token
```

gibi mekanizmalardır.

---

# 100. Leader Lease

Leader:

```text
lease expires at T
```

Renew:

```text
heartbeat
```

Network problemi olursa lease expire olur.

Ama stale leader'ın eski write yapmasını önlemek için fencing gerekebilir.

---

# 101. Security as a System Property

Security sonradan eklenen bir middleware değildir.

Düşün:

```text
Authentication
Authorization
Encryption
Secrets
Network isolation
Rate limiting
Audit logging
Input validation
Abuse prevention
Data minimization
```

---

# 102. Authentication vs Authorization

Authentication:

> Kimsin?

Authorization:

> Bunu yapmaya yetkin var mı?

Örnek:

```text
JWT valid
```

authentication olabilir.

Ama:

```text
user A -> access user B's payment
```

authorization failure'dır.

---

# 103. Observability

Üç ana pillar:

```text
Logs
Metrics
Traces
```

## Logs

"Ne oldu?"

## Metrics

"Ne sıklıkta / ne kadar?"

## Traces

"Request nerelerden geçti?"

---

# 104. RED Method

Request-driven systems:

```text
Rate
Errors
Duration
```

Örneğin:

```text
RPS = 20k
Error rate = 0.5%
p99 = 800ms
```

---

# 105. USE Method

Resource-oriented systems:

```text
Utilization
Saturation
Errors
```

Örneğin DB:

```text
CPU utilization = 70%
connection saturation = 95%
errors = 0.1%
```

---

# 106. SLI, SLO, SLA

## SLI

Ölçtüğün şey.

```text
request success rate
```

## SLO

Hedef.

```text
99.95% successful requests
```

## SLA

Müşteriyle yapılan sözleşmesel taahhüt.

---

# 107. Error Budget

SLO:

```text
99.9%
```

ise:

```text
0.1%
```

failure budget'in vardır.

Bu:

> Engineering velocity ile reliability arasında trade-off yönetmek için kullanılır.

---

# 108. Monitoring vs Observability

Monitoring:

> Önceden bildiğin failure'ları detect etmek.

Observability:

> Sistemin internal state'ini external outputs üzerinden anlayabilmek.

Örneğin bilinmeyen bir latency regression'ı trace + metrics + logs ile debug etmek.

---

# 109. Disaster Recovery

Failure levels:

```text
Process failure
Instance failure
AZ failure
Region failure
Data corruption
Operator mistake
Security incident
```

Her biri farklı recovery strategy gerektirir.

---

# 110. Backup Strategy

Backup çeşitleri:

- Full backup
- Incremental backup
- Snapshot
- WAL/archive log
- Cross-region copy

Backup'ın olması yeterli değildir.

> **Restore test edilmelidir.**

"Backup var" ile:

"Backup'tan 15 dakika içinde restore edebiliyoruz"

aynı şey değildir.

---

# 111. Multi-AZ vs Multi-Region

Multi-AZ:

```text
Region
 +-- AZ1
 +-- AZ2
 +-- AZ3
```

Genellikle:

- High availability
- Lower latency
- Daha düşük complexity

Multi-region:

```text
Region A <--> Region B
```

Daha fazla:

- Disaster tolerance
- Geographic latency optimization

ama çok daha fazla:

- Consistency
- Replication
- Routing
- Failover
- Data residency
- Cost

complexity getirir.

---

# 112. Active-Passive

```text
Region A = active
Region B = standby
```

Failover:

```text
A fails
 |
 v
B becomes active
```

Basit.

Ama standby resource'ları atıl olabilir.

---

# 113. Active-Active

```text
Region A <--> Region B
```

ikisi de traffic alır.

Daha iyi resource utilization.

Ama:

- Conflict resolution
- Global consistency
- Routing
- Duplicate processing

daha zordur.

---

# 114. DNS Failover

Traffic:

```text
api.example.com
       |
       v
DNS
  +--> Region A
  +--> Region B
```

Ama DNS caching nedeniyle failover anında gerçekleşmeyebilir.

Bu yüzden:

> DNS TTL = failover time değildir.

---

# 115. CDN

CDN:

> Static veya cacheable content'i kullanıcıya yakın edge location'dan sunar.

```text
User
 |
 v
Edge
 |
 +--> HIT
 |
 +--> origin
```

Latency azaltır ve origin load'unu düşürür.

---

# 116. CDN Cache Key

Cache key:

```text
scheme + host + path + query + selected headers
```

gibi bileşenlere bağlı olabilir.

Yanlış cache key:

```text
User A's private response
        |
        v
cached
        |
        v
User B
```

çok ciddi security problemidir.

---

# 117. Data Locality

Data user'a yakınsa:

```text
User -> nearby region -> DB
```

latency düşebilir.

Ama global replication consistency complexity getirir.

---

# 118. Fan-out

Bir request:

```text
A
+--> B
+--> C
+--> D
+--> E
```

Fan-out arttıkça:

- Latency
- Failure probability
- Connection usage
- Retry amplification

artabilir.

---

# 119. Fan-in

Birçok producer tek dependency'ye yükleniyorsa:

```text
A \
B  \
C ---> DB
D  /
E /
```

DB bottleneck olur.

Scaling sadece upstream'e yapılırsa:

```text
more app servers
```

sorunu büyütebilir.

---

# 120. Little's Law ile Queue Capacity

Örneğin:

```text
incoming = 1000/sec
consumer = 800/sec
```

net backlog:

```text
200/sec
```

10 dakika sonra:

```text
200 * 600 = 120,000 jobs
```

Bu nedenle queue depth'in trend'i önemlidir.

---

# 121. Bottleneck Thinking

Bir system design'da önce:

> En zayıf link neresi?

diye sor.

Örnek:

```text
100 app servers
      |
      v
10 DB connections
```

App scale etmek DB bottleneck'i çözmez.

---

# 122. Capacity Planning

Her component için:

```text
capacity
load
headroom
```

hesapla.

Örneğin:

```text
1 server = 500 req/s

expected = 5,000 req/s

minimum = 10 servers
```

Ama production:

```text
headroom = 30%
```

istiyorsan:

```text
~14-15 servers
```

gibi düşün.

---

# 123. Peak Traffic

Average traffic ile tasarım yapmak tehlikelidir.

Örneğin:

```text
average = 10k RPS
peak    = 50k RPS
```

System 10k'ya göre tasarlanırsa peak'te:

```text
queue
latency
errors
```

patlayabilir.

---

# 124. Traffic Estimation

System design interview'da kaba hesap yeterlidir.

Örneğin:

```text
100M users
10% daily active
10 requests/user/day
```

DAU:

```text
10M
```

requests/day:

```text
100M
```

average RPS:

```text
100M / 86,400
≈ 1,157 RPS
```

Peak 10x:

```text
≈ 11.6k RPS
```

---

# 125. Storage Estimation

Örneğin:

```text
10M events/day
1 KB/event
```

Daily:

```text
10 GB/day
```

Yearly:

```text
~3.65 TB/year
```

Replication 3x:

```text
~10.95 TB/year
```

Index/metadata/overhead dahil değildir.

---

# 126. Bandwidth Estimation

```text
RPS × response size
```

Örneğin:

```text
10k RPS
×
100 KB
=
1 GB/s
```

Bu çok büyük bir bandwidth'tir.

Bu noktada:

- CDN
- Compression
- Pagination
- Smaller payload
- Binary protocol

düşünülür.

---

# 127. Compression Trade-off

Compression:

```text
network bandwidth ↓
storage ↓
```

ama:

```text
CPU ↑
latency bazı durumlarda ↑
```

CPU constrained system'de compression her zaman doğru çözüm değildir.

---

# 128. Serialization

JSON:

- Human readable
- Easy
- Larger payload

Binary formats:

- Smaller
- Faster bazı workload'larda
- Schema management complexity

Seçim workload'a göre yapılır.

---

# 129. Synchronous vs Asynchronous

Synchronous:

```text
request
 |
 v
dependency
 |
 v
response
```

Avantaj:

- Simple semantics

Dezavantaj:

- Latency coupling
- Failure propagation

Async:

```text
request
 |
 v
queue
 |
 v
worker
```

Avantaj:

- Decoupling
- Burst absorption

Dezavantaj:

- Eventual consistency
- More operational complexity

---

# 130. When to Use Async?

Eğer operation:

- Kullanıcı response'unu beklemek zorunda değilse
- Uzun sürüyorsa
- Retry gerekliyse
- Burst workload varsa

async iyi adaydır.

Örnek:

```text
POST /video
```

response:

```text
202 Accepted
job_id = 123
```

sonra:

```text
GET /jobs/123
```

---

# 131. Polling vs Push

Polling:

```text
client -> server
"anything new?"
```

Basit ama boş request üretir.

Push:

```text
server -> client
event
```

Daha efficient olabilir.

Alternatifler:

- WebSocket
- SSE
- Long polling

---

# 132. WebSocket

Long-lived bidirectional connection.

İyi kullanım:

- Chat
- Live collaboration
- Gaming
- Real-time updates

Trade-off:

- Connection state
- Load balancing
- Connection draining
- Reconnect logic
- Backpressure

---

# 133. Stateless vs Stateful

Stateless app:

```text
Request A -> server 1
Request B -> server 2
```

çalışabilir.

State externalized:

```text
App
 |
 +--> DB
 +--> Redis
```

Scaling kolaylaşır.

Stateful app:

```text
session lives on server
```

routing/stickiness complexity getirir.

---

# 134. Sticky Sessions

Client hep aynı instance'a gider.

Avantaj:

- Stateful app kolaylaşır.

Dezavantaj:

- Uneven load
- Failover
- Scaling complexity

Genellikle state'i externalize etmek daha scalable'dır.

---

# 135. Consistent Routing vs Random Routing

Random load balancing:

```text
A/B/C equally
```

Sticky/consistent hashing:

```text
user X -> same server
```

Session/cache locality için kullanılabilir.

---

# 136. Load Balancing Algorithms

## Round Robin

```text
A B C A B C
```

Basit.

## Least Connections

En az active connection'a gönder.

## Weighted

```text
A = 2x capacity
B = 1x
```

## Consistent Hashing

Aynı key'i aynı node'a yönlendirmeye yardımcı olur.

---

# 137. Retry Storm

Dependency yavaş:

```text
requests
   |
 timeout
   |
 retry
   |
 more load
   |
 more timeout
   |
 more retry
```

Feedback loop oluşur.

Bu:

> Positive feedback failure loop

gibi düşünülebilir.

Çözümler:

- Exponential backoff
- Jitter
- Retry budget
- Circuit breaker
- Load shedding
- Deadline propagation

---

# 138. Deadline Propagation

Request:

```text
deadline = 500ms
```

A:

```text
remaining = 450ms
```

B:

```text
remaining = 300ms
```

DB:

```text
remaining = 150ms
```

Her dependency:

> Kalan budget'ı bilmeli.

Bu, timeout'ların birbirinden bağımsız rastgele seçilmesinden daha sağlıklıdır.

---

# 139. Retry Budget

Bir request'in total latency budget'i:

```text
500ms
```

ise:

```text
retry #1
retry #2
retry #3
```

yaparken toplam budget aşılmamalıdır.

Retry:

> Yeni bir request değil, aynı user intent'in continuation'ıdır.

---

# 140. Failure Domains

Failure'ı şu seviyelerde düşün:

```text
process
instance
host
rack
AZ
region
provider
network
database
operator
data
```

Her level için redundancy düşün.

---

# 141. Blast Radius

Bir failure kaç şeyi etkiliyor?

```text
One worker dies
    |
    v
one request

One DB dies
    |
    v
whole service

One region dies
    |
    v
global users
```

Amaç:

> Failure'ın blast radius'unu küçültmek.

Bulkhead, sharding, multi-AZ gibi pattern'ler burada önemlidir.

---

# 142. Cell Architecture

Büyük sistemi bağımsız hücrelere böl:

```text
Cell 1
 users A-D

Cell 2
 users E-H

Cell 3
 users I-L
```

Cell 1 fail olursa tüm sistem değil belirli bir segment etkilenir.

Bu:

> Blast radius reduction

için güçlü bir mimaridir.

---

# 143. Retry vs Duplicate Side Effects

Bir request retry ediliyorsa operation idempotent olmalı.

```text
GET
```

genelde doğal olarak idempotent.

Ama:

```text
POST /charge
```

değildir.

Idempotency key eklemek gerekir.

---

# 144. Data Consistency ve Business Consistency

Teknik olarak:

```text
DB replicas consistent
```

olabilir.

Ama business invariant bozulabilir:

```text
inventory = -1
```

Bu nedenle:

> Distributed system design'da teknik consistency kadar **business invariants** düşünülmelidir.

Örnek:

```text
balance >= 0
order cannot be both CANCELLED and SHIPPED
payment cannot be charged twice
```

---

# 145. Invariants

System design'a başlamadan önce:

> "Hangi şeyler asla yanlış olamaz?"

listesi çıkar.

Örneğin payment:

```text
money cannot be created accidentally
payment cannot be charged twice
completed payment cannot become unpaid
```

Bu invariant'lar architecture'ı belirler.

---

# 146. Source of Truth + Derived State

Örneğin:

```text
Postgres
   |
   +--> Kafka
          |
          +--> Elasticsearch
          +--> Redis
          +--> Analytics
```

Postgres:

```text
source of truth
```

diğerleri:

```text
derived state
```

Derived state bozulursa yeniden üretilebilir.

Bu çok güçlü bir design property'dir.

---

# 147. Rebuildability

Bir component'i silip event/source data'dan yeniden oluşturabiliyor musun?

Örneğin:

```text
Kafka events
     |
     v
Search index
```

Search index bozuldu:

```text
drop index
replay events
rebuild
```

Bu operational resilience sağlar.

---

# 148. Replay

Event log durable ise:

```text
events:
1
2
3
4
5
```

consumer yeniden başlayıp:

```text
1 -> 5
```

replay edebilir.

Bu:

- Recovery
- New projections
- Debugging
- Backfill

için değerlidir.

---

# 149. Retention

Event/log storage:

```text
forever
```

değilse:

```text
7 days
30 days
1 year
```

gibi retention policy gerekir.

Retention:

- Cost
- Compliance
- Recovery
- Replay capability

arasında trade-off yaratır.

---

# 150. Cost as a First-Class Requirement

System design'da:

```text
Can we build it?
```

kadar:

```text
Can we afford it?
```

sorusunu sor.

Cost drivers:

- Compute
- Storage
- Network egress
- Replication
- Managed services
- Logging
- Monitoring
- Cross-region traffic

---

# 151. Cost Optimization Trade-offs

Daha fazla replica:

```text
availability ↑
cost ↑
```

Daha fazla cache:

```text
latency ↓
DB load ↓
memory cost ↑
consistency complexity ↑
```

Daha fazla index:

```text
read latency ↓
write cost ↑
storage ↑
```

Daha fazla retention:

```text
recovery/replay ↑
storage cost ↑
```

---

# 152. Complexity Budget

Bir architecture'ın:

```text
10 microservices
5 queues
3 databases
2 caches
multi-region
CQRS
Saga
event bus
```

olması onu otomatik olarak iyi yapmaz.

Her component:

- operational burden
- failure mode
- observability
- on-call complexity

getirir.

> **Complexity is a cost.**

---

# 153. Distributed System Tax

Bir şeyi distributed yaptığın anda:

```text
network
timeouts
partial failures
consistency
coordination
retries
ordering
observability
```

gibi problemler gelir.

Bu nedenle:

> "Microservice yapalım" bir çözüm değil, bir trade-off'tur.

---

# 154. Microservices vs Monolith

Monolith:

```text
simple deployment
simple transaction
simple local calls
```

Ama:

```text
scaling unit
team coupling
blast radius
```

problemleri olabilir.

Microservices:

```text
independent scaling
team autonomy
failure isolation
```

ama:

```text
network calls
distributed transactions
observability
deployment complexity
```

getirir.

---

# 155. Service Boundary Nasıl Seçilir?

En iyi boundary genellikle:

```text
business capability
```

etrafındadır.

Örneğin:

```text
Payment
Inventory
Shipping
Identity
```

sadece:

```text
users table
orders table
payments table
```

diye DB table'larına göre servis bölmek iyi boundary olmayabilir.

---

# 156. Shared Database Problemi

```text
Service A
   |
   v
 shared DB
   ^
   |
Service B
```

Servisler DB schema'ya coupling olur.

Bir service migration yaptığında diğerleri kırılabilir.

Gerçek service ownership:

```text
Service A -> DB A
Service B -> DB B
```

şeklinde daha net olabilir.

Ama her servis için ayrı DB de otomatik olarak doğru değildir.

---

# 157. API Gateway

Gateway:

```text
Client
 |
 v
API Gateway
 +--> Service A
 +--> Service B
 +--> Service C
```

Sağlayabilir:

- Auth
- Rate limit
- Routing
- TLS termination
- Request shaping

Ama:

> Gateway tek bottleneck/SPOF olmamalıdır.

---

# 158. Service Discovery

Dynamic instances:

```text
A1
A2
A3
```

client'ın güncel instance'ları bulması gerekir.

Yaklaşımlar:

- DNS
- Registry
- Platform-native discovery

---

# 159. Retryable vs Non-Retryable Errors

Retry yapılabilecek:

```text
timeout
temporary unavailable
connection reset
```

Retry yapılmaması gereken:

```text
invalid input
permission denied
business rule violation
```

Her error'ı retry etmek production bug'ıdır.

---

# 160. HTTP Status Semantics

Örnek:

```text
400 -> malformed/invalid request
401 -> unauthenticated
403 -> authenticated but forbidden
404 -> resource not found
409 -> state conflict
429 -> rate limited
500 -> internal failure
502 -> bad upstream response
503 -> service unavailable
504 -> upstream timeout
```

Ama status code seçimi API semantics ile uyumlu olmalıdır.

---

# 161. 502 vs 503 vs 504

```text
502:
gateway upstream'ten geçersiz response aldı

503:
service şu anda request işleyemiyor

504:
upstream response timeout
```

Bunlar monitoring ve client retry davranışını etkileyebilir.

---

# 162. Graceful Shutdown

Production service:

```text
SIGTERM
   |
   v
stop new traffic
   |
   v
finish in-flight requests
   |
   v
stop workers
   |
   v
close DB/clients
   |
   v
exit
```

K8s/container ortamında kritik.

---

# 163. Deploy Strategy

## Rolling Deploy

Instance'lar yavaşça değiştirilir.

## Blue/Green

```text
Blue = current
Green = new
```

traffic switch edilir.

## Canary

```text
1%
5%
25%
50%
100%
```

Yeni version kademeli traffic alır.

Canary:

- Error rate
- Latency
- Business metrics

ile değerlendirilmelidir.

---

# 164. Feature Flags

Deploy:

```text
code exists
```

Release:

```text
feature enabled
```

ayrılabilir.

Avantaj:

- Safer rollout
- Quick disable
- Experiment

Ama:

> Feature flag'ler teknik borca dönüşebilir.

Eski flag'ler temizlenmelidir.

---

# 165. System Design'da Trade-off Düşünme Sistemi

Her architecture decision için şu 8 soruyu sor:

```text
1. Ne kazanıyorum?
2. Ne kaybediyorum?
3. Hangi failure mode ortaya çıkıyor?
4. Latency'ye etkisi?
5. Availability'ye etkisi?
6. Consistency'ye etkisi?
7. Cost'a etkisi?
8. Operational complexity'ye etkisi?
```

Bu 8 soru çoğu mimari kararı açıklamak için yeterlidir.

---

# 166. Trade-off Matrix

| Karar | Kazanç | Bedel |
|---|---|---|
| Cache | Latency ↓ | Staleness/invalidations |
| Replica | Availability ↑ | Consistency/cost |
| Sharding | Capacity ↑ | Query complexity |
| Async queue | Decoupling | Eventual consistency |
| Strong consistency | Correctness | Latency/availability |
| Multi-region | DR/availability | Complexity/cost |
| More retries | Transient failure recovery | Load amplification |
| Compression | Bandwidth ↓ | CPU ↑ |
| Denormalization | Read speed ↑ | Write complexity |
| More indexes | Read speed ↑ | Write/storage cost |
| Bigger pool | Concurrency ↑ | Downstream pressure |
| Load shedding | System survival | Some requests rejected |

---

# 167. Bir System Design Sorusu Nasıl Çözülür?

Şimdi asıl framework.

## Phase 1 — Clarify

Soruyu hemen çözmeye başlama.

Şunları sor:

```text
Users?
Traffic?
Read/write ratio?
Latency target?
Availability?
Data size?
Retention?
Consistency?
Geography?
Security?
Cost?
```

---

# 168. Phase 2 — Functional Requirements

Sadece kritik feature'ları yaz.

Örneğin URL shortener:

```text
1. Create short URL
2. Redirect short URL
3. Optional analytics
```

Analytics kritik değilse ilk architecture'ı onunla karmaşıklaştırma.

---

# 169. Phase 3 — Non-Functional Requirements

Örneğin:

```text
100M URLs
10M DAU
100k redirects/sec peak
p99 < 100ms
99.99% availability
```

Artık architecture şekillenmeye başlar.

---

# 170. Phase 4 — Back-of-the-Envelope

Hesapla:

```text
RPS
Peak RPS
Read/write ratio
Storage
Bandwidth
Connection count
Queue throughput
```

Exact olması gerekmez.

Ama sayısal düşün.

---

# 171. Phase 5 — Identify Access Patterns

Sor:

```text
En sık hangi query?
En pahalı query?
En büyük dataset?
En hot key?
Read/write ratio?
```

Database seçimi buradan çıkar.

---

# 172. Phase 6 — Define Source of Truth

Örneğin:

```text
Postgres = source of truth
Redis = cache
Kafka = event transport
Elastic = search index
```

Bu distinction architecture'ı çok sadeleştirir.

---

# 173. Phase 7 — Start With Simplest Architecture

İlk:

```text
Client
 |
Load Balancer
 |
App
 |
DB
```

Sonra bottleneck geldikçe:

```text
          +--> Redis
Client -> LB -> App -> DB
             |
             +--> Queue -> Worker
```

Daha sonra gerekirse:

```text
sharding
replication
multi-region
```

ekle.

---

# 174. Golden Rule

> **Bottleneck yoksa optimization yapma.**

"Her şeyi Redis'e koyalım."

"Kafka kullanalım."

"Microservice yapalım."

"Multi-region yapalım."

bunlar requirement'tan çıkmıyorsa architecture decoration'dır.

---

# 175. Phase 8 — Failure Analysis

Her dependency için:

```text
What if it is slow?
What if it is down?
What if response is lost?
What if request is duplicated?
What if data is stale?
What if network partitions?
```

sor.

---

# 176. Dependency Failure Matrix

| Dependency | Failure | Response |
|---|---|---|
| Redis | down | DB fallback |
| DB | down | read cache / fail |
| Queue | down | reject/degrade |
| Payment | timeout | retry/idempotency |
| Search | down | fallback DB |
| Recommendation | down | omit feature |

Bu tablo production thinking'i güçlendirir.

---

# 177. Phase 9 — Consistency Decision

Her data için ayrı karar ver:

```text
Payment status -> strong
Inventory -> strong-ish / transactional
Like count -> eventual
Search index -> eventual
Analytics -> eventual
```

"System eventual consistent" demek fazla geneldir.

> **Consistency requirement data/domain bazında belirlenir.**

---

# 178. Phase 10 — Scaling Decision

Her bottleneck için:

```text
CPU -> horizontal scale
DB read -> replicas/cache
DB write -> partition/shard
large dataset -> shard/object storage
burst -> queue
hot key -> cache/replication
```

Çözüm bottleneck'e göre seçilir.

---

# 179. Phase 11 — Resilience

Şunları ekle:

```text
timeouts
retries
backoff
jitter
circuit breakers
bulkheads
load shedding
rate limiting
idempotency
graceful degradation
```

Ama hepsini otomatik ekleme.

Failure mode gerektiriyorsa ekle.

---

# 180. Phase 12 — Observability

En azından:

```text
RPS
error rate
p50/p95/p99 latency
CPU
memory
DB connections
queue depth
cache hit ratio
replication lag
```

ve distributed flow için trace.

---

# 181. Phase 13 — Security

Kontrol listesi:

```text
Authentication
Authorization
Encryption in transit
Encryption at rest
Secrets
Rate limiting
Input validation
Audit logs
PII
Tenant isolation
Abuse prevention
```

---

# 182. Phase 14 — Disaster Recovery

Sor:

```text
RPO?
RTO?
AZ failure?
Region failure?
Data corruption?
Accidental deletion?
```

Sonra:

```text
backup
replication
restore
failover
```

tasarla.

---

# 183. Phase 15 — Cost

Son olarak:

```text
What is expensive?
```

Özellikle:

```text
cross-region traffic
storage
replicas
high-cardinality logs
managed DB
idle standby
```

bak.

---

# 184. Final Architecture Review

Diagram'a bak ve şu sırayla incele:

```text
1. Single points of failure?
2. Bottlenecks?
3. Hotspots?
4. Unbounded queues?
5. Unbounded goroutines?
6. Unbounded retries?
7. Missing timeouts?
8. Duplicate side effects?
9. Consistency bugs?
10. Data loss?
11. Recovery path?
12. Observability?
13. Security?
14. Cost?
```

---

# 185. System Design Decision Record

Her önemli karar için:

```text
Decision:
Use Redis cache.

Why:
DB read latency is too high.

Benefit:
Lower latency and DB load.

Cost:
Stale data and invalidation complexity.

Failure:
Redis unavailable.

Fallback:
Read from DB.

Consistency:
Eventual for this data.

Operational:
Monitor hit ratio and memory.

Alternative:
DB read replicas.
```

Bu formatı kullanırsan architecture kararların çok daha net olur.

---

# 186. Problem Çözme Algoritması

System Design için zihinsel pseudocode:

```text
function design(requirements):

    requirements = clarify(requirements)

    functional = extract_functional(requirements)
    nonfunctional = extract_nonfunctional(requirements)

    traffic = estimate_traffic()
    storage = estimate_storage()
    bandwidth = estimate_bandwidth()

    invariants = identify_business_invariants()
    access_patterns = identify_access_patterns()

    source_of_truth = choose_source_of_truth()

    architecture = simplest_viable_architecture()

    while bottleneck_or_requirement_not_met:

        bottleneck = identify_bottleneck()

        if bottleneck == reads:
            consider(cache, read_replica)

        if bottleneck == writes:
            consider(sharding, batching)

        if bottleneck == burst:
            consider(queue, backpressure)

        if bottleneck == latency:
            consider(cache, parallelism, locality)

        if bottleneck == availability:
            consider(redundancy, failover)

        if bottleneck == durability:
            consider(replication, WAL, backup)

        if bottleneck == hotspot:
            consider(partitioning, caching, key redesign)

    define_consistency()

    design_failure_handling()

    define_observability()

    define_security()

    define_DR()

    estimate_cost()

    review_tradeoffs()

    return architecture
```

---

# 187. Bir Requirement'tan Architecture'a Nasıl Gidilir?

Örnek:

> "100k reads/sec ve p99 < 100ms."

Bu requirement:

```text
100k RPS
+
low latency
```

diyor.

Düşün:

```text
DB direct reads
      |
      v
probably expensive
```

Cache:

```text
Client
 |
 v
Cache
 |
 +--> HIT -> fast
 |
 +--> MISS -> DB
```

Ama sonra:

```text
cache failure?
stale data?
hot keys?
cache stampede?
```

sorularını sor.

Architecture requirement'tan türetilir.

---

# 188. İkinci Örnek: Payment System

Requirement:

```text
Payment cannot be duplicated.
```

Bu requirement:

```text
idempotency
+
durability
+
strong business invariants
```

gerektirir.

Architecture:

```text
Client
 |
Idempotency-Key
 |
Payment Service
 |
DB transaction
 |
Outbox
 |
Event Bus
```

Retry:

```text
same key
    |
existing result
    |
return same result
```

Burada cache ilk çözüm değildir.

Invariant önce gelir.

---

# 189. Üçüncü Örnek: Notification System

Requirement:

```text
10M notifications
burst traffic
delivery can be delayed
```

Bu:

```text
async
+
queue
+
worker pool
+
rate limiting
+
retry
```

düşündürür.

Architecture:

```text
API
 |
Queue
 |
+--> Worker pool
      |
      +--> Email
      +--> SMS
      +--> Push
```

Her provider için bulkhead:

```text
Email pool
SMS pool
Push pool
```

gerekebilir.

---

# 190. Dördüncü Örnek: Social Feed

Requirement:

```text
read-heavy
low latency
huge user base
```

Düşün:

```text
cache
fan-out strategy
```

İki yaklaşım:

## Fan-out on write

Tweet oluştururken follower feed'lerine yaz.

Avantaj:

```text
read fast
```

Dezavantaj:

```text
celebrity with 50M followers
```

write explosion.

## Fan-out on read

Read sırasında takip edilen kişilerin postlarını merge et.

Avantaj:

```text
write cheap
```

Dezavantaj:

```text
read expensive
```

Hybrid çoğu zaman daha gerçekçidir.

---

# 191. Trade-off'u Açıklamanın En İyi Formatı

Bir interviewer'a:

> "Redis kullanırım."

deme.

Şöyle de:

> "Bu endpoint read-heavy ve p99 latency hedefimiz düşük. DB'nin her request'te hit edilmesi gereksiz load oluşturacağı için cache-aside Redis kullanabilirim. Bunun karşılığında cache invalidation ve stale-read problemi getiriyorum. Bu data business-critical değilse eventual consistency kabul edilebilir. Redis failure durumunda DB fallback'i de tasarlayarak cache'i source of truth yapmam."

Bu:

> System Design düşüncesidir.

---

# 192. Her Architecture Decision İçin 12 Soru

```text
1. Requirement ne?
2. Bottleneck ne?
3. Failure mode ne?
4. State nerede?
5. Source of truth nerede?
6. Consistency ne kadar gerekli?
7. Latency budget ne?
8. Availability target ne?
9. Scale nasıl?
10. Recovery nasıl?
11. Operational complexity ne?
12. Cost ne?
```

---

# 193. System Design Cheat Sheet

## Traffic

```text
RPS
peak RPS
read/write ratio
payload size
```

## Storage

```text
records/day
bytes/record
retention
replication factor
indexes
```

## Performance

```text
p50
p95
p99
throughput
queue depth
```

## Reliability

```text
availability
RPO
RTO
failure domains
```

## Consistency

```text
strong
eventual
read-your-writes
monotonic reads
ordering
```

## Scaling

```text
vertical
horizontal
partitioning
sharding
caching
replicas
```

## Resilience

```text
timeout
retry
backoff
jitter
circuit breaker
bulkhead
load shedding
idempotency
```

## Data

```text
transactions
indexes
normalization
denormalization
CQRS
outbox
saga
```

## Async

```text
queue
stream
consumer groups
backpressure
DLQ
replay
```

## Operations

```text
logs
metrics
traces
alerts
deploy
rollback
DR
```

## Security

```text
authn
authz
TLS
secrets
rate limit
audit
tenant isolation
```

---

# 194. En Sık Yapılan System Design Hataları

## Hata 1

"Traffic yokken Kafka eklemek."

## Hata 2

"Her şeyi cache'lemek."

## Hata 3

"Multi-region çünkü high availability lazım."

## Hata 4

"Microservices daha scalable."

## Hata 5

"Retry ekledim, failure handling tamam."

## Hata 6

"Database replica ekledim, availability çözüldü."

## Hata 7

"Queue koydum, scalability çözüldü."

## Hata 8

"Exactly once."

## Hata 9

"p99 yerine average latency."

## Hata 10

"Consistency requirement'ını domain'den bağımsız düşünmek."

## Hata 11

"Backup var, DR tamam."

## Hata 12

"Observability sonradan eklenir."

## Hata 13

"Her şeyi horizontally scale ederim."

Ama DB bottleneck olabilir.

---

# 195. Architecture Smell Checklist

Şunu görürsen düşün:

```text
retry without timeout
```

→ Retry storm.

```text
queue without max depth
```

→ Unbounded memory/storage.

```text
worker pool without downstream limit
```

→ Downstream overload.

```text
cache without invalidation strategy
```

→ stale data.

```text
replicas without consistency model
```

→ confusing reads.

```text
multi-region without conflict strategy
```

→ distributed write conflicts.

```text
DB without indexes
```

→ latency growth.

```text
many indexes
```

→ write amplification.

```text
global ordering
```

→ scalability bottleneck.

```text
shared database between services
```

→ coupling.

```text
synchronous fan-out
```

→ tail latency amplification.

```text
no idempotency on payment
```

→ duplicate side effects.

---

# 196. System Design'i Bir Optimizasyon Problemi Gibi Düşün

Her architecture:

```text
minimize:
    cost
    complexity
    latency
    failure impact

subject to:
    availability >= target
    durability >= target
    throughput >= target
    consistency >= requirement
    security >= requirement
```

Bu mental model çok değerlidir.

Çünkü:

> Hiçbir system design her şeyi maksimum yapamaz.

---

# 197. Pareto Thinking

Örneğin:

```text
Consistency ↑
Availability ↓

Replication ↑
Cost ↑

Caching ↑
Latency ↓
Staleness ↑

Retries ↑
Transient recovery ↑
Load amplification ↑

Sharding ↑
Capacity ↑
Query complexity ↑
```

Mükemmel architecture yoktur.

> **Requirement'lara en uygun trade-off vardır.**

---

# 198. System Design'da Öncelik Sırası

Bir soruyu çözerken genellikle:

```text
1. Correctness
2. Business invariants
3. Safety
4. Reliability
5. Availability
6. Performance
7. Scalability
8. Cost optimization
9. Complexity reduction
```

şeklinde düşünmek faydalıdır.

Ama domain'e göre değişebilir.

Payment'ta correctness çok yüksek öncelikteyken analytics'te latency/cost daha önemli olabilir.

---

# 199. The "Why This, Not That?" Rule

Bir component eklediğinde hemen sor:

```text
Why Redis?
Why Kafka?
Why SQL?
Why NoSQL?
Why replica?
Why shard?
Why async?
Why microservice?
Why multi-region?
Why cache?
Why strong consistency?
```

Ve her biri için:

```text
requirement
   ↓
problem
   ↓
solution
   ↓
trade-off
```

zinciri kur.

---

# 200. Ultimate System Design Framework

Bir interview veya gerçek proje için şu sırayı ezberleyebilirsin:

```text
┌──────────────────────────────────────────┐
│ 1. REQUIREMENTS                          │
│    Functional + Non-functional           │
├──────────────────────────────────────────┤
│ 2. SCALE                                 │
│    RPS + storage + bandwidth             │
├──────────────────────────────────────────┤
│ 3. INVARIANTS                            │
│    What must NEVER go wrong?             │
├──────────────────────────────────────────┤
│ 4. ACCESS PATTERNS                       │
│    Read/write/query patterns             │
├──────────────────────────────────────────┤
│ 5. SOURCE OF TRUTH                       │
│    Where is authoritative state?         │
├──────────────────────────────────────────┤
│ 6. SIMPLE DESIGN                         │
│    Start with minimum viable architecture│
├──────────────────────────────────────────┤
│ 7. BOTTLENECKS                            │
│    CPU / DB / network / storage / hotkey │
├──────────────────────────────────────────┤
│ 8. SCALE                                  │
│    Cache / replica / shard / queue       │
├──────────────────────────────────────────┤
│ 9. CONSISTENCY                            │
│    Strong / eventual / ordering          │
├──────────────────────────────────────────┤
│ 10. FAILURE                               │
│     timeout / retry / breaker / bulkhead │
├──────────────────────────────────────────┤
│ 11. DATA CORRECTNESS                      │
│     idempotency / transaction / outbox   │
├──────────────────────────────────────────┤
│ 12. OBSERVABILITY                         │
│     logs / metrics / traces              │
├──────────────────────────────────────────┤
│ 13. SECURITY                              │
│     auth / isolation / abuse             │
├──────────────────────────────────────────┤
│ 14. DR                                    │
│     RPO / RTO / backup / failover        │
├──────────────────────────────────────────┤
│ 15. COST                                  │
│     infrastructure + operational cost    │
├──────────────────────────────────────────┤
│ 16. TRADE-OFFS                            │
│     Why this design? Why not alternatives│
└──────────────────────────────────────────┘
```

---

# 201. 30-Second Mental Checklist

Bir system design sorusu geldiğinde:

```text
WHO?
    users / tenants

WHAT?
    functional requirements

HOW MUCH?
    RPS / storage / bandwidth

HOW FAST?
    latency / throughput

HOW CORRECT?
    consistency / invariants

HOW AVAILABLE?
    redundancy / failover

HOW DURABLE?
    replication / WAL / backup

HOW SCALE?
    cache / replicas / shards / queues

WHAT FAILS?
    dependency / network / DB / region

WHAT HAPPENS THEN?
    timeout / retry / fallback / degradation

HOW DO WE KNOW?
    metrics / logs / traces

HOW DO WE RECOVER?
    RPO / RTO / restore

HOW MUCH DOES IT COST?
    compute / storage / network / complexity

WHY THIS DESIGN?
    explicit trade-offs
```

---

# 202. 5 Dakikalık Interview Flow

Bir interview'da hızlıca:

### Dakika 0-1

Requirements:

```text
functional
traffic
latency
availability
consistency
```

### Dakika 1-2

Capacity estimation:

```text
RPS
storage
bandwidth
```

### Dakika 2-3

High-level architecture:

```text
LB
App
Cache
DB
Queue
```

### Dakika 3-4

Deep dive:

```text
scaling
consistency
failure
data model
```

### Dakika 4-5

Trade-offs:

```text
why cache?
why async?
why SQL?
why shard?
what if DB fails?
what if traffic 10x?
```

---

# 203. Gerçek Projede Daha Derin Review

Interview'dan farklı olarak production design'da ayrıca:

```text
deployment
migration
rollback
capacity limits
alerts
on-call
runbooks
compliance
cost
ownership
dependency contracts
schema evolution
backward compatibility
```

düşün.

---

# 204. Son Mental Model

Bütün bu dokümanı tek bir modele sıkıştır:

```text
                 REQUIREMENTS
                      |
                      v
                  WORKLOAD
                      |
                      v
                 BOTTLENECKS
                      |
                      v
                  STATE
                      |
          +-----------+-----------+
          |                       |
          v                       v
     CONSISTENCY              AVAILABILITY
          |                       |
          +-----------+-----------+
                      |
                      v
                  SCALING
                      |
          +-----------+-----------+
          |                       |
          v                       v
        SYNC                    ASYNC
          |                       |
          v                       v
      LOW LATENCY              BUFFERING
          |                       |
          +-----------+-----------+
                      |
                      v
                   FAILURE
                      |
          +-----------+-----------+
          |           |           |
       timeout      retry      fallback
          |           |           |
          +-----------+-----------+
                      |
                      v
                 OBSERVABILITY
                      |
                      v
                  RECOVERY
                      |
                      v
                    COST
                      |
                      v
                 TRADE-OFF
```

Ve en önemli cümle:

> **System Design = requirements'ları karşılayan en basit sistemi tasarla; sonra gerçek bottleneck ve failure mode'ları gördükçe complexity ekle.**

---

# 205. Kendini Test Etmek İçin Soru Seti

Her design'da bunları cevaplayabiliyor ol:

### Availability

- Single point of failure nerede?
- AZ ölürse ne olur?
- Region ölürse ne olur?
- Failover nasıl gerçekleşir?

### Scalability

- En hızlı büyüyen resource hangisi?
- Horizontal scale nerede?
- Shard key ne?
- Hot key olabilir mi?

### Latency

- p99 neden yükseliyor?
- Critical path nedir?
- Fan-out kaç dependency?
- Tail latency nereden geliyor?

### Durability

- ACK ne zaman veriliyor?
- O anda data nerede?
- Crash olursa ne kaybolur?
- RPO nedir?

### Consistency

- Hangi data strong olmalı?
- Hangisi eventual olabilir?
- Stale read kabul ediliyor mu?
- Ordering gerekli mi?

### Failure

- Dependency timeout olursa?
- Retry güvenli mi?
- Circuit breaker?
- Fallback?
- Load shedding?

### Data

- Source of truth?
- Transaction boundary?
- Index?
- Partition key?
- Retention?

### Async

- Queue neden var?
- Queue capacity?
- Backpressure?
- Duplicate message?
- Ordering?
- DLQ?
- Replay?

### Operations

- Nasıl deploy?
- Nasıl rollback?
- Nasıl observe?
- Nasıl alert?
- Nasıl restore?

### Cost

- En pahalı component?
- Network egress?
- Replication?
- Storage?
- Idle capacity?

---

# 206. Öğrenme Sırası

Bu konuları aynı anda ezberlemeye çalışma.

Önerilen sıra:

```text
LEVEL 1 — Foundations
    |
    +-- latency
    +-- throughput
    +-- availability
    +-- reliability
    +-- scalability
    +-- durability
    |
    v
LEVEL 2 — Data
    |
    +-- transactions
    +-- indexes
    +-- replication
    +-- partitioning
    +-- sharding
    +-- consistency
    |
    v
LEVEL 3 — Performance
    |
    +-- caching
    +-- CDN
    +-- batching
    +-- connection pools
    +-- queues
    |
    v
LEVEL 4 — Distributed Systems
    |
    +-- CAP
    +-- PACELC
    +-- quorum
    +-- consensus
    +-- leader election
    +-- ordering
    +-- clocks
    |
    v
LEVEL 5 — Resilience
    |
    +-- timeout
    +-- retry
    +-- backoff
    +-- jitter
    +-- circuit breaker
    +-- bulkhead
    +-- load shedding
    +-- idempotency
    |
    v
LEVEL 6 — Data Workflows
    |
    +-- outbox
    +-- saga
    +-- CQRS
    +-- event sourcing
    +-- replay
    |
    v
LEVEL 7 — Production
    |
    +-- observability
    +-- SLO/SLA
    +-- DR
    +-- deployment
    +-- security
    +-- cost
```

---

# 207. Sonuç: Artık Soruyu Ezberlemeyeceksin

İyi bir System Designer:

> "Twitter için Kafka kullanırım."

demez.

Şunu düşünür:

```text
Requirement:
100M users

Workload:
read-heavy

Latency:
p99 < 200ms

Problem:
timeline read expensive

Option A:
fan-out on read

Option B:
fan-out on write

Problem:
celebrity accounts create write amplification

Decision:
hybrid fan-out

New failure:
feed cache stale

Decision:
eventual consistency acceptable

New problem:
cache stampede

Decision:
request coalescing + jitter

New problem:
queue backlog

Decision:
bounded workers + autoscaling + load shedding

New requirement:
99.99 availability

Decision:
multi-AZ + replicas

New requirement:
regional disaster

Decision:
DR / multi-region strategy

Trade-off:
complexity + cost increased
```

İşte hedeflenen düşünme biçimi budur.

**Bir architecture çizmek son adımdır.**

Asıl skill:

```text
Requirement
   ↓
Constraint
   ↓
Bottleneck
   ↓
Failure mode
   ↓
Trade-off
   ↓
Decision
   ↓
Validation
```

Bu zinciri herhangi bir probleme uygulayabiliyorsan, daha önce görmediğin bir System Design sorusunu da çözebilirsin.
