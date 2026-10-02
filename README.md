# Kanban

Birkaç takımın günlük iş takibi için kullandığı, kuralları board başına yapılandırılabilen bir kanban uygulaması. [collage](https://collage.furkanbaytekin.dev/) ile sunucu tarafında render edilir. İstemcide yalnızca sürükle-bırak ve kart diyaloğu için küçük bir JS katmanı vardır. Tasarım: [`kanban-spec.md`](kanban-spec.md). Aşama planları: [`docs/superpowers/plans/`](docs/superpowers/plans/).

## Neler var

- **Giriş:** OIDC (authorization code + PKCE). Kullanıcı `(issuer, sub)` ile tanınır. `ADMIN_EMAILS` ilk admini belirler; adminlik `/admin/users` üzerinden yönetilir.
- **Takımlar ve board'lar:** takım lead'leri board açar. Board'a erişim takım üyeliğiyle verilir, takım dışındakiler 404 görür.
- **Kartlar:** başlık, açıklama, atanan kişi, tahmin, son tarih, öncelik, etiketler, kontrol listesi, bağımlılıklar (döngü engellenir), arşiv.
  - **Şablonlar:** board ayarlarında kart şablonları; şablondan kart açma; günlük, haftalık ya da aylık zamanlamayla kartı kendiliğinden açma. Kurallar reddederse şablon sahibine bildirim gider.
  - **Markdown:** kart açıklaması ve yorumlar Markdown olarak gösterilir (GFM: tablo, görev listesi, üstü çizili; `==metin==` ile işaretleme); ham HTML etiket olarak çalışmaz, yazılan metin olduğu gibi görünür; görsel yerine alt metni gösterilir.
- **Taşıma:**
  - SortableJS ile sürükle-bırak; değişiklik açık board'lara collage-live ile anında itilir.
  - Bayat bir kartı taşımak 409 döner.
  - JS olmadan kart sayfasındaki "kolona taşı" formu kullanılabilir.
  - **Şeritler:** board atanan kişiye ya da önceliğe göre şeritlere bölünebilir; kartı başka bir şeride sürüklemek kartı o kişiye atar ya da önceliğini değiştirir, kurallardan geçerek.
- **Kural motoru** (`internal/rules`, saf Go):
  - Kurallar: geçişler, taşıma yetkileri, giriş ve çıkış koşulları, kolon WIP'i, kişi WIP'i.
  - Kural çiğneyen bir istek, ihlallerin hepsi kullanıcının dilinde listelenerek 422 ile reddedilir.
- **İşbirliği:** yorumlar ve `@` ile anma, kart ve board düzeyinde activity akışı, dosya ekleri (en fazla 5 MB; SVG her zaman indirilir).
- **Bildirimler:**
  - Uygulama içi bildirimler ve canlı rozet.
  - Outbox üzerinden e-posta, alıcının dilinde.
  - Son tarih hatırlatmaları.
  - `/me/settings` üzerinden dil ve e-posta tercihleri.
  - **Takvim:** `/me/settings` üzerinden kişiye özel bir adresle bana atanan kartlar ya da bir board'un kartları takvim uygulamasına (iCal) eklenir; adres sıfırlanabilir.
- **Dil:** TR + EN. Dil yalnızca URL'den belirlenir: `/…` varsayılan dil, `/en/…` İngilizce. Saat dilimi `TIMEZONE` ile verilir.
- **Filtre ve arama:** board'da metin, atanan kişi, etiket, öncelik ve son tarihe göre filtre; uymayan kartlar soluklaşır ve filtre canlı güncellemelerde korunur. `/search`, takımlardaki bütün kartlarda arar.

Takvim adresleri (`/cal/…`) oturum gerektirmez; kimliği adresteki token belirler. Ters vekil ya da erişim kuralları `/cal/` yolunu dışarıya açık bırakmalıdır.

## Geliştirme

```bash
docker compose up -d                 # PostgreSQL 17, localhost:55432
cp .env.example .env.development      # OIDC istemcisini ve anahtarları doldur
collage dev                           # http://localhost:6060

export KANBAN_TEST_DATABASE_URL='postgres://kanban:kanban@localhost:55432/kanban?sslmode=disable'
go test ./... -race                   # veritabanı testleri bu değişken yoksa atlanır
```

- **PostgreSQL eklentisi:** migration 008, `pg_trgm`'i kurar (`CREATE EXTENSION`). `pg_trgm` güvenilir (trusted) bir eklentidir: veritabanının sahibi olan kullanıcı superuser olmadan kurabilir. Uygulamanın kullanıcısı veritabanının sahibi değilse eklentiyi bir kez elle kurun.
- **Gerçek giriş yerelde denenemeyebilir.** Callback adresi IdP'de kayıtlı redirect URI'dir, `OIDC_REDIRECT_URL` ile verilir ve `BASE_URL` ile aynı origin'de olmalıdır. Kayıtlı adres `localhost` değilse gerçek giriş yerelde yapılamaz; testler sahte bir issuer kullanır (`internal/auth/authtest`).
- **Çıkış:** sonrasında `<BASE_URL>/login` adresine dönülür.
- **Migration'lar** binary'ye gömülüdür ve açılışta çalışır.

### Yerel Authentik

`dev/authentik/` test provider'la aynı sürümde (2026.8.3) bir Authentik çalıştırır. Kanban uygulamasını, OAuth2 provider'ını ve iki kullanıcıyı (`alice`, `bob`; şifre `kanban`) blueprint ile kurar.

```bash
cd dev/authentik
printf 'PG_PASS=%s\nAUTHENTIK_SECRET_KEY=%s\n' "$(openssl rand -hex 24)" "$(openssl rand -hex 50)" > .env
docker compose up -d                  # ilk açılış ~1 dakika; http://localhost:9000 (akadmin / kanban-admin)
```

`.env.development` için gereken değerler:

```
BASE_URL=http://localhost:6060
OIDC_ISSUER=http://localhost:9000/application/o/kanban-dev/
OIDC_REDIRECT_URL=http://localhost:6060/auth/openid/authentik
OIDC_CLIENT_ID=kanban-dev
OIDC_CLIENT_SECRET=kanban-dev-secret
ADMIN_EMAILS=alice@example.com
```

## Ortam değişkenleri

| Değişken | Zorunlu | Açıklama |
| --- | --- | --- |
| `BASE_URL` | evet | Sitenin kendi origin'i, ör. `https://kanboard-test.unoghub.org` |
| `DATABASE_URL` | evet | PostgreSQL bağlantısı |
| `DEFAULT_LOCALE` | hayır | `tr` (varsayılan) ya da `en` |
| `OIDC_ISSUER` | evet | IdP'nin issuer URL'si, sondaki `/` dahil |
| `OIDC_REDIRECT_URL` | evet | IdP'de kayıtlı redirect URI; `BASE_URL` üzerinde olmalı |
| `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET` | evet | IdP istemcisi |
| `OIDC_SCOPES` | hayır | Varsayılan `openid email profile` |
| `ADMIN_EMAILS` | hayır | İlk girişte admin olacak e-postalar, virgülle ayrılmış |
| `SESSION_KEY`, `CSRF_KEY`, `FLASH_KEY` | evet | Her biri en az 32 bayt, hex (`openssl rand -hex 32`) |
| `ATTACHMENTS_DIR` | evet | Dosya eklerinin dizini |
| `TIMEZONE` | hayır | "Bugün" ve zamanlamaların saat dilimi, IANA adı (ör. `Europe/Istanbul`). Verilmezse sunucunun yerel saati |
| `SMTP_HOST`, `SMTP_FROM` | evet | E-posta sunucusu ve gönderen |
| `SMTP_PORT` | hayır | Varsayılan `587` |
| `SMTP_USER`, `SMTP_PASSWORD` | hayır | Yalnız TLS üzerinden ya da localhost'ta kullanılır |

Uygulama, eksik ya da hatalı her değeri adıyla raporlar ve başlamaz. IdP'ye ulaşılamazsa da başlamaz.

## Yapı

```
main.go                    süreç: config → db → migrate → OIDC → web → worker + scheduler
internal/config            ortam değişkenleri
internal/db                bağlantı, gömülü migration'lar, test veritabanı (dbtest)
internal/store             PostgreSQL erişimi; taşıma, kural ve activity transaction'ları
internal/rules             kural motoru (saf)
internal/authz             takım ve board yetkileri (saf)
internal/auth              OIDC istemcisi, giriş/çıkış, oturumdaki kullanıcı
internal/web               sayfalar, fragment'lar, action'lar, dosya indirme
internal/notify            bildirimler, outbox worker'ı, son tarih zamanlayıcısı
internal/mail              SMTP gönderimi
internal/files             dosya eklerinin diski
templates/, static/, locales/
```

## Framework sorunları

collage ya da eklentilerinde karşılaşılan sorunlar [`framework-issues/`](framework-issues/) altında. 001 ve 002 çözüldü. 003 (fragment adresinin `ErrNotFound`'da 500 dönmesi) açık; etkisi o dosyada anlatılıyor.
