# Typed Domain Errors (Tipli Domain Hataları)

## 1. Genel Bakış ve Tanım
Hata türü, operasyon ve alan detaylarını taşıyan struct hatalar ve errors.As.

Bu desen, yüksek ölçekli ve kritik Go backend servislerinde sistemin hata altında dayanıklı (resilient), kaynak tüketimi açısından sınırlandırılmış (bounded) ve gözlemlenebilir (observable) olmasını sağlar.

## 2. Çözdüğü Production Problemleri
- **Kaynak Tükenmesi**: Sınırlandırılmamış goroutine, bağlantı veya bellek kullanımının sistemi çökertmesi.
- **Kademeli Çöküş (Cascading Failure)**: Yavaşlayan bir dış bağımlılığın tüm upstream servisleri kilitlemesi.
- **Veri Tutarsızlığı**: Ağ kopmaları ve eşzamanlı yarış durumlarında (race condition) verinin bozulması veya çift işlem yapılması.
- **Görünürlük Eksikliği**: Hatanın nerede ve neden oluştuğunun tespit edilememesi.

## 3. Mimari Akış ve Çalışma Mekanizması

```text
Gelen İstek / Görev
        |
        v
[ typed_errors Kontrol Katmanı ]
        |
        +---> Başarılı / İzin Verildi ---> Normal İş Mantığı Yürütme
        |
        +---> Hata / Sınır Aşıldı    ---> Kontrollü Red / Telafi / Fallback
```

## 4. Production İnce Ayarları ve Best Practice'ler
1. **Süre Sınırı (Timeout)**: Her I/O işlemine kesin bir `context.Context` deadline süresi atayın.
2. **Kapasite Üst Sınırı (Upper Bound)**: Kuyruk, havuz ve bellek boyutlarını her zaman sabit üst sınırlarla boyutlandırın.
3. **Fail-Safe Varsayılanlar**: Belirsizlik anında güvenli tarafta kalın (Fail-Closed yetkilendirme, güvenli fallback).
4. **İzlenebilirlik**: Başarısızlık ve tetiklenme durumlarında log ve metrik üretin.

## 5. Go İmplementasyonu ve Test Referansı
- **İmplementasyon**: `typed_errors.go`
- **Test Senaryoları**: `typed_errors_test.go`
- **İngilizce Doküman**: `typed_errors.md`
