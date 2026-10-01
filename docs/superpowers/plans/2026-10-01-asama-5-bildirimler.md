# Aşama 5 — Bildirimler: Uygulama Planı

**Goal:** Uygulama içi bildirimler ve rozet, e-posta tercihleri, outbox ve gönderim worker'ı, son tarih zamanlayıcısı ve `/me/settings`.

**Spec:** §10, §11 (SMTP), §14 Aşama 5. Aşama 1'in Global Constraints'i geçerlidir. Bu plan gece boyunca kendi başıma çalışırken yazıldı.

## Kararlar

- **Migration `005_notifications.sql`:** `notifications` (`dedupe_key` UNIQUE ve boş olabilir), `notification_prefs`, `email_outbox`.
- **`internal/notify`:**
  - `Notifier.Emit` olayları alır ve kişinin kendi yaptığı işlemi düşürür (§10).
  - Alıcının e-posta tercihine bakar. E-postayı **alıcının** diliyle, collage-i18n `Translator` ve `html/template` kullanarak üretir; HTML ve düz metin birlikte. Kartın bağlantısı `BASE_URL` ile kurulur.
  - Bildirimi ve outbox satırını tek transaction'da yazar, sonra `notifications:<userId>` tag'ini invalidate eder.
- **Kart değişikliği ile bildirim aynı transaction'da değil.** Bildirimler, değişikliğin commit'inden hemen sonra kendi transaction'larında yazılır (bkz. Sapmalar).
- **Olaylar ve tetikleyiciler:**
  - `assigned`: atanan kişi değiştiğinde.
  - `mentioned`: yorumda anılan her üyeye.
  - `commented`: kartın atanan kişisine; yazar kendisiyse ya da zaten anıldıysa gönderilmez.
  - `unblocked`: bir kart bitti kolonuna taşındığında ya da arşivlendiğinde, bloklarının hepsi biten kartların atanan kişisine.
  - `due_soon` ve `overdue`: zamanlayıcı her 15 dakikada tarar. `dedupe_key` `due_soon:<card>:<due_date>` biçimindedir; son tarih değişince yeni bir hatırlatma çıkar.
- **Son tarih anlamı:** `due_date` günün başı (sunucu saati) kabul edilir. `due_soon`, son tarihten 24 saat önce başlar. `overdue`, son tarih günü bitince başlar. Bitti kolonundaki ve arşivdeki kartlar taranmaz.
- **E-posta varsayılanları (§10):**
  - Açık: `assigned`, `mentioned`, `due_soon`, `overdue`.
  - Kapalı: `commented`, `unblocked`.
  - Tercihler yalnız değiştirilince yazılır.
- **Worker:**
  - **Tarama:** 30 saniyede bir `FOR UPDATE SKIP LOCKED` ile en fazla 20 satır alır.
  - **Yeniden deneme:** başarısız satır artan gecikmeyle yeniden denenir: `2^attempts` dakika, en fazla 6 saat.
  - **Vazgeçme:** 8 denemeden sonra satır `failed` olur ve loglanır.
  - **Kapanış:** context iptal edildiğinde döngü biter; o an gönderilen e-posta tamamlanır.
- **SMTP (§11):** `SMTP_HOST` ve `SMTP_FROM` zorunludur, `SMTP_PORT` varsayılan olarak 587'dir, `SMTP_USER` ve `SMTP_PASSWORD` isteğe bağlıdır. Sunucu STARTTLS sunuyorsa kullanılır. Kimlik doğrulama yalnız TLS üzerinden ya da localhost'ta yapılır.
- **Rozet:** `/notifications/badge` fragment'ı app layout'ta durur ve `notifications:<userId>` tag'iyle push alır.
- **`/me/settings`:** dil tercihi (`users.locale`, e-postaların dili) ve tür başına e-posta tercihleri.

## Sapmalar

- §5.3, bildirimlerin taşıma transaction'ı içinde oluşturulmasını öngörüyor. Burada değişikliğin commit'inden hemen sonra ayrı bir transaction'da oluşturuluyorlar. Süreç tam bu ikisinin arasında çökerse bir bildirim kaybolabilir. Kartın kendisi ve activity kaydı yine atomiktir. Bildirim mantığı store transaction'larının dışında, tek bir yerde kalır.

## Kabul (§14 Aşama 5)

- SMTP kapalıyken yapılan işlemler başarılı olur, e-postalar kuyrukta bekler ve SMTP açılınca gönderilir.
- `due_soon` bildirimi bir kez gelir; son tarih değişince yeniden gelir.
- E-posta alıcının dilinde gider.
