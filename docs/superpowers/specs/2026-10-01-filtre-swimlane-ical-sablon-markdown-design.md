# Filtre, swimlane, iCal, şablon ve Markdown — tasarım

Tarih: 2026-10-01 · Durum: onaylandı (sohbette), yazılı hali gözden geçirilecek

Kanban uygulamasına beş özellik ekler. Mimari aynı kalır: collage ile sunucu tarafında render, collage-live ile push, istemcide state kopyası yok, CSP uyumlu, TR + EN. Kural motoru (`internal/rules`) yeni bir kural almaz. Var olan kurallar yeni yollarda da (şerit değiştiren taşıma, şablondan kart) aynen uygulanır.

`kanban-spec.md` §13'te kapsam dışı sayılan swimlane'ler bu tasarımla kapsama girer. "İstemci tarafında anlık filtre" kapsam dışı kalır. Buradaki filtre sunucuda uygulanır. Aşama 1'in sonunda §13 buna göre güncellenir.

## 1. Kullanıcının seçimleri

- Filtreye uymayan kartlar **soluklaşır**, gizlenmez.
- Arama hem **board içinde** (filtrenin parçası) hem **genel** (`/search`) yapılır.
- Swimlane'ler bir **görünüm**dür. **Şeritler arasında sürüklemek kartın alanını değiştirir** (atanan kişi ya da öncelik).
- iCal: **kişisel** ve **board** beslemesi.
- Tekrar **şablona bağlıdır**. Şablon elle de kullanılabilir.
- Markdown **açıklamada ve yorumlarda** geçerlidir.
- Yaklaşımlar:
  - filtre sunucuda uygulanır ve fragment URL'sine taşınır;
  - arama `pg_trgm` + `ILIKE` ile yapılır;
  - Markdown goldmark + bluemonday ile render edilir.

## 2. Aşamalar

Her aşama tek başına deploy edilebilir. Her aşamanın sonunda durulur.

1. Filtre ve arama (§3), `TIMEZONE` ortam değişkeni (§6.4) bu aşamada eklenir
2. Swimlane'ler (§4)
3. iCal beslemesi (§5)
4. Şablonlar ve tekrar (§6)
5. Markdown (§7)

## 3. Filtre ve arama

### 3.1 Board filtresi

Board başlığının altında bir filtre çubuğu bulunur. Çubuk `GET` formudur ve JS olmadan da çalışır. Parametreler:

| Parametre  | Değerler                                                      | Birleşim                  |
| ---------- | ------------------------------------------------------------- | ------------------------- |
| `q`        | metin; başlık, açıklama ve yorumlarda arar                    | —                         |
| `assignee` | kullanıcı id'si, `me`, `none`; birden çok olabilir             | kendi içinde "veya"        |
| `label`    | etiket id'si; birden çok olabilir                             | kendi içinde "herhangi biri" |
| `priority` | `1`–`4`; birden çok olabilir                                  | kendi içinde "veya"        |
| `due`      | `overdue`, `today`, `week`, `none`                            | tek değer                 |

- Boyutlar birbirleriyle "ve" mantığıyla birleşir.
- `q`, 2 karakterden kısaysa yok sayılır.
- Bilinmeyen ya da hatalı bir değer sessizce yok sayılır, 400 dönülmez. Böylece eski bir bağlantı kırılmaz.
- `due` hesaplanırken "bugün" ve "bu hafta" uygulamanın saat dilimine göre belirlenir (§6.4 `TIMEZONE`). Hafta pazartesi başlar.
- Parametreler sayfa URL'sinde ve board fragment'inin URL'sinde durur:
  `data-collage-fragment="{{fragmentURL …}}?{{.FilterQuery}}"`.
  Paylaşılmayan bir render her bağlantı için o bağlantının isteğiyle yapıldığından, her okuyucu kendi filtresiyle render edilmiş HTML'i alır.
- Uymayan kart `card--dimmed` sınıfıyla render edilir: soluk görünür ama tıklanabilir ve sürüklenebilir. Kolon sayaçları ve WIP göstergeleri gerçek değeri gösterir.
- Çubukta "N karttan M'si eşleşiyor" sayacı ve bir "Temizle" bağlantısı bulunur. Filtre etkin değilken sayaç gösterilmez.
- JS ile:
  - metin kutusu yazmayı bitirince formu gönderir (300 ms bekleme), öbür denetimler değişince formu gönderir;
  - URL `history.replaceState` ile güncellenir;
  - fragment'in `data-collage-fragment`'i yeni sorguyla değiştirilir ve `collageLive.refresh` çağrılır.
- Gantt sayfası da aynı parametreleri kabul eder ve uymayan çubukları soluklaştırır.

**Varsayım (Aşama 1'in ilk adımında doğrulanır):** collage-live'ın `RenderFragment(r, FragmentRequest{Path: url})` çağrısı URL'deki sorgu dizgisini render isteğine taşır. Taşımıyorsa framework issue'su açılır ve iş durur.

### 3.2 Genel arama

- Sayfa: `/search` ve `/en/search`. Kenar çubuğunda bir arama kutusu olur, sonuçlar `q` parametresiyle bu sayfada gösterilir.
- Kapsam: kullanıcının üyesi olduğu takımların arşivlenmemiş board'ları. Admin de yalnız kendi takımlarını görür, bugünkü erişim kuralı korunur.
- Aranan alanlar kart başlığı, açıklama ve yorumlardır. Tamamlanmış ve arşivlenmiş kartlar da sonuçlarda çıkar ve bir rozetle işaretlenir.
- Sonuçlar board'a göre gruplanır. Her sonuçta kart başlığı ve eşleşen parçanın vurgulu bir alıntısı bulunur. Alıntı en fazla 160 karakterdir, sunucuda escape edilir ve vurgu `<mark>` ile yapılır.
- Sıralama: önce başlıkta eşleşme, sonra `similarity`, sonra son güncelleme.
- Sayfa başına 50 sonuç gösterilir ve `page` parametresiyle sayfalanır.

### 3.3 Veri

Migration `008_search.sql`:

```sql
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX cards_title_trgm ON cards USING gin (lower(title) gin_trgm_ops);
CREATE INDEX cards_description_trgm ON cards USING gin (lower(description) gin_trgm_ops);
CREATE INDEX comments_body_trgm ON comments USING gin (lower(body) gin_trgm_ops);
```

- Büyük/küçük harf eşleşmesi `lower()` ile yapılır. Türkçe `İ/i` ve `I/ı` davranışı veritabanı collation'ına bağlıdır, testle sabitlenir. `lower()` collation'a göre doğru sonuç vermiyorsa, sorgu ve indeks aynı Go-tarafı normalleştirmeyi kullanan bir ifadeye çevrilir.
- `pg_trgm` kurulamazsa (yetki yoksa) migration hata verir ve uygulama başlamaz. README'de gereksinim olarak yazılır.

### 3.4 Store

- `BoardFilter` struct'ı, URL'den `ParseBoardFilter(url.Values, members, labels)` ile okunur.
- `MatchingCardIDs(ctx, boardID, filter) (map[int64]bool, error)` döner. Filtre boşsa sorgu çalışmaz.
- `Search(ctx, userID, q, page) ([]SearchHit, bool, error)` döner. İkinci değer "daha fazla sonuç var mı" bilgisidir.

### 3.5 Test

- Store: her filtre boyutu tek başına ve birlikte; `me`/`none`; `due`'nun gün sınırları; Türkçe harf eşleşmesi; görünmeyen takımın kartının sonuçlarda çıkmaması; arşivli board'un sonuçlarda çıkmaması; sayfalama.
- Web:
  - filtrelenmiş fragment URL'sine yapılan pushun filtreyi koruması (§3.1'deki varsayımın testi);
  - JS'siz `GET` yolu;
  - hatalı parametrelerle 200 dönmesi;
  - alıntının escape edilmesi.

## 4. Swimlane'ler

### 4.1 Görünüm

- Filtre çubuğunda bir "Şeritle" seçimi bulunur, değeri `lane` parametresinde tutulur: boş, `assignee` ya da `priority`. Filtreyle birlikte kullanılabilir ve fragment URL'sine aynı yoldan taşınır.
- Kolon başlıkları ve WIP göstergeleri en üstte bir kez gösterilir ve kolon genelindeki gerçek sayıyı yansıtır.
- Her şeridin başlığında şeridin adı ve kart sayısı bulunur. Başlık `<details>` ile katlanabilir, katlanma durumu saklanmaz.
- **`assignee`:** önce "Atanmamış" şeridi, sonra takım üyeleri ada göre sıralanır. Board'da açık kartı olmayan üye için şerit gösterilmez. Takımda olmayan ama kartı bulunan kişinin şeridi soluk görünür.
- **`priority`:** Acil, Yüksek, Orta, Düşük ve Yok. Boş şeritler gösterilmez.
- Kolon içindeki sıra değişmez. Her hücre, kolonun genel sırasının o şeride düşen kartlarından oluşur.

### 4.2 Şeritler arası sürükleme

- Her (şerit, kolon) hücresi ayrı bir Sortable listesidir ve hepsi aynı `group`'tadır. Pause, put ve resume akışı bugünkü gibi board fragment'i üzerinde yapılır.
- Taşıma isteğine iki alan eklenir:
  - `lane` ve `lane_value`: hedef şeridin türü ve değeri. Değer kullanıcı id'si, öncelik ya da `none`'dır. Şerit değişmediyse alan gönderilmez.
  - `before_card_id`: kartın üstüne bırakıldığı kart, şerit sonuna bırakıldıysa boş.
- Şerit görünümünde sunucu `ToIndex`'i `before_card_id`'den hesaplar:
  - `before_card_id` doluysa o kartın kolondaki indeksi kullanılır;
  - boşsa şeridin o kolondaki son kartının hemen arkası kullanılır;
  - şeridin o kolonda hiç kartı yoksa kolonun sonu kullanılır.
- `store.Move`'a isteğe bağlı bir `Set *LaneChange` alanı eklenir. Taşıma ve alan değişikliği aynı transaction'da, aynı board kilidi altında yapılır.
  - Kolon değiştiyse taşıma kuralları, son kartın alan değerleriyle değerlendirilir.
  - Atanan kişi değiştiyse `EvaluateAssign` ve kişi WIP'i değerlendirilir.
  - Kuralın biri bile çiğnenirse hiçbir şey değişmez ve bugünkü gibi 422 ile bütün ihlaller listelenir.
- Şerit hedefindeki kişi artık takım üyesi değilse istek 422 alır. Bunun için yeni bir ihlal metni eklenir.
- Activity akışına iki ayrı kayıt düşer: taşıma ve alan değişikliği. Bildirimler bugünkü atama bildirimiyle aynıdır.
- JS yoksa şeritler yalnız görünümdür. Kart sayfasındaki formlar bugünkü gibi çalışır.

### 4.3 Test

- Store:
  - taşıma ile şerit değişikliği birlikteyken kurallar ikisinden biri yüzünden reddedildiğinde hiçbir şeyin değişmemesi (kolon, sıra, alan);
  - kişi WIP'i;
  - takımda olmayan hedef kişi;
  - `before_card_id`'den indeks hesabı: başta, ortada, şerit sonunda, boş hücrede, başka şeritten bir `before_card_id` geldiğinde;
  - bayat sürüm için 409.
- Web: `lane` parametresiyle render, filtreyle birlikte kullanım, katlanmış şeritte bile kart sayısının doğru olması.

## 5. iCal beslemesi

### 5.1 Token

- `users` tablosuna `calendar_token_hash bytea UNIQUE` eklenir (migration `009_calendar.sql`).
- Token 32 bayt rastgele değerin base64url biçimidir. Veritabanında yalnız SHA-256 özeti saklanır.
- `/me/settings` sayfasında bir "Takvim" bölümü bulunur. Bölüm üç işlem sunar:
  - **Oluştur:** token yokken görünür.
  - **Sıfırla:** onay ister ve eski adresi hemen geçersiz kılar.
  - **Kapat:** onay ister ve token'ı siler.
- Adres yalnız oluşturma ya da sıfırlama anında, flash ile bir kez gösterilir. Kişisel adres ve board adres kalıbı birlikte gösterilir.

### 5.2 Adresler

`app.Handle` ile bağlanır, yalnız `GET` ve `HEAD` kabul eder. Oturum ve CSRF yoktur, kimliği token belirler. `kanban-spec.md` §2 madde 5 gereği bu yolda cache ve gövde limiti yoktur. Gövde zaten yoktur, cache başlıkları elle yazılır.

- `/cal/{token}/me.ics`: bana atanmış kartlar. Yalnız erişebildiğim board'lardaki kartlar girer: takım üyeliği istek anında kontrol edilir.
- `/cal/{token}/boards/{id}.ics`: board'un kartları. Üye değilsem ya da board arşivliyse 404 döner.
- Token geçersizse ya da kullanıcı devre dışıysa 404 döner.
- Board sayfasındaki menüde "Takvime abone ol" öğesi bulunur. Kullanıcının token'ı varsa adres kalıbını gösterir, yoksa ayarlara götürür. Token'ın açık hali saklanmadığından adres bu menüde tam olarak gösterilemez.

### 5.3 İçerik

- Yalnız son tarihi olan, tamamlanmamış ve arşivlenmemiş kartlar beslemeye girer.
- Her kart bir `VEVENT` olarak yazılır:
  - `UID`: `card-{id}@{BASE_URL host}`.
  - `DTSTART;VALUE=DATE`: başlangıç tarihi, yoksa son tarih.
  - `DTEND;VALUE=DATE`: son tarihin ertesi günü (bitiş günü dahil değildir).
  - `SUMMARY`: `[Board adı] Kart başlığı`.
  - `DESCRIPTION`: kart bağlantısı (kullanıcının dilinde) ve ardından açıklamanın ham metni.
  - `URL`: kart bağlantısı.
  - `DTSTAMP`: yanıtın üretildiği an. `LAST-MODIFIED`: kartın activity akışındaki son kaydın zamanı, kayıt yoksa `created_at`. Kartlarda `updated_at` sütunu yoktur ve eklenmez.
  - `PRIORITY`: Acil→1, Yüksek→3, Orta→5, Düşük→7.
- Takvim başlığı `X-WR-CALNAME` ile kullanıcının dilinde verilir: "Kanban — bana atananlar" ya da "Kanban — {board}".
- Kodlama `internal/ical` paketinde saf Go ile yapılır, bağımlılık eklenmez:
  - satır sonu CRLF;
  - satırlar 75 oktette katlanır, çok baytlı UTF-8 karakter bölünmez;
  - `\`, `,`, `;` ve satır sonu escape edilir.
- Yanıt başlıkları: `Content-Type: text/calendar; charset=utf-8`, `Cache-Control: private, max-age=300`, içerikten üretilen `ETag`. `If-None-Match` geldiğinde 304 döner.

### 5.4 Test

- `internal/ical`: katlama (çok baytlı karakter sınırda), escape, altın dosya.
- Web:
  - sıfırlamadan sonra eski token'ın 404 alması;
  - kapatmadan sonra 404;
  - takımdan çıkarılan kişinin board beslemesinde 404 alması ve kişisel beslemede o board'un kartlarının görünmemesi;
  - tamamlanmış ve arşivlenmiş kartların beslemeye girmemesi;
  - başka takımın board'unda 404;
  - 304;
  - `POST` isteğinde 405.

## 6. Şablonlar ve tekrar

### 6.1 Veri

Migration `010_templates.sql`:

- `card_templates`: `id`, `board_id`, `name` (board içinde benzersiz), `title`, `description`, `priority`, `estimate`, `assignee_id`, `column_id` (`ON DELETE SET NULL`), `due_in_days` (≥ 0, boş olabilir), `updated_by`, `updated_at`, zamanlama alanları (§6.3), `schedule_since`.
- `card_template_labels (template_id, label_id)`: iki taraf da `ON DELETE CASCADE`.
- `card_template_checklist (template_id, position, text)`.
- `template_runs`:
  - alanlar: `template_id`, `scheduled_for timestamptz`, `status` (`created` | `failed`), `card_id`, `violations jsonb`, `created_at`;
  - birincil anahtar: `(template_id, scheduled_for)`.

### 6.2 Yönetim ve elle kullanım

- Board ayarlarına bir "Şablonlar" sekmesi eklenir. Diğer ayarlar gibi bu sekmeyi de lead'ler yönetir.
- Hedef kolonu boşalmış bir şablon uyarıyla gösterilir ve zamanlaması çalışmaz.
- Board'daki yeni kart formuna isteğe bağlı bir "Şablondan" seçimi eklenir. Seçildiğinde kart şablonun hedef kolonuna açılır. Formda kolon seçilmişse o kolona açılır.
- Kart normal oluşturma yolundan açılır (`allow_create`, giriş koşulları, WIP). Şablon alanları aynı transaction'da doldurulur, son tarih açılış gününe `due_in_days` eklenerek bulunur.
- Atanan kişi kişi WIP'ini aşıyorsa ya da takım üyesi değilse kart atanmamış açılır ve formda bir uyarı gösterilir. Kart açmanın kendisi reddedilirse 422 ile ihlaller gösterilir.

### 6.3 Zamanlama

- Alanlar:
  - `schedule_kind`: `daily`, `weekly`, `monthly` ya da boş.
  - `schedule_weekdays`: bit maskesi, pazartesi = bit 0.
  - `schedule_monthday`: 1–31. Kısa aylarda ayın son günü kullanılır.
  - `schedule_time`: `time`, varsayılan 09:00.
- Saf bir fonksiyon, `internal/schedule.Latest(sched, loc, now) time.Time`, `now`'dan önceki ya da `now`'a eşit en son zamanlanmış anı döndürür. Yaz saati geçişinde var olmayan saat ileri kaydırılır, iki kez yaşanan saatte ilki alınır.
- `schedule_since`, zamanlamanın açıldığı ya da değiştirildiği andır. Bu anın öncesi doldurulmaz.
- `notify` paketinin yanına bir `Recurrer` eklenir, mevcut `every()` döngüsüyle 5 dakikada bir çalışır. Her zamanlanmış şablon için:
  1. `t := Latest(...)`. `t < schedule_since` ise atlanır.
  2. Transaction'da `INSERT INTO template_runs … ON CONFLICT DO NOTHING` çalıştırılır. Satır eklenmediyse şablon bu an için zaten işlenmiştir, atlanır.
  3. Kart, §6.2'deki yoldan açılır. `created_by` değeri `updated_by` olur. Activity'ye "şablondan zamanlanarak açıldı" kaydı düşer.
  4. Kurallar reddederse ya da `updated_by` artık takımda değilse, run `failed` olarak ihlallerle kaydedilir.
     - Yeni bildirim türü `template_failed` gönderilir: uygulama içinde ve e-posta tercihine göre e-postayla. Alıcı `updated_by`'dır, o takımda değilse takımın lead'leridir.
     - Ayarlar sekmesinde şablonun yanında son çalışmanın durumu gösterilir.
- Yalnız en son an işlenir. Sunucu üç gün kapalı kaldıysa yalnız bir kart açılır.
- Board arşivliyse şablon atlanır ve run kaydı yazılmaz.

### 6.4 `TIMEZONE`

İsteğe bağlı bir ortam değişkenidir, IANA adı alır (örn. `Europe/Istanbul`). Verilmezse sunucunun yerel saati kullanılır. Hatalı bir değer config doğrulamasında adıyla raporlanır ve uygulama başlamaz. Bu değişken zamanlamada ve §3.1'deki `due` filtresinde kullanılır. Son tarih hatırlatmalarına dokunulmaz.

### 6.5 Test

- `internal/schedule`:
  - günlük;
  - haftalık, birden çok gün ile;
  - aylık 31 (şubat, nisan), 29 şubat;
  - `Europe/Istanbul` ve yaz saati geçişi olan bir dilim (`Europe/Berlin`);
  - `now` tam zamanlanmış anda.
- Store ve worker:
  - iki eşzamanlı tick'te tek kart açılması;
  - ret durumunda `failed` kaydı ve bildirim;
  - kaçırılan anlarda tek kart;
  - takımdan ayrılan `updated_by` (lead'lere bildirim);
  - silinen hedef kolon;
  - silinen etiket;
  - arşivli board.
- Web: lead dışı birinin şablonu düzenleyememesi; elle şablondan kart açma, atanan kişi düşürüldüğünde uyarı.

## 7. Markdown

### 7.1 Render

- `internal/markup` paketinde `Markdown(s string) template.HTML` bulunur. Kart açıklamasında ve yorumlarda `richText`'in yerini alır. Şablon fonksiyonunun adı `markdown`'dır.
- goldmark ayarları:
  - GFM: tablo, üstü çizili, otomatik link, görev listesi;
  - `html.WithHardWraps()`, bugünkü "satır sonu korunur" davranışı sürsün diye;
  - ham HTML kapalı (`WithUnsafe` yok);
  - başlıklar bir seviye aşağı kaydırılır, `h6`'nın altına inmez.
- Çıktı bluemonday politikasından geçer:
  - izinli öğeler: `p br strong em del code pre blockquote ul ol li h2–h6 table thead tbody tr th td hr a`, `input` (yalnız `type="checkbox"` ve `disabled`);
  - `a[href]` yalnız `http`, `https` ve `mailto` şemalarını alır, her bağlantıya `rel="noopener noreferrer nofollow"` eklenir;
  - `img`, `style` ve `class` öznitelikleri izinli değildir. Tablo hizalaması için goldmark'ın `style` çıktısı atılır.
- Anmalar bugünkü gibi düz metin kalır.
- `.rich` CSS'ine liste, kod, alıntı ve tablo stilleri eklenir. Tablolar kendi kutusunda yatay kayar, sayfa kaymaz.

### 7.2 Düzenleme

Textarea'lar olduğu gibi kalır. Altlarında kısa bir "Markdown desteklenir" ipucu ve sözdizimi örneklerini açan bir `<details>` bulunur. Önizleme yoktur.

### 7.3 Değişmeyenler

- Metinler veritabanında ham olarak durur. Migration yoktur.
- E-postalar, iCal açıklaması, activity alıntıları ve arama alıntıları ham ya da düz metindir.
- Bağımlılık olarak `github.com/yuin/goldmark` ve `github.com/microcosm-cc/bluemonday` eklenir.

### 7.4 Test

- XSS tablosu: `<script>`, `<img onerror>`, `javascript:` ve `JaVaScRiPt:` bağlantıları, entity ile gizlenmiş şema, `data:` URI, ham HTML bloğu, satır içi ham HTML, iç içe `[x](javascript:…)`, autolink ile `javascript:`. Hiçbiri çalıştırılabilir çıktı üretmemeli.
- GFM öğelerinin altın çıktıları; satır sonunun korunması; başlık kaydırma.
- `text_test.go`'daki bağlantı ve escape beklentilerinin `markup` testlerine taşınması. `richText` başka bir yerde kullanılmıyorsa silinir.
