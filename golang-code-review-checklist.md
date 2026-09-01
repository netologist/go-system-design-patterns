# Golang Live Coding Mülakatı — Code Review / Bug-Hunting Checklist

Mülakatta sana bir kod parçası gösterip "bunu review et" dediklerinde, rastgele bakmak yerine bu sırayla tara. Yüksek sesle düşünerek ilerle — mülakatçı genelde *süreci* de değerlendirir, sadece bulduğun bug'ları değil.

---

## 0. İlk Okuma (30-60 saniye)
- Fonksiyon/paket ne yapmaya çalışıyor? Amaç ne? (Yorumla, isimlerle anla)
- Giriş noktası nerede, veri akışı nasıl? (input → transform → output)
- "Happy path" nedir, önce onu izle.

Yüksek sesle söyle: *"Öncelikle genel akışı anlamaya çalışıyorum..."*

---

## 1. Correctness / Mantık Hataları
- [ ] **Off-by-one hataları**: `<` vs `<=`, slice indexleri (`s[i:j]`), `len()` sınırları
- [ ] **Nil pointer / nil map dereference**: `var m map[string]int; m["x"] = 1` → panic (nil map'e yazma)
- [ ] **Nil interface tuzağı**: `var err *MyError; return err` → `err != nil` true döner (typed nil)
- [ ] **Değer vs pointer semantiği**: struct'ı value ile mi pointer ile mi geçiriyor, beklenmedik kopyalama var mı?
- [ ] **Slice/array paylaşımı**: `append` başka bir slice'ın underlying array'ini bozuyor mu? (`cap` aşılmadan append yapılırsa aynı array paylaşılır)
- [ ] **Map iterasyon sırası**: Go'da map iterasyonu deterministik değil — sıraya bağımlı mantık var mı?
- [ ] **Integer overflow / division/modulo** ile negatif sayı davranışı
- [ ] **String/byte/rune karışıklığı**: `len(s)` byte sayar, unicode karakter değil; `for i, r := range s` rune index'i byte offset'i verir, indeks değil
- [ ] **Loop variable capture (Go <1.22)**: `for _, v := range items { go func(){ use(v) }() }` → hepsi son değeri kullanır (Go 1.22+'de düzeldi, versiyonu sor!)
- [ ] Karşılaştırma hataları: struct'larda `==` kullanımı (karşılaştırılamayan alan varsa compile error, slice/map/func varsa özellikle)

---

## 2. Concurrency (Go mülakatlarının favori konusu)
- [ ] **Race condition**: Paylaşılan değişkene birden fazla goroutine'den mutex/atomic olmadan erişim var mı?
- [ ] **Deadlock**: Kanal okuma/yazma karşılıksız mı kalıyor? Unbuffered channel'a gönderirken karşı taraf yoksa bloklanır
- [ ] **Goroutine leak**: Goroutine'in sonsuza kadar bloklanma ihtimali var mı (context cancel edilmiyor, channel hiç kapanmıyor)?
- [ ] **WaitGroup kullanımı**: `Add()` goroutine başlamadan önce mi çağrılıyor? `Done()` her yolda (panic dahil, `defer` ile) çağrılıyor mu?
- [ ] **Channel kapatma disiplinleri**: Kapalı kanala yazma → panic. Kanalı sadece gönderen taraf kapatmalı. Kapalı kanaldan okuma sıfır değer + `ok=false` döner, bu kontrol ediliyor mu?
- [ ] **Mutex kilit kapsamı**: `Lock()` sonrası `defer Unlock()` var mı, yoksa early return'de kilit kalıyor mu?
- [ ] **Context kullanımı**: `ctx.Done()` dinleniyor mu, cancel fonksiyonu `defer cancel()` ile çağrılıyor mu (context leak önleme)?
- [ ] Select bloklarında default case olmadan sonsuz blok riski var mı?

Söylenecek söz: *"Burada goroutine'ler arası paylaşılan state var, bunu -race flag'i ile test etmek isterim."*

---

## 3. Error Handling
- [ ] Hatalar yutuluyor mu (`_ = err` veya sessizce ignore)?
- [ ] `err != nil` kontrolü her çağrıdan sonra var mı, yoksa bir yerde atlanmış mı?
- [ ] Error wrapping doğru mu (`fmt.Errorf("...: %w", err)`) yoksa bilgi mi kayboluyor (`%v` ile context siliniyor)?
- [ ] Sentinel error karşılaştırması `errors.Is` / `errors.As` ile mi yapılıyor, yoksa `==` ile kırılgan mı?
- [ ] Panic/recover doğru yerde mi kullanılıyor (genelde sadece paket sınırında, aşırı kullanım code smell)?
- [ ] Defer içinde recover varsa, sadece log mu atıyor yoksa akışı doğru mu yönetiyor?

---

## 4. Kaynak Yönetimi / Performans
- [ ] **Defer ile kaynak kapatma**: `os.Open`, `resp.Body`, `db.Rows` gibi kaynaklar `defer Close()` ile kapatılıyor mu?
- [ ] **Defer'in loop içinde kullanımı**: Loop içindeki `defer` fonksiyon bitene kadar birikir — bellek/kaynak sızıntısı riski
- [ ] **Gereksiz kopyalama**: Büyük struct'lar value olarak fonksiyonlara geçiliyor mu (pointer alması gerekirken)?
- [ ] **Slice preallocation**: `append` döngüde kapasite belirtilmeden mi büyütülüyor (`make([]T, 0, n)` ile önceden ayrılabilir miydi)?
- [ ] **String concatenation**: Döngüde `+=` ile string birleştirme yerine `strings.Builder` kullanılmalı mıydı?
- [ ] N+1 problem / gereksiz tekrar hesaplama var mı (cache'lenebilir bir değer her seferinde yeniden hesaplanıyor mu)?

---

## 5. API Tasarımı & Idiomatic Go
- [ ] Fonksiyon imzaları idiomatic mi (`error` her zaman son dönüş değeri mi)?
- [ ] Interface'ler gereksiz yere büyük mü ("accept interfaces, return structs" prensibi uygulanmış mı)?
- [ ] Exported/unexported (büyük/küçük harf) kullanımı mantıklı mı, gereksiz export var mı?
- [ ] Zero-value'lar kullanışlı mı (struct'ın sıfır değeri anlamlı bir başlangıç durumu mu)?
- [ ] Gereksiz getter/setter'lar (Go'da `GetX()` yerine sadece `X()` idiomatic) var mı?
- [ ] `interface{}`/`any` aşırı kullanılmış mı, generics veya somut tip daha uygun olur muydu?

---

## 6. Okunabilirlik & İsimlendirme
- [ ] İsimler kısa ama anlamlı mı (Go kısa isimleri sever: `i`, `err`, `ctx` — ama anlam kaybı olmamalı)
- [ ] Fonksiyon çok mu uzun / çok iş mi yapıyor (tek sorumluluk ihlali)?
- [ ] Magic number/string var mı, const olarak tanımlanmalı mıydı?
- [ ] Yorum satırları kodu tekrar mı ediyor, yoksa gerçekten "neden"i mi açıklıyor?

---

## 7. Testability
- [ ] Fonksiyon test edilebilir mi (dış bağımlılıklar interface arkasında mı, yoksa doğrudan `time.Now()`, `http.Get()` gibi çağrılar mı gömülü)?
- [ ] Dependency injection var mı yoksa global state'e mi bağımlı?
- [ ] Edge case'ler için test yazılmış mı (boş slice, nil input, 0 değeri, tek elemanlı liste)?

---

## Mülakatta Nasıl Sunmalısın (süreç tavsiyesi)
1. **Önce oku, sonra konuş.** Sessizce 30 saniye oku, sonra genel akışı özetle.
2. **Ciddiyet sırasına göre söyle**: Önce crash/panic/data-race gibi kritik bug'lar, sonra mantık hataları, en son style/idiomatic öneriler.
3. **Her bulguyu somutlaştır**: "Burada satır X'te nil pointer riski var çünkü Y" gibi, sadece "bu kötü" deme.
4. **Düzeltmeyi öner, hatta küçük bir kod parçasıyla göster.**
5. **Belirsizlik varsa sor**: "Bu fonksiyon concurrent çağrılacak mı?" gibi netleştirici sorular sormaktan çekinme — gerçek code review'da da böyle yapılır.

---

## Hızlı Referans: En Sık Çıkan Go Tuzakları
| Tuzak | Belirti |
|---|---|
| Loop variable capture | Goroutine'ler hep aynı/son değeri kullanır |
| Nil map write | `panic: assignment to entry in nil map` |
| Typed nil interface | `err != nil` beklenmedik şekilde true |
| Slice aliasing | `append` başka slice'ı bozar |
| Kapalı kanala yazma | `panic: send on closed channel` |
| Defer'de kilit yok | Panic/erken return'de mutex kilitli kalır |
| Range üzerinde rune index | Unicode string'de yanlış index hesaplama |
