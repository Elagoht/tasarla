# Aşama 2 — Board ve kart: Uygulama Planı

**Goal:** Takımların board'ları, kolonları ve kartları; sürükle-bırak taşıma, sürüm çakışması (409), collage-live push ile anlık yenileme, "kolona taşı" yedek yolu ve `/me/tasks`.

**Spec:** `kanban-spec.md` §4, §5.3, §6, §7, §8, §14 Aşama 2. Aşama 1 planının Global Constraints'i aynen geçerlidir.

**Bu plan, gece boyunca kendi başıma çalışırken yazıldı.** Aşama 1'deki tam kodlu plan yerine kararları ve dosya haritasını tutar. Kod, her adımda önce test yazılarak geliştirilir.

## Kararlar

- **Migration `002_boards_cards.sql`:** `boards` (`transitions_mode`, `person_wip_limit` dahil; Aşama 3 kullanır), `columns` (`wip_limit`, `is_done`, `allow_create`, `counts_person_wip` dahil), `cards`, `labels`, `card_labels`, `checklist_items`, `card_dependencies`. Kural tabloları (`transitions`, `move_permissions`, `column_conditions`, `board_roles`, `board_role_members`) Aşama 3'te gelir.
- **Yeni board üç varsayılan kolonla açılır:** Yapılacak / Yapılıyor / Bitti, oluşturanın dilinde. İlk kolon `allow_create`, son kolon `is_done`.
- **Yetki (`authz.Board`):** admin ve takım lead'i yönetir (board ayarları, kolonlar, etiketler). Takım üyeleri görür ve düzenler. Takımda olmayanlar 404 alır.
- **IDOR:** her kart, etiket, checklist maddesi ve bağımlılık `board_id` ile birlikte sorgulanır.
- **Taşıma (`store.MoveCard`):** §5.3'teki transaction. Önce `boards` satırı `FOR UPDATE` ile kilitlenir. Kart okunur; `column_id <> expected_from` ya da `version <> expected_version` ise `ErrConflict` döner. Ardından bir `check` kancası çalışır; Aşama 3 kural motorunu buraya takar. Sonra pozisyonlar yeniden numaralanır ve `version++` yapılır. Aynı kolonda sıra değiştirmek de bu yoldan geçer.
- **Sürüm:** kartta yapılan her değişiklik `version`'ı artırır. Yalnız alan düzenleme ve taşıma `expected_version` gönderir (§4); çakışma 409 döner.
- **Cevaplar:** `Collage-Fetch` başlıklı istekler fragment alır: taşımada kolonlar (200 / 409; 422 Aşama 3'te), kart panelinde panel. Başlıksız istekler (JS yokken) karta 303 ile yönlendirilir ve flash mesajı görür.
- **Tag'ler:** kolonlar fragment'ı `board:<id>` döndürür, kart paneli `card:<id>`. Kartı değiştiren her action ikisini de invalidate eder.
- **Kart detayı:** sayfa `/boards/{id}/cards/{card}`, panel fragment'ı `…/panel`. Formların hepsi aynı URL'ye post eder ve bir `op` alanı taşır. JS varsa panel bir `<dialog>` içinde açılır.
- **İstemci:** `static/board.js` (ES module) ve vendor edilmiş `static/vendor/Sortable.min.js`. Inline script yoktur (CSP).
- **Tahmin `numeric`** değeri Go'da `*float64`, **son tarih** `*time.Time` (yalnız tarih), **öncelik** 1–4 arası `*int16`.
- **Bağımlılıklar** aynı board içinde tutulur. Döngü recursive CTE ile engellenir (§4).
- **`/me/tasks`:** kullanıcının erişebildiği board'larda ona atanmış, arşivlenmemiş kartlar, board'a göre gruplanmış.
- **Ana sayfa:** takımlar ve her takımın board'ları.

## Dosyalar

```
internal/db/migrations/002_boards_cards.sql
internal/store/boards.go, columns.go, cards.go, move.go, labels.go, checklist.go, deps.go (+ _test.go)
internal/authz/board.go (+ test)
internal/web/pages_board.go, pages_card.go, pages_board_settings.go, pages_tasks.go (+ tests)
templates/pages/board.html, fragments/columns.html, pages/card.html, fragments/card_panel.html,
  pages/board_settings.html, pages/tasks.html
static/board.js, static/vendor/Sortable.min.js
locales/{tr,en}.json (yeni anahtarlar)
```

## Kabul (spec §14 Aşama 2)

| Kriter | Test |
| --- | --- |
| Bir tarayıcıdaki taşıma diğerinde görünüyor | Canlı akış testi: `/_live/stream/` açık bir bağlantı taşıma sonrası yeni kolonlar fragment'ını alır |
| Bayat kart taşıma 409 ile reddediliyor, kart güncel yerinde | `TestMovingAStaleCardIsAConflict` |
| Sürükleme sırasında gelen push board'u bozmuyor | İstemci tarafı (`pause`/`resume`); elle doğrulanır |
| Döngü engelleme | `A → B → C` varken `C → A` reddedilir |
