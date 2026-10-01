# Aşama 4 — İşbirliği: Uygulama Planı

**Goal:** Yorumlar ve mention'lar, activity akışı (kart detayında ve board düzeyinde) ve dosya ekleri.

**Spec:** §4 (comments, comment_mentions, attachments, activity), §9, §12.3, §14 Aşama 4. Aşama 1'in Global Constraints'i geçerlidir. Bu plan gece boyunca kendi başıma çalışırken yazıldı.

## Kararlar

- **Migration `004_collaboration.sql`:** `comments`, `comment_mentions`, `activity`. `attachments` tablosu 003'te açılmıştı. `activity.actor_id` boş olabilir: kural motorunun ve sistemin işleri bir kişiye ait değildir.
- **Activity yazımı:** taşıma, oluşturma, alan güncelleme ve arşivleme activity satırlarını kendi transaction'larında yazar (§5.3). `UpdateCard` ve `ArchiveCard` bunun için `actorID` alır. Etiket, checklist, bağımlılık, yorum ve ek işlemleri `LogActivity` ile işlemin ardından yazılır.
  - **Kayıt türleri:** `card_created`, `card_moved` {from, to}, `card_updated` {fields}, `card_archived`, `labels_changed`, `checklist_added`, `checklist_checked`, `dependency_added`, `dependency_removed`, `comment_added`, `attachment_added`, `attachment_deleted`.
  - **Taşıma kaydı:** yalnız kolon değiştiğinde yazılır. Aynı kolonda sıra değiştirmek kayıt üretmez.
- **Mention'lar:** `@ad` ifadesi `[\p{L}\p{N}._-]+` kalıbıyla aranır ve takım üyelerine karşı çözülür. Eşleştirme büyük/küçük harfe duyarsızdır; aday değerler e-postanın `@` öncesi kısmı ve boşlukları atılmış ad. Eşleşmeyen ifade düz metin kalır. Yorum formu bir `<datalist>` ile önerir.
- **Yorumlar:**
  - Yorumu yalnız yazarı düzenleyebilir.
  - Silmek yumuşaktır (`deleted_at`). Yazar ya da board yöneticisi silebilir; silinen yorum "silindi" olarak görünür.
  - Metin düz metindir. Satır sonları korunur, `http(s)` bağlantıları otomatik linklenir (§12.3, açıklama için de). Bu, `richText` adlı bir template fonksiyonuyla yapılır; önce kaçışlama, sonra linkleme.
- **Ekler (§9):**
  - **Yükleme:** kart sayfasının action'ı `WithActionFor` ile kaydedilir ve `WithMaxBodyBytes(5<<20 + 64<<10)` alır. 5 MB'ı aşan dosya formda 422 ve mesajla reddedilir.
  - **Saklama:** içerik türü ilk 512 bayttan `http.DetectContentType` ile belirlenir. Dosya `ATTACHMENTS_DIR/<rastgele hex>` olarak yazılır. `ATTACHMENTS_DIR` artık zorunlu bir ortam değişkenidir.
  - **İndirme:** `app.Handle("/files/", …)` üzerinden yapılır. Kullanıcı middleware'in koyduğu context'ten okunur; ek, kartı ve board'u üzerinden yetkiye bağlanır, yetkisiz istek 404 alır. Dosya `os.OpenRoot` ile açılıp `http.ServeContent` ile sunulur. Yalnız PNG, JPEG, GIF ve WebP `inline` gösterilir, gerisi `attachment` olarak iner (SVG her zaman indirme). Her cevapta `nosniff` bulunur.
  - **Silme:** yükleyen ya da board yöneticisi siler; dosya diskten de kaldırılır.
- **Board activity:** `/boards/{id}/activity` sayfası son 100 kaydı listeler. Kart paneli kartın kendi akışını gösterir.

## Kabul (§14 Aşama 4)

- 5 MB'ı aşan dosya form üzerinde mesajla reddedilir.
- Başka bir takımın ekine doğrudan URL ile erişim 404 döner.
- SVG her zaman indirme olarak sunulur.
- Her taşıma ve alan değişikliği activity akışında kim, ne ve ne zaman bilgisiyle görünür.
