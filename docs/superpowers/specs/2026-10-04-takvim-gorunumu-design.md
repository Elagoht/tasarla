# Board takvim görünümü — tasarım

Tarih: 2026-10-04 · Durum: tasarım sohbette onaylandı, yazılı hali gözden geçirilecek

Board'un kartları bugün tarihleriyle yalnız Gantt'ta görülüyor. Gantt bir zaman çizelgesidir; "bu ay hangi gün ne var" sorusuna bakmak için ay takvimi daha doğal. Gantt'ın yanına, aynı kart tarihlerini ve aynı filtreleri kullanan bir Takvim sekmesi ekliyoruz.

## 1. Kullanıcının seçimleri

- **Etkileşim:** okuma + sürükleyerek tarih değiştirme + boş güne tıklayıp kart açma.
- **Aralık:** yalnız ay görünümü (önceki / bugün / sonraki). Hafta görünümü sonraya.
- **Yeni kartın kolonu:** board'un kart açılabilen ilk kolonu; yalnız başlık sorulur, bitiş tarihi tıklanan gün olur.
- **Yaklaşım:** Gantt'ın yapısını kopyala, altyapıyı paylaş. Gantt sayfasına üçüncü ölçek eklenmez, istemci tarafı takvim kütüphanesi kullanılmaz.

## 2. Sayfa ve gezinme

- Yeni sayfa `board-calendar`, yol `/boards/{id}/calendar`; parça `board-calendar-grid`, yol `/boards/{id}/calendar/grid`. Kuruluş `boardGanttPage` ile aynıdır: başlık, sekmeler ve filtre çubuğu sayfa parçasında, ızgara slot'taki dinamik parçada. Ay değişimi ve filtre yalnız ızgarayı yeniler (`data-collage-fragment`, `data-collage-push`).
- `board_tabs.html`'e Gantt'tan hemen sonra "Takvim" sekmesi eklenir (`boardTabs(id, "calendar")`). Takvim sekmesi `calendar` ikonunu alır; Gantt için `icons.html`'e yatay çubuklardan oluşan yeni bir `gantt` ikonu eklenir.
- Başlıkta: ay adı (yerele göre, ör. "Ekim 2026"), ‹ önceki, Bugün, sonraki › bağlantıları ve Gantt'takiyle aynı "bitenleri göster" düğmesi.
- Ay `?month=YYYY-MM` ile gelir. Yoksa ya da geçersizse uygulamanın saat dilimindeki (`h.loc`) bugünün ayı kullanılır. Ay ve `done` filtre çubuğunda gizli alan olarak taşınır (`fv.Extra`), böylece yeni bir filtre ayı kaybetmez.

## 3. Izgara düzeni

Saf Go fonksiyonu, HTTP olmadan test edilir:

```go
func layoutCalendar(month time.Time, cards []store.Card, now time.Time, lanes int) calendarView
```

- Haftalar Pazartesi başlar. Izgara ayın ilk gününü içeren haftadan son gününü içeren haftaya kadardır (5–6 satır); komşu aylardan gelen günler soluk gösterilir ama üzerlerindeki kartlar da görünür.
- Kartın günleri `span()` ile bulunur (başlangıç+bitiş, yalnız bitiş ya da yalnız başlangıç). Tarihi olmayan kart takvimde yoktur.
- Birden çok güne yayılan kart haftanın içinde günleri boyunca uzanan bir çubuktur. Hafta sınırını aşan kart her hafta için bir parçaya bölünür; parçalar devam ettiğini köşesiz kenarla belli eder.
- Her hafta içinde kartlara şerit (lane) verilir: önce erken başlayan, eşitse uzun süren. Bir kart haftası boyunca aynı şeritte kalır, çubuklar üst üste binmez.
- Bir günde `lanes` (3) şeritten fazlası varsa o gün "+k daha" bağlantısı gösterir; tıklanınca o günün bütün kartlarının listesi açılır (`<details>` tabanlı açılır kutu, JS gerektirmez).
- Gecikmiş kart (bitiş tarihi bugünden önce, bitmemiş) Gantt'taki "geç" görünümünü alır. Bugünün hücresi işaretlenir.
- Kartlar `GanttCards(boardID, done)` ile yüklenir ve board filtresinden geçer; Gantt'ın `loadGantt`'ındaki eşleşme mantığı ortak bir yardımcıya çıkarılarak iki görünümde de kullanılır.

## 4. Etkileşimler — `static/js/calendar.js`

- **Karta tıklama:** kartı board'da ve Gantt'ta açıldığı gibi açar (aynı bağlantı).
- **Sürükleme:** kart başka bir güne bırakılınca başlangıç ve bitiş aynı gün sayısı kadar kayar ve kartın `set_dates` işlemiyle, ızgaranın çizildiği sürümle (`expected_version`) kaydedilir. Yalnız tek tarihi olan kartta o tarih kayar. Yanıtlar Gantt'la aynı ele alınır: 409 → çakışma bildirimi, 422 → sıra hatası bildirimi, diğerleri → genel hata; her durumda ızgara yenilenir ve kart yeniden odaklanır.
- **Klavye:** odaklanmış kartta ← / → bir gün, ↑ / ↓ bir hafta kaydırır; kayıt sürüklemeyle aynıdır.
- **Kart açma:** düzenleme yetkisi olan kullanıcı bir günün boş yerine tıklayınca (ya da hücredeki "+" düğmesine basınca) hücrede tek satırlık başlık alanı açılır. Enter gönderir, Esc vazgeçer. Form JS olmadan da çalışır: her hücrede gizli bir `<form>` vardır, JS yalnız gösterir ve gönderimi fetch ile yapar.
- `CanEdit` olmayan kullanıcı ızgarayı görür; sürükleme, klavye ile kaydırma ve kart açma yoktur. (Bugün takım üyesi her zaman düzenleyebilir ve arşivli board 404 döner, dolayısıyla bu kapı ileriye dönüktür.)

## 5. Kart açma işlemi

- Takvim sayfasının eylemi: `op=create_card`, alanlar `title` (zorunlu, en çok 200) ve `due` (`YYYY-MM-DD`).
- Hedef kolon `creatableColumns(cols)[0]`: kart açılabilen ilk kolon, hiçbiri işaretli değilse board'un ilk kolonu (board'daki "kart ekle" ile aynı). 409 yalnız board'da hiç kolon yoksa.
- Yeni store fonksiyonu `CreateCardDue(ctx, boardID, columnID, title, due, createdBy)`: `CreateCard` ile aynı işlem içinde kartı bitiş tarihiyle oluşturur, böylece `rules.EvaluateCreate`'e giden anlık görüntüde tarih bulunur ve kolonun giriş kuralları (ör. "bitiş tarihi zorunlu") onu görür. Ortak kod `CreateCard` ile paylaşılır.
- Kural ihlali `violationMessages` ile 422 ve bildirim olarak döner; boş başlık alan hatasıyla reddedilir. Başarıda ızgara parçası döner ve `boardTag` geçersiz kılınır; atanma bildirimi `createCard`'daki gibi gönderilir.

## 6. Hatalar ve uç durumlar

- Geçersiz `month` → bu ay. Geçersiz `due` → 400.
- Silinmiş kolon ya da board → 400 / 404, mevcut desenle.
- Izgaranın dışında kalan kartlar yüklenir ama çizilmez; `GanttCards` zaten board'un bütün tarihli kartlarını tek sorguda getirir.

## 7. Çeviri ve stil

- `calendar_view.*` anahtarları `tr` ve `en` için (`calendar.*` iCal aboneliğine ait): başlık, sekme, önceki / sonraki / bugün, "+%d daha", yeni kart alanı ve hata bildirimleri.
- Hafta günü ve ay adları yerelleştirilir (sabit tablo, `time.Format` İngilizce verdiği için).
- `static/css/calendar.css`: ızgara CSS grid ile; ölçüler rem; renkler mevcut tema değişkenleriyle (açık ve gece moru). Dar ekranda ızgara yatay kaydırılır, hücre en az genişliği korunur.

## 8. Test

- `layoutCalendar` birim testleri: başka ayda başlayan hafta, hafta sınırını aşan kart, şerit yerleşimi, taşma ("+k daha"), yalnız bitiş / yalnız başlangıç tarihli kart, gecikmiş kart, 6 satırlık ay.
- `calendar_page_test.go` (`gantt_test.go` deseniyle): sayfa çizilir, `month` parametresi çalışır, filtre uygulanır, `done` düğmesi çalışır, `create_card` bitiş tarihiyle kart açar, giriş kuralı ihlali 422, dışarıdaki kullanıcı 404.
- `CreateCardDue` store testi: tarih kaydedilir, kural tarihi görür.

## Kapsam dışı

Hafta / gün görünümü, saatli kartlar, takvimde kart silme veya kolon değiştirme, birden çok board'u bir arada gösteren kişisel takvim.
