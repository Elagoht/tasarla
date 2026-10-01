# Arayüz yenileme — tasarım

Tarih: 2026-10-01 · Durum: onaylandı (sohbette), yazılı hali gözden geçirilecek

Kanban uygulamasının görsel dilini ve etkileşimlerini baştan kurar. İlk kullanımdaki geri bildirimleri de giderir. Mimari aynı kalır: collage ile sunucu tarafında render, az JS, CSP uyumlu, TR + EN. Veri modeli ve kural motoru (`internal/rules`) değişmez.

## 1. Amaç ve ölçüt

Ekip Vikunja'dan büyük ölçüde tasarımı yüzünden ayrılıyor. Yaşadıkları sorunlar:

- Düzenle ve sil düğmeleri yan yana ve aynı görünümde duruyordu.
- Silmek onay istemiyordu.
- Hiçbir öğe diğerinden ayrılmıyordu; ne kenarlık ne zemin vardı.

Yeni arayüz bunların tersini yapmalı.

**Başarı ölçütü:**
- Her öğe zemin, kenarlık ve gölgeyle ayrışır.
- Geri alınamaz işlemler görsel olarak ayrıdır ve onay ister.
- Kurallar okunarak anlaşılır.
- Kartta yazılan bir şey hiçbir etkileşimle kaybolmaz.

**Kullanıcının seçimleri** (görsel yardımcıda):
- Görsel yön: **C, sıcak kağıt**.
- Kural düzenleme: **cümle cümle liste**.
- Kart detayı: **sağdan açılan panel**.
- Gezinme: **sol kenar çubuğu**.

## 2. Davranış düzeltmeleri

### 2.1 Kendi rolünü düşürememe

- **Admin:** `/admin/users` sayfasında kendi satırındaki "Adminliği kaldır" ve "Devre dışı bırak" düğmeleri pasif görünür. Sunucu da reddeder: `op=set_admin` ve `op=set_disabled`'da hedef kendisiyse 403 ve bir mesaj döner.
- **Takım lideri:** takım sayfasında kendi satırındaki rol seçimi ve "Çıkar" düğmesi pasif görünür. Sunucu, `set_role` ve `remove_member`'da hedef kendisiyse reddeder.
- **Mevcut kurallar:** "son aktif admin kaldırılamaz" kuralı (store) yerinde kalır. Admin başka birinin adminliğini yine kaldırabilir.

### 2.2 Kart panelinde alan bazlı kayıt

Tek büyük güncelleme formu kalkar. Her alan kendi küçük formudur:

| Alan | Kayıt anı | İşlem |
| --- | --- | --- |
| Başlık | alan terk edilince ya da Enter'da | `op=set_field field=title` |
| Açıklama | alan terk edilince | `field=description` |
| Kolon | seçilince | `op=move` (kural motorundan geçer) |
| Atanan, son tarih, öncelik, tahmin | değişince | `field=assignee \| due_date \| priority \| estimate` |
| Etiketler | seçim değişince | `op=labels` |

- **Kayıt:** her istek `expected_version` taşır. Başarılı olursa alanın yanında "Kaydedildi ✓" görünür, panelin sürüm numarası güncellenir.
- **Kural ihlali** (kolon, atanan kişi için kişi WIP'i) ya da **sürüm çakışması** olursa alan eski değerine döner ve yanında sebep yazar. Diğer alanlarda yazılanlara dokunulmaz.
- **JS yoksa** her alan, "Kaydet" düğmeli kendi formudur ve karta geri yönlendirir. Yalnızca o alan gönderilir, başka alan kaybolmaz.
- **Sunucu:** `set_field` yalnızca istenen alanı değiştirir; diğerlerini kartın güncel halinden alır. Bunun için store'a `UpdateCardField` eklenir. Activity, notify ve kural (kişi WIP'i) davranışı `UpdateCard` ile aynıdır.

### 2.3 Kolonların toplu düzenlenmesi

- **Tablo:** Ayarlar → Kolonlar sekmesi tek bir tablodur. Sütunlar: sürükleme tutamacı, ad, WIP limiti, "kart burada açılır", "bitti", "kişi WIP'ine sayılır", sil.
- **Kaydetme:** "Kolon ekle" tabloya bir satır ekler. Tek "Kaydet" bütün değişiklikleri (sıra, ad, bayraklar, yeni ve silinen kolonlar) `store.SaveColumns` ile tek transaction'da, board kilidi altında uygular.
- **Doğrulama:**
  - Ad boş olamaz.
  - WIP pozitif bir tam sayı olmalı ya da boş kalmalı.
  - En az bir kolon kalmalı.
  - Kartı olan bir kolon silinemez.
  - Bir yetkinin "nereden" kolonu olan kolon silinemez.

  Hata varsa hiçbir şey kaydedilmez; tablo, girilen değerler ve satır bazlı hata mesajlarıyla yeniden gösterilir.
- **JS ile:** sürükleyerek sıralama (SortableJS), kaydedilmemiş değişiklik sayacı ve sayfadan çıkarken uyarı. **JS yoksa:** her satırda bir "sıra" sayı alanı vardır.

### 2.4 Board rolleri

Ayarlar → Roller sekmesinde her rol için ad (yeniden adlandırılabilir), takım üyelerinden seçilen üyeler ve sil düğmesi bulunur. Bir yetki rolü kullanıyorsa silme reddedilir ve sebebi yazılır (`store.ErrInUse`, zaten var). Store'a `RenameBoardRole` eklenir.

### 2.5 Kurallar: cümle listesi

Ayarlar → Kurallar sekmesi, board'un bütün kurallarını okunabilir cümleler olarak listeler. Her satırda bir "Sil" düğmesi vardır. "Kural ekle" ile bir cümle türü seçilir ve boşlukları doldurulur:

| Cümle | Saklandığı yer |
| --- | --- |
| **{kolon}** kolonuna yalnızca **{kim, birden çok}** kart taşıyabilir. | `move_permissions` (to = kolon, from = boş); seçilen her özne için bir satır |
| **{kolon}** kolonuna **{kolon}** kolonundan yalnızca **{kim}** taşıyabilir. | `move_permissions` (from dolu) |
| **{kolon}** kolonuna yalnızca **{kolonlar}** kolonlarından gelinebilir. | `transitions` (+ board `transitions_mode = restricted`) |
| **{kolon}** kolonuna **girerken / çıkarken** kartın **{şart}** olmalı. | `column_conditions` |
| **{kolon}** kolonunda aynı anda en fazla **{N}** kart olabilir. | `columns.wip_limit` |
| Bir kişinin **{işaretli kolonlarda}** en fazla **{N}** kartı olabilir. | `boards.person_wip_limit` + `columns.counts_person_wip` |

Notlar:

- **Geçişler:** kolon başına gruplanıp tek cümle olarak gösterilir. Hiç geçiş cümlesi kalmazsa board `open` moduna döner. Restricted modda hiç geçişi olmayan bir kolon için listede şu uyarı görünür: "Bu kolona hiçbir yerden gelinemiyor".
- **Yetkiler:** aynı hedef ve kaynak için tek cümlede birleştirilir ("yalnızca liderler ve QA"). Cümle silinince o gruptaki bütün satırlar silinir.
- **"Kimler" seçenekleri:** Takımın her üyesi, Takım lideri, Kartın atanan kişisi, board rolleri.
- **Şart seçenekleri:** spec §5.1 kataloğu. "Etiketlerden biri" seçilince etiket seçimi, "En az N dosya eki" seçilince sayı alanı açılır.
- **Sunucu:** `op=rule_add` (cümle türü + alanlar) ve `op=rule_delete` (kural kimliği: tür + id'ler). Bunlar mevcut store fonksiyonlarına çevrilir.
- **JS yoksa** her cümle türü için ayrı küçük bir form vardır.

### 2.6 Silme ve geri alınamaz işlemler

- **Kapsam:** kart arşivleme, kolon, etiket, rol, kural, yorum ve ek silme, üye çıkarma, board arşivleme, kullanıcı devre dışı bırakma.
- **Görünüm:** hepsi tehlikeli düğme stilindedir ve diğer düğmelerden boşlukla ayrılır.
- **Onay:** `confirm.js` bir `<dialog>` açar. Başlık "… silinsin mi?", altında sonucu anlatan bir cümle; "Vazgeç" ve kırmızı "Evet, sil" düğmeleri.
- **JS yoksa** form `?confirm=1` içermeden gönderilirse sunucu bir onay sayfası döner. Bu sayfada aynı form `confirm=1` ile gönderilir.

## 3. Görsel dil

### 3.1 Renk ve tema

Tokenlar CSS değişkeni olarak tanımlanır. Koyu tema `prefers-color-scheme: dark` ile seçilir.

| Token | Açık | Koyu |
| --- | --- | --- |
| zemin | `#fbfaf7` | `#191714` |
| yüzey (kart, panel) | `#fffefb` | `#2b2722` |
| ikincil yüzey (kolon) | `#f2efe8` | `#221f1b` |
| kenarlık | `#ebe6db` | `#39342d` |
| metin / soluk metin | `#2a2620` / `#857c6f` | `#eee9df` / `#a89e8e` |
| vurgu | `#e8590c` | `#ff7a2f` |
| tehlike | `#b42318` (zemin `#fde2e2`) | `#ff8f85` (zemin `#3d1c1a`) |
| başarı | `#2b8a3e` | `#8ce99a` |

- **Etiket paleti:** 8 sıcak renk. Her renk için açık ve koyu temada ayrı zemin ve yazı rengi tanımlanır; `data-color` ile seçilir. Kolon noktaları aynı paletten sırayla atanır.
- **Gölgeler:** kart için `0 1px 0` kağıt gölgesi, panel ve diyalog için yumuşak ve geniş gölge.

### 3.2 Bileşenler

- **Düğme:**
  - Türler: birincil (vurgu, dolu), ikincil (kenarlıklı), sessiz (metin), tehlikeli (kırmızı); küçük ve normal boy.
  - Pasif düğmede `aria-disabled` ve açıklayan bir `title` bulunur.
- **Girdi:** text, textarea, select, date ve checkbox aynı yükseklikte ve çerçevede. Odakta vurgu renginde halka. Hata durumunda kırmızı kenarlık ve altında mesaj.
- **Kart, panel, tablo, sekme, çip, avatar, rozet:** baş harfli avatarın rengi kullanıcı id'sinden türetilir.
- **Diyalog, bildirim baloncuğu, boş durum.**
- **İkonlar:** satır içi SVG, `currentColor`, 16 px. Set: kalem, çöp kutusu, zil, tutamaç, kapat, artı, takvim, kullanıcı, kilit, ok. Emoji kullanılmaz.
- **Odak:** her etkileşimli öğede `:focus-visible` halkası.

### 3.3 Tipografi ve aralık

Sistem font yığını kullanılır.

- **Boyutlar:** gövde 15 px / 1,55; küçük metin 13 px; başlıklar 24 / 18 / 15 px.
- **Aralık ölçeği:** 4 · 8 · 12 · 16 · 24 · 32 px.
- **Köşeler:** 8 px (girdi), 10–12 px (kart, panel), 999 px (çip).

## 4. Ekranlar

### 4.1 İskelet

- **Sol çubuk** (240 px, daraltılabilir, durum `localStorage`'da):
  - Logo; Board'larım, Bana atananlar, Bildirimler (rozet push ile).
  - Takımlar ve altlarında board'ları. Etkin board vurgulu.
  - **Alt kısım:** Takımlar, Kullanıcılar (yalnız admin), profil (Ayarlarım), dil seçimi, Çıkış.
- **Dar ekran:** 900 px altında çubuk, menü düğmesiyle açılan bir çekmeceye dönüşür.
- **Flash mesajları** bildirim baloncuğu olarak sağ altta görünür.

### 4.2 Board

- **Başlık satırı:** ad, takım adı, Etkinlik ve Ayarlar (yönetici).
- **Kolon:** ikincil yüzeyde bir panel. Başlıkta renkli nokta, ad, kart sayısı ve WIP göstergesi `2 / 3`; dolunca turuncu, aşılınca kırmızı.
- **"Kart ekle":** kart açılabilen kolonların altında bulunur ve satır içinde bir başlık alanı açar. Hangi kolonların kart açabildiğini kural motoru belirler; bu davranış değişmez.
- **Kart:** başlık; etiket çipleri; alt satırda avatar, son tarih (gecikmişse kırmızı), kontrol listesi `1/3`, ek sayısı, bloklu rozeti.
- **Boş kolonda** soluk bir "Kart yok" yazısı ve bırakma alanı görünür.
- **Taşıma reddedilince** kolonların üstünde ihlalleri listeleyen bir uyarı kutusu çıkar ve birkaç saniye sonra kapanır.

### 4.3 Kart paneli

- **Açılış ve URL:** board'da karta tıklanınca sağdan açılır. URL kartın adresi olur (`history.pushState`). Esc ve kapat düğmesi paneli kapatır.
- **Başlık bölümü:** düzenlenebilir başlık, `Board › Kolon` yolu, kapat düğmesi.
- **Özellik ızgarası** (2 sütun): kolon, atanan kişi, son tarih, öncelik, tahmin, etiketler.
- **Sekmeler:**
  - **Detay:** açıklama (görüntüleme `richText` ile, tıklayınca düzenleme), kontrol listesi, bağımlılıklar, ekler.
  - **Yorumlar (n):** liste ve en altta yazma alanı. `@` önerileri `<datalist>` ile gelir.
  - **Etkinlik.**
- **Alt kısım:** "Arşivle" (tehlikeli, onaylı).
- **Tam sayfa:** aynı içerik `/boards/{id}/cards/{card}` adresinde tam sayfa olarak açılır; sol çubuk görünür.

### 4.4 Ayarlar

`/boards/{id}/settings?tab=…` adresinde Genel, Kolonlar, Etiketler, Roller ve Kurallar sekmeleri bulunur. Sekme seçimi URL'de durur, böylece JS olmadan da çalışır.

### 4.5 Diğer ekranlar

Takımlar, takım sayfası (üye kartları, rol seçimi, board'lar), Kullanıcılar, Bildirimler, Ayarlarım ve Bana atananlar aynı bileşenlerle yeniden yazılır. Her listenin bir boş durumu vardır. Giriş hatası ve 404/500 sayfaları sol çubuksuz, ortalanmış bir kart olarak gösterilir.

## 5. Teknik yaklaşım

- **CSS:**
  - Dosyalar `static/css/` altında: `tokens.css`, `base.css`, `components.css`, `layout.css` ve sayfa dosyaları (`board.css`, `card.css`, `settings.css`). Base layout bunları `{{asset}}` ile yükler.
  - Build adımı yok; inline style yok.
  - Avatar rengi, etiket ve kolon rengi `data-*` öznitelikleriyle belirlenir.
- **JS:** `/static/js/` altında ES modülleri: `app.js` (çubuk, bildirim baloncukları), `board.js` (mevcut), `drawer.js`, `autosave.js`, `confirm.js`, `columns-editor.js`, `rules.js`.
  - Hepsi `data-*` işaretlerine bağlanır ve collage-live'ın `collage:swap` olayında yeniden bağlanır.
  - JS olmadan her şey normal formlarla çalışır.
- **Sunucu tarafında yeni action işlemleri:** `set_field`, `columns_save`, `role_rename`, `rule_add` ve `rule_delete`, `confirm` akışı.
- **Store'a eklenenler:** `UpdateCardField`, `SaveColumns`, `RenameBoardRole`, ve kuralları cümlelere gruplayan `BoardRuleSentences`.
- **Kaldırılan sayfa parçaları:** `templates/` altındaki sayfalar yeni bileşenlerle yeniden yazılır. Eski satır bazlı kolon formları ve eski kural listeleri kaldırılır.
- **i18n:** yeni metinler iki kataloğa birlikte eklenir (`Strict`).

## 6. Test

- **Go testleri:**
  - Kendi rolünü düşürememe: admin ve lider, sunucu tarafında.
  - `set_field`: yalnız o alan değişir, sürüm çakışmasında alan geri alınır (script ile istek 422 ve mesaj, scriptsiz istek flash ile karta yönlendirme), atamada kişi WIP ihlali.
  - `columns_save`:
    - Sıra, ad ve bayraklar değişir; yeni ve silinen kolonlar uygulanır.
    - Kartı olan kolon silinmez.
    - Hata durumunda hiçbir şey kaydedilmez.
  - Rolü yeniden adlandırma.
  - Kural cümleleri:
    - Her cümle türü doğru satırlara çevrilir; gruplanmış cümlenin silinmesi bütün satırlarını siler.
    - Son geçiş cümlesi silinince board `open` moduna döner.
  - JS'siz silmede onay sayfası.
- **Görsel doğrulama:** uygulama yerel Authentik ile çalıştırılır. Headless Chrome ile board, kart paneli, ayarlar sekmeleri, takım sayfası ve bildirimler açık ve koyu temada ekran görüntüsü olarak alınıp incelenir.
- **Erişilebilirlik:**
  - Klavye ile gezinme: sekmeler, diyalog, panel; Esc paneli kapatır.
  - Kontrast: metin en az 4,5:1.
  - Her ikonlu düğmenin `aria-label`'ı vardır.

## 7. Kapsam dışı

Kullanıcıya özel tema seçici (yalnız sistem teması izlenir), klavye kısayolları, sürükleyerek kolon sıralama board ekranında (yalnız ayarlarda), mobil uygulama.
