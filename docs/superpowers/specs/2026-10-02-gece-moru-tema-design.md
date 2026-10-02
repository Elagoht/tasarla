# "Gece moru" teması — tasarım

Tarih: 2026-10-02 · Durum: onaylandı (sohbette, görsel yardımcıyla), yazılı hali gözden geçirilecek

Ekip, uygulamanın bugünkü görünümünü ("sıcak kağıt": bej zemin, turuncu vurgu) beğenmedi ve onu genel bir "Claude Code uygulaması" gibi buldu. Bu tasarım, iskeleti olduğu gibi bırakıp yalnız görsel dili değiştirir.

## 1. Kullanıcının seçimleri

Görsel yardımcıda yapıldı.

- **Yön:** A3, "Gece moru". Koyu lacivert-mor kenar çubuğu, soğuk lavanta-gri zemin, indigo vurgu, Manrope, daha kalın başlıklar.
- **Koyu tema:** D2, "menü yükseltilmiş". Kenar çubuğu zeminden daha açık ve kenarında çizgi var, zemin daha derin, mor ton her yerde.
- **Kurum kimliği:** yok, seçim serbest.

## 2. Kapsam

**Değişen:** `static/css/` ve `static/fonts/`. Şablonlara yalnız font yükleme gerekiyorsa dokunulur; bugün fontlar `tokens.css` içinde `@font-face` ile tanımlı, büyük olasılıkla şablon değişmez.

**Değişmeyen:**
- yerleşim (kenar çubuğu, board, kart paneli, sayfa sütunu),
- ikonlar, metinler, davranış, JS,
- etiket renk değerleri (veritabanında `#rrggbb` olarak kayıtlı),
- CSP (font yine yerel dosyadan yüklenir).

## 3. Token'lar

Değerler `static/css/tokens.css` içindedir. Var olan token adları korunur, çünkü başka stil dosyaları onları kullanıyor. "Kağıt" adlandırması (`--paper`, `--shelf`) bu yüzden olduğu gibi kalır; yalnız dosya başındaki açıklama yeni temayı anlatacak şekilde güncellenir.

### 3.1 Açık tema

| Token | Değer | Not |
| --- | --- | --- |
| `--paper` | `#f6f6fb` | sayfa zemini |
| `--sheet` | `#ffffff` | kart, panel, diyalog |
| `--shelf` | `#eeeef7` | kolon zemini |
| `--shelf-strong` | `#e4e4f1` | üzerine gelme |
| `--line` | `#e1e1ef` | |
| `--line-strong` | `#cfcfe4` | |
| `--ink` | `#16152b` | |
| `--ink-soft` | `#5d5c78` | |
| `--ink-muted` | `#7d7c98` | |
| `--ink-faint` | `#a9a8c2` | |
| `--accent` | `#5b4cff` | |
| `--accent-strong` | `#3d2fd6` | |
| `--accent-soft` | `#ecebff` | |
| `--accent-line` | `#c9c4ff` | |
| `--on-accent` | `#ffffff` | |
| `--danger` / `-strong` / `-soft` / `-line` | `#c0262d` / `#9b1c22` / `#fde8e8` / `#f6c4c6` | |
| `--ok` / `-soft` | `#1f8a4c` / `#dcf3e6` | |
| `--warn` / `-soft` | `#a65d00` / `#fff0d9` | |
| `--focus` | `rgb(91 76 255 / 0.4)` | |

Gölgeler serin ve hafif olur:
- `--shadow-paper`: `0 1px 0 #e6e6f2, 0 1px 2px rgb(22 21 43 / 0.04)`
- `--shadow-lift`: `0 2px 4px rgb(22 21 43 / 0.06), 0 10px 24px rgb(22 21 43 / 0.08)`
- `--shadow-float`: `0 12px 40px rgb(22 21 43 / 0.18), 0 2px 6px rgb(22 21 43 / 0.08)`

### 3.2 Koyu tema (D2)

| Token | Değer |
| --- | --- |
| `--paper` | `#0b0a16` |
| `--sheet` | `#1b1a36` |
| `--shelf` | `#13122a` |
| `--shelf-strong` | `#1f1d3d` |
| `--line` | `#2a2848` |
| `--line-strong` | `#3a375f` |
| `--ink` / `-soft` / `-muted` / `-faint` | `#ecebff` / `#c3c2e3` / `#a3a2c4` / `#6e6c93` |
| `--accent` / `-strong` / `-soft` / `-line` | `#7d72ff` / `#a49cff` / `#27245a` / `#433e8f` |
| `--on-accent` | `#ffffff` |
| `--danger` / `-strong` / `-soft` / `-line` | `#ff8a8f` / `#ffb0b3` / `#3a1c27` / `#6b2b3a` |
| `--ok` / `-soft` | `#7fe0a8` / `#153a2a` |
| `--warn` / `-soft` | `#ffc078` / `#3b2a14` |
| `--focus` | `rgb(125 114 255 / 0.55)` |

Gölgeler bugünkü koyu değerlerdir: siyah ve yoğun.

### 3.3 Kenar çubuğu

Kenar çubuğu açık temada da koyu olduğu için kendi token'larını tanımlar. Kenar çubuğunun içindeki her öğe (menü, takım başlıkları, arama kutusu, hesap menüsü ve açılır listesi, "Board yok" metni) bu token'lardan beslenir.

| | Açık tema | Koyu tema |
| --- | --- | --- |
| `.sidebar` zemini | `#1b1a3a` | `#1f1d3d`, sağ kenarda `1px solid #34315e` |
| `--ink` | `#ffffff` | `#ffffff` |
| `--ink-soft` | `#c3c2e3` | `#c3c2e3` |
| `--ink-muted` | `#9a99c2` | `#9a99c2` |
| `--ink-faint` | `#6f6e98` | `#6f6e98` |
| `--line` | `#2c2a5c` | `#34315e` |
| `--sheet` (açılır liste, arama kutusu) | `#25234a` | `#2a2850` |
| `--shelf-strong` (üzerine gelme) | `#2c2a5c` | `#322f6a` |
| `--accent-strong` | `#c4beff` | `#c4beff` |

- Seçili menü öğesi `#2c2a5c` (koyu temada `#322f6a`) zeminli olur, yazısı beyaz ve kalındır. Bugünkü "beyaz kart" görünümü (`--sheet` + gölge) kaldırılır.
- `.nav__item.is-active .nav__tab` halkası kenar çubuğu zeminine göre ayarlanır.
- Mobilde kenar çubuğu üstte açıldığında aynı renkleri kullanır.
- Hesap menüsünün açılır listesi kenar çubuğunun içinde olduğu için koyu yüzeyi kullanır.
- Tehlikeli eylem (Çıkış) üzerine gelince `--danger-soft` yerine kenar çubuğuna uygun koyu bir kırmızı alır: zemin `#4a1d2e`, yazı `#ffb0b3`.

### 3.4 Yazı tipi ve şekil

- `--font-body` ve `--font-display` ikisi de `"Manrope", ui-sans-serif, system-ui, -apple-system, "Segoe UI", sans-serif`.
- Manrope değişken font olarak (`wght 200–800`) iki alt kümeyle `static/fonts/`'a konur: `manrope-latin-wght-normal.woff2` ve `manrope-latin-ext-wght-normal.woff2`. Türkçe harfler latin-ext'tedir. Lisansı `OFL-manrope.txt` olarak eklenir.
- Atkinson Hyperlegible ve Bricolage Grotesque dosyaları ile `@font-face` tanımları kaldırılır. Lisans dosyaları da kaldırılır.
- Başlıklar (`h1` ve `.board-head h1` dahil, `--font-display` kullanan her yer) `font-weight: 800; letter-spacing: -0.02em` olur.
- `--r-sm: 0.5rem`, `--r-md: 0.625rem`, `--r-lg: 0.75rem`. `--r-lg` 0.875'ten düşer.

### 3.5 Etiketler ve avatarlar

- Etiket paletinin açık tema değerleri aynı kalır.
- Etiket paletinin koyu temadaki `--chip-bg` değerleri yeni yüzeylere göre ayarlanır: aynı renk tonu, `#1b1a36` üzerinde okunur bir koyu zemin. `#495057` (gri) için `--chip-bg: #2a2848; --chip-ink: #c3c2e3` olur.
- Avatar renklerinden `data-hue="0"` turuncu (`#e8590c`) yerine indigo `#5b4cff` olur. `data-hue="7"`, bugün kahverengi-gri olan `#5c554a`, `#5d5c78` olur.

### 3.6 Elle yazılmış renkler

Token dışında kalan 7 renk (`board.css` 1, `components.css` 4, `gantt.css` 2) tek tek gözden geçirilir. Yeni temaya uymayanlar bir token'a bağlanır ya da yeni temanın değerine çevrilir.

## 4. Doğrulama

- **Ekranlar:** Chrome'da açık ve koyu temada şu sayfalar görülür (koyu tema `prefers-color-scheme` öykünmesiyle):
  - board (şeritsiz ve şeritli),
  - kart paneli, Gantt, arama,
  - board ayarları (Genel, Kolonlar, Kurallar, Şablonlar),
  - Ayarlarım, bildirimler, takım sayfası, giriş hatası sayfası.
- **Telefon genişliği:** 390 piksel genişlikte board ve kart paneli.
- **Kontrast:** gövde metni ile zemin ve kenar çubuğu yazısı ile kenar çubuğu zemini WCAG AA'yı (4.5:1) sağlamalı. Değerler tarayıcının hesapladığı renklerle kontrol edilir.
- **Testler:** `go test ./...`. Hiçbir test görünüme bağlı değildir; bir test yanlışlıkla bir CSS sınıfına bağlıysa ortaya çıkar.
