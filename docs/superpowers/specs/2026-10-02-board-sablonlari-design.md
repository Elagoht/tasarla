# Hazır board şablonları — tasarım

Tarih: 2026-10-02 · Durum: bölüm 1–2 sohbette onaylandı, yazılı hali gözden geçirilecek

Bugün her board aynı üç kolonla (Yapılacak, Yapılıyor, Bitti) açılır. Ekipler sık karşılaşılan senaryolar için hazır, standart kabul edilen board kurulumları istiyor: kolonlar, WIP limitleri, etiketler, kart şablonları ve kurallar tek seferde gelsin.

## 1. Kullanıcının seçimleri

- **Tür:** hazır *board* şablonları (kart şablonu kataloğu değil).
- **Kaynak:** uygulamayla gelen sabit katalog. Kullanıcı kendi şablonunu kaydedemez, veritabanında şablon tablosu yoktur.
- **İçerik:** kolonlar ve WIP limitleri, etiketler, kart şablonları (tekrarlar dahil), kurallar.
- **Uygulama yeri:** yalnız yeni board oluştururken. Var olan board'lara şablon uygulanmaz.
- **Seçim:** aktarılacaklar grup grup seçilir. Kolonlar her zaman gelir.
- **Yaklaşım:** A. Katalog Go kodunda, metinler çeviri dosyalarında.

## 2. Katalog

| Şablon | Kolonlar (WIP) | Etiketler | Kart şablonları | Kurallar |
| --- | --- | --- | --- | --- |
| `simple` Basit | Yapılacak → Yapılıyor → Bitti | — | — | — |
| `scrum` Scrum | Backlog → Sprint → Yapılıyor (3) → İncelemede (2) → Bitti | Hikâye, Hata, Teknik borç, Araştırma | Kullanıcı hikâyesi, Hata, Haftalık retrospektif (her Cuma 16:00) | Yapılıyor'a girerken atanan kişi zorunlu; Bitti'ye girerken kontrol listesi tamam |
| `bugs` Hata takibi | Yeni → Önceliklendirildi → Düzeltiliyor (3) → Test ediliyor → Kapandı | Kritik, Regresyon, Arayüz, Sunucu | Hata raporu | Düzeltiliyor'a girerken atanan kişi zorunlu; Kapandı'ya yalnız takım lideri taşır |
| `software` Yazılım geliştirme | Backlog → Hazır → Geliştirme (3) → Kod incelemesi (2) → Test → Yayında | Özellik, İyileştirme, Altyapı | Özellik, Teknik görev | Kod incelemesinden çıkarken engelleyiciler bitmiş olmalı; kişi başı WIP 2 |
| `content` İçerik takvimi | Fikirler → Yazılıyor → Editörde → Planlandı → Yayında | Blog, Sosyal medya, Bülten, Video | Blog yazısı, Haftalık bülten (her Pazartesi 09:00) | Planlandı'ya girerken bitiş tarihi zorunlu |
| `hiring` İşe alım | Başvurular → Ön görüşme → Teknik mülakat → Teklif → İşe alındı | Ön yüz, Arka yüz, Tasarım, Stajyer | Aday | Teklif'e yalnız takım lideri taşır |
| `support` Destek talepleri | Yeni → İnceleniyor (5) → Yanıt bekleniyor → Çözüldü | Acil, Fatura, Hesap, Hata | Destek talebi | İnceleniyor'a girerken atanan kişi zorunlu |

Ortak kurallar:

- **Kolon ayarları:** Her şablonda ilk kolonda kart açılabilir, son kolon bitti kolonudur. Diğer kolonlarda kart açılamaz. Bu bugünkü varsayılanla aynıdır.
- **Etiket renkleri:** Yalnız mevcut paletten seçilir (`#e03131`, `#f08c00`, `#2f9e44`, `#1971c2`, `#7048e8`, `#c2255c`, `#0c8599`, `#495057`).
- **Atanan kişi ve roller:** Hiçbir şablonda atanan kişi ya da board rolü yoktur, çünkü şablon takımın üyelerini bilemez. Taşıma kurallarında yalnız "takım lideri", "atanan kişi" ve "herhangi bir üye" kullanılır.
- **Kart şablonlarının içeriği:** Açıklama iskeletleri Markdown'dır. Örneğin hata raporunda "Adımlar / Beklenen / Gerçekleşen / Ortam" başlıkları kalın satırlar olarak yer alır, çünkü uygulamanın Markdown'ı başlık desteklemez. Kontrol listeleri 3–5 maddedir.
- **Öncelik:** Hata raporu, Destek talebi ve Hata kart şablonları "Yüksek" öncelikle gelir, diğerleri öncelik vermez.
- **Bitiş süresi:** Blog yazısı 7 günlük bitiş süresiyle gelir (`DueInDays`). Diğer şablonlarda bitiş süresi yoktur.

## 3. Paket: `internal/blueprint`

```go
type Blueprint struct {
	Key       string
	Columns   []Column
	Labels    []Label
	Templates []CardTemplate
	Rules     Rules
}

type Column struct {
	Key         string // blueprints.<bp>.columns.<key>
	WIP         int    // 0: limitsiz
	AllowCreate bool
	Done        bool
}

type Label struct{ Key, Color string } // blueprints.<bp>.labels.<key>

type CardTemplate struct {
	Key       string // blueprints.<bp>.templates.<key>.{name,title,description,checklist.1…n}
	Column    int    // kolon indeksi
	Priority  *int16
	DueInDays *int
	Labels    []int // etiket indeksleri
	Checklist int   // madde sayısı
	Schedule  store.Schedule
}

type Rules struct {
	PersonWIP   int // 0: yok
	Permissions []Permission
	Conditions  []Condition
}

type Permission struct {
	To      int
	From    *int
	Subject string // "team_lead" | "assignee" | "any_member"
}

type Condition struct {
	Column int
	Phase  string // "enter" | "exit"
	Kind   string // store'daki koşul türlerinden biri
}
```

- **Fonksiyonlar:**
  - `blueprint.All() []Blueprint` kataloğu §2'deki sırayla verir.
  - `blueprint.Find(key) (Blueprint, bool)` anahtarla bir şablon bulur.
  - `blueprint.Default` `"simple"`dır.
- **Dile çevirme:** `blueprint.Render(bp, t func(key string) string) Rendered` her metni verilen dilde doldurur. Rendered, store'un kurulum için beklediği somut değerleri taşır: adlar, açıklamalar, kontrol listesi maddeleri.
- **Metinler:**
  - Şablonların içerik metinleri `locales/tr.json` ve `locales/en.json` içinde `blueprints.<bp>.*` altında durur.
  - Şablon seçicide görünen ad ve tek cümlelik açıklama `blueprints.<bp>.name` ve `blueprints.<bp>.summary` anahtarlarındadır.
- **Dilin sabitlenmesi:** Board, kuran kişinin dilinde kaydedilir; sonradan dil değişirse çevrilmez.

## 4. Kurulum: store

`store.CreateBoardFromPlan(ctx, teamID int64, name string, plan BoardPlan, actorID int64) (Board, error)` her şeyi tek bir veritabanı işleminde kurar. Bir adım başarısız olursa hiçbir şey kalmaz.

```go
type BoardPlan struct {
	Columns   []PlanColumn   // Name, WIP *int, AllowCreate, Done
	Labels    []PlanLabel    // Name, Color
	Templates []PlanTemplate // TemplateInput gibi; ColumnIndex ve LabelIndexes ile
	PersonWIP *int
	Permissions []PlanPermission // ToIndex, FromIndex *int, Subject
	Conditions  []PlanCondition  // ColumnIndex, Phase, Kind
}
```

- **Planın kurulması:** `blueprint` paketi, `Rendered` ile kullanıcının seçimlerinden bir `BoardPlan` kurar: `blueprint.Plan(rendered, Options)`.
- **Seçeneklerin etkisi:**
  - **WIP kapalıysa:** kolonların WIP limiti ve kişi başı WIP boştur.
  - **Etiketler kapalıysa:** etiket yoktur ve kart şablonlarının etiket indeksleri düşer.
  - **Kart şablonları kapalıysa:** şablon yoktur, dolayısıyla tekrar da yoktur.
  - **Tekrarlar kapalıysa:** şablonların zamanlaması `Kind: ""` olur.
  - **Kurallar kapalıysa:** izin, koşul ve kişi başı WIP yoktur.
- **Tek kurulum yolu:** Bugünkü `CreateBoard(ctx, teamID, name, columns)` imzası korunur. Testler ona bağlıdır. İçeride kolon adlarından bir `BoardPlan` kurup `CreateBoardFromPlan`'ı çağırır. Böylece tek kurulum yolu kalır.
- **Ortak yardımcılar:** `CreateLabel`, `saveTemplate`, `AddMovePermission`, `AddCondition` ve `SetBoardPolicy` fonksiyonlarının içi, dışarıdan verilen bir `pgx.Tx` ile çalışan yardımcılara ayrılır. Dışarıdan görünen davranışları değişmez, doğrulama kodu iki yolda ortaktır.
- **Tekrarın başlangıcı:** Tekrarlı şablonların `schedule_since` değeri kurulum anıdır (`now()`, bugünkü `saveTemplate` gibi). Geçmiş tarihler için kart açılmaz.
- **Geçmiş kaydı:** Kart şablonları `actorID` ile yazılır. Board için yeni bir geçmiş türü eklenmez.

## 5. Arayüz: takım sayfasındaki "Board oluştur" formu

```
Board adı  [______________________]

Şablon
( ) Basit              (•) Scrum              ( ) Hata takibi       …
    3 kolon                5 kolon · 4 etiket     5 kolon · 4 etiket
                           3 kart şablonu         1 kart şablonu
                           2 kural                2 kural

Aktarılacaklar
[x] WIP limitleri  [x] Etiketler  [x] Kart şablonları  [x] Tekrarlar  [x] Kurallar

                                                   [ + Board oluştur ]
```

- **Şablon seçici:**
  - Şablonlar bir radyo kartı ızgarasıdır (`fieldset`, `input type=radio name=blueprint`).
  - Her kartta adı, tek cümlelik açıklaması ve sayılar görünür: kolon, etiket, kart şablonu, kural.
  - Kolon adları kartın içinde küçük bir satırda okunur ("Backlog → Sprint → …").
  - Varsayılan seçim Basit'tir.
- **Aktarılacaklar:** Bunlar bir `fieldset` içindeki onay kutularıdır (`include=wip|labels|templates|recurring|rules`) ve hepsi varsayılan işaretlidir.
- **İşaretsiz kutular:** İşaretsiz onay kutusu forma hiç gelmez. Bu yüzden form gizli bir `include_present=1` alanı da gönderir:
  - Bu alan varsa yalnız işaretli `include` grupları kurulur.
  - Alan yoksa, örneğin istek eski bir sayfadan geliyorsa, şablonun her şeyi kurulur.
- **JavaScript olmadan:** Form eksiksiz çalışır. Bağımlılıklar sunucuda uygulanır: kart şablonu yoksa tekrar da yoktur.
- **`blueprint-form.js`:**
  - Seçilen şablonda olmayan grupların kutularını devre dışı bırakır. Örneğin Basit'te hepsi devre dışıdır, Hata takibi'nde Tekrarlar devre dışıdır.
  - "Kart şablonları" kapanınca "Tekrarlar"ı da kapatıp devre dışı bırakır.
- **Ölçüler:** Ölçüler rem'dir. Kartlar `--sheet` zeminli, seçili kart `--accent` kenarlı ve `--accent-soft` zeminlidir. Dar ekranda ızgara tek sütuna iner.
- **Doğrulama:**
  - Ad zorunludur, en fazla 100 karakterdir (bugünkü gibi).
  - Bilinmeyen bir `blueprint` değeri alan hatası verir ("Bu şablon yok"); form girilen değerleri koruyarak geri döner.
  - Bilinmeyen `include` değerleri yok sayılır.
- **Yeni çeviri anahtarları:**
  - `board.blueprint` ve `board.include.*` (alan adları);
  - `board.blueprint_unknown` (hata mesajı);
  - sayıların çoğul biçimleri.

## 6. Hatalar

- **Kurulum başarısız olursa:** İşlem geri alınır ve hata üst katmana iletilir; uygulama bugünkü gibi 500 sayfası gösterir. Katalog sabit olduğu için normalde böyle bir hata beklenmez, testler her şablonu kurar.
- **Aynı ad:** Aynı takımda aynı adla board açılabilir, bu bugün de böyle. Şablon bunu değiştirmez.

## 7. Test

- **`internal/blueprint`:**
  - Her şablon her dilde (tr, en) eksiksiz metne çıkar: boş ad ve eksik anahtar yoktur, çünkü çeviriler katı modda yüklenir.
  - Kural ve şablon indeksleri gerçekten var olan kolonları ve etiketleri gösterir.
  - Etiket renkleri paletin içindedir.
  - Her şablonda tam bir bitti kolonu vardır ve son kolondur.
  - `Plan` her seçenek kombinasyonunda §4'teki kuralları uygular: etiket yoksa şablon etiketi de yoktur, kart şablonu yoksa tekrar da yoktur.
- **store** (veritabanıyla):
  - Her şablon tam seçenekle kurulur; kolon, WIP, etiket, şablon, zamanlama ve kural sayıları beklenenle aynıdır.
  - Seçenekler kapatılınca ilgili parçalar oluşmaz.
  - Yarıda başarısız olan bir plan hiçbir satır bırakmaz. Bunun için geçersiz bir koşul türü verilir.
  - `CreateBoard` bugünkü testleriyle aynı sonucu verir.
- **web:**
  - Form şablon ve seçeneklerle gönderilir, board'a yönlendirir ve kolonlar o dilde kurulur.
  - Kutuların hepsi kaldırılarak gönderilen form (`include_present=1`, `include` yok) yalnız kolonları kurar.
  - `include_present` olmadan gönderilen istek, örneğin eski bir sayfadan gelen form, şablonun her şeyini kurar.
  - Bilinmeyen bir şablon reddedilir.
  - Lider olmayan kullanıcı formu bugünkü gibi göremez ve gönderemez.
- **Testlerin çalıştırılması:** `KANBAN_TEST_DATABASE_URL` tanımlıyken `go test ./... -race -count=1` çalıştırılır. Tanımlı değilse veritabanı testleri atlanır.

## 8. Kapsam dışı

- Kullanıcının kendi şablonunu kaydetmesi.
- Var olan board'a şablon uygulamak.
- Tek tek parça seçimi (etiket etiket, kural kural).
- Şablonlarda board rolleri ve atanan kişi.
