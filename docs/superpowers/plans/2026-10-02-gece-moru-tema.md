# "Gece moru" teması: Uygulama Planı

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** "Sıcak kağıt" görünümünün yerine "Gece moru" teması gelir: soğuk lavanta-gri zemin, indigo vurgu, koyu lacivert-mor kenar çubuğu, Manrope; koyu temada "yükseltilmiş" kenar çubuğu. Yerleşim, metinler ve davranış değişmez.

**Architecture:** Bütün renk ve ölçüler `static/css/tokens.css`'teki token'lardan gelir. Plan token değerlerini değiştirir ve kenar çubuğunun kendi içinde token'ları yeniden tanımlamasını sağlar. Ardından token dışında kalan birkaç elle yazılmış rengi ve başlık ağırlığını düzeltir. Font, yerel `woff2` dosyalarından yüklenir.

**Tech Stack:** Düz CSS (custom properties), `@font-face`, collage'ın `/static` sunumu.

**Spec:** `docs/superpowers/specs/2026-10-02-gece-moru-tema-design.md`. Bütün hex değerleri spec §3'ten **olduğu gibi** alınır.

## Global Constraints

- Yalnız `static/css/` ve `static/fonts/` değişir. Şablonlar, Go kodu ve JS değişmez. Tek istisna: bir dosya eski font adını ya da dosyasını anıyorsa o anma güncellenir.
- Var olan token adları korunur. Yeni token yalnız kenar çubuğu için eklenir (`--side-*`).
- CSP değişmez: dış kaynaktan font, stil ya da satır içi `style` yok.
- Koyu tema bugünkü gibi `@media (prefers-color-scheme: dark)` ile gelir.
- Commit mesajları semantik ve İngilizcedir. Biçim: konu satırı, boş satır, `Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>`.
- Testler: `KANBAN_TEST_DATABASE_URL='postgres://kanban:kanban@localhost:55432/kanban?sslmode=disable' go test ./... -race -count=1`.

## Review Focus

1. **Kenar çubuğundaki okunabilirlik:** açık temada da koyu zemin üstünde her metin (menü, takım başlığı, "Board yok", e-posta, arama kutusunun yer tutucusu) okunmalı. Kontrast gövde metninde en az 4.5:1, yer tutucu ve ikincil metinde en az 3:1 olmalı. *(Task 3, Task 5)*
2. **Hesap menüsünün açılır listesi** kenar çubuğunun içinde olduğu için koyu yüzeyi kullanmalı. Çıkış öğesinin kırmızısı orada da okunmalı. *(Task 3)*
3. **Türkçe harfler:** başlıklarda ve gövdede ı, İ, ş, ğ, ç, ö ve ü Manrope'la görünmeli, sistem fontuna düşmemeli. latin-ext alt kümesi bunu sağlar. *(Task 1, Task 5)*
4. **Etiketler ve avatarlar:** koyu temada her etiket rengi yeni yüzeyde okunmalı; avatarların beyaz harfi her tonda görünmeli. *(Task 2, Task 5)*
5. **Mobil:** 390 px genişlikte açılan kenar çubuğu, kart paneli ve board yatay olarak taşmamalı. *(Task 5)*

## Dosya haritası

| Dosya | Değişiklik |
| --- | --- |
| `static/fonts/` | Manrope eklenir; Atkinson ve Bricolage kaldırılır |
| `static/css/tokens.css` | `@font-face`, açık ve koyu token'lar, kenar çubuğu token'ları, etiketler, avatarlar |
| `static/css/base.css` | Başlık ağırlığı ve harf aralığı |
| `static/css/layout.css` | Kenar çubuğu: zemin, seçili öğe, hesap menüsü |
| `static/css/board.css`, `components.css`, `gantt.css` | Elle yazılmış renkler, başlık ağırlıkları |

---

### Task 1: Manrope

**Files:** `static/fonts/`, `static/css/tokens.css` (`@font-face` blokları ve `--font-*`)

- [ ] **Step 1: Font dosyalarını indir.** Google Fonts CSS2 API'sinin verdiği adresler (Manrope v20, `wght 200–800`):

```bash
cd static/fonts
curl -fsSL -o manrope-latin-wght-normal.woff2 https://fonts.gstatic.com/s/manrope/v20/xn7gYHE41ni1AdIRggexSvfedN4.woff2
curl -fsSL -o manrope-latin-ext-wght-normal.woff2 https://fonts.gstatic.com/s/manrope/v20/xn7gYHE41ni1AdIRggmxSvfedN62Zw.woff2
curl -fsSL -o OFL-manrope.txt https://raw.githubusercontent.com/sharanda/manrope/master/OFL.txt
file manrope-*.woff2   # "Web Open Font Format (Version 2)" olmalı
```

  Lisans adresi açılmazsa `https://raw.githubusercontent.com/google/fonts/main/ofl/manrope/OFL.txt` kullanılır. Bu adres de olmazsa SIL OFL 1.1 metni yazılır; ilk satırı `Copyright 2018 The Manrope Project Authors (https://github.com/sharanda/manrope)` olur.

- [ ] **Step 2: `tokens.css`'teki bütün `@font-face` bloklarını** (Atkinson ve Bricolage) şunlarla değiştir:

```css
@font-face {
  font-family: "Manrope";
  font-style: normal;
  font-weight: 200 800;
  font-display: swap;
  src: url("/static/fonts/manrope-latin-wght-normal.woff2") format("woff2-variations");
  unicode-range: U+0000-00FF, U+0131, U+0152-0153, U+02BB-02BC, U+02C6, U+02DA, U+02DC, U+0304, U+0308, U+0329, U+2000-206F, U+20AC, U+2122, U+2191, U+2193, U+2212, U+2215, U+FEFF, U+FFFD;
}
@font-face {
  font-family: "Manrope";
  font-style: normal;
  font-weight: 200 800;
  font-display: swap;
  src: url("/static/fonts/manrope-latin-ext-wght-normal.woff2") format("woff2-variations");
  unicode-range: U+0100-02BA, U+02BD-02C5, U+02C7-02CC, U+02CE-02D7, U+02DD-02FF, U+0304, U+0308, U+0329, U+1D00-1DBF, U+1E00-1E9F, U+1EF2-1EFF, U+2020, U+20A0-20AB, U+20AD-20C0, U+2113, U+2C60-2C7F, U+A720-A7FF;
}
```

  `--font-body` ve `--font-display`'in ikisini de `"Manrope", ui-sans-serif, system-ui, -apple-system, "Segoe UI", sans-serif` yap.

- [ ] **Step 3: Eski dosyaları sil:** `git rm static/fonts/atkinson-* static/fonts/bricolage-* static/fonts/OFL-atkinson.txt static/fonts/OFL-bricolage.txt`. Ardından `grep -rni "atkinson\|bricolage" . --exclude-dir=.git --exclude-dir=.superpowers --exclude-dir=docs` boş çıkmalı.

- [ ] **Step 4:** Tam test koşusu; var olan testler (CSP ve statik dosya testleri dahil) geçmeli. Commit: `feat(ui): Manrope in place of Atkinson Hyperlegible and Bricolage Grotesque`.

---

### Task 2: Token'lar

**Files:** `static/css/tokens.css`

- [ ] **Step 1: Açık tema.** `:root` içindeki Paper, Ink, Accent and states ve Shape gruplarını spec §3.1 ve §3.4'teki değerlerle değiştir. Gölgeler §3.1'deki üç satırdır. Dosya başındaki açıklama şöyle olur:

  `/* Design tokens: "night purple". Every colour and size in the other sheets comes from here; the dark theme follows the system setting, and the sidebar redefines the ones it needs (layout.css). */`

  Gruplardaki yorumlar korunur ya da yeni temaya göre güncellenir: "the desk: page background" doğru kalır.

- [ ] **Step 2: Koyu tema.** `@media (prefers-color-scheme: dark) { :root { … } }` bloğunu §3.2'deki değerlerle değiştir. Gölgeler bugünkü koyu değerlerdir; dokunma.

- [ ] **Step 3: Kenar çubuğu token'ları.** `:root` içine, açık tema değerleriyle:

```css
  /* The sidebar is dark in both themes (layout.css maps these onto the
     tokens inside it). */
  --side-bg: #1b1a3a;
  --side-edge: transparent;
  --side-ink: #ffffff;
  --side-ink-soft: #c3c2e3;
  --side-ink-muted: #9a99c2;
  --side-ink-faint: #6f6e98;
  --side-line: #2c2a5c;
  --side-raised: #25234a;
  --side-hover: #2c2a5c;
  --side-active: #2c2a5c;
  --side-link: #c4beff;
  --side-danger-bg: #4a1d2e;
  --side-danger-ink: #ffb0b3;
```

  Koyu tema bloğuna, §3.3'e göre:

```css
    --side-bg: #1f1d3d;
    --side-edge: #34315e;
    --side-line: #34315e;
    --side-raised: #2a2850;
    --side-hover: #322f6a;
    --side-active: #322f6a;
```

- [ ] **Step 4: Etiketler** (§3.5). Açık tema blokları aynı kalır. Koyu temadaki `--chip-bg` değerleri şöyle olur:

| Renk | `--chip-bg` | `--chip-ink` |
| --- | --- | --- |
| `#e03131` | `#3d1a2a` | `#ffa8a8` |
| `#f08c00` | `#3b2a17` | `#ffc078` |
| `#2f9e44` | `#15342a` | `#8ce99a` |
| `#1971c2` | `#152a4a` | `#8fc4f5` |
| `#7048e8` | `#2a2458` | `#b9a6ff` |
| `#c2255c` | `#3a1834` | `#faa2c1` |
| `#0c8599` | `#10303d` | `#7fd8e6` |
| `#495057` | `#2a2848` | `#c3c2e3` |

  `--chip-ink` değerleri, son satır dışında, bugünkü değerler olarak kalır.


- [ ] **Step 5: Avatarlar** (§3.5): `[data-hue="0"] { --hue: #5b4cff; }`, `[data-hue="7"] { --hue: #5d5c78; }`.

- [ ] **Step 6:** Tam test koşusu. Commit: `feat(ui): night purple tokens, light and dark`.

---

### Task 3: Kenar çubuğu

**Files:** `static/css/layout.css`

- [ ] **Step 1:** `.sidebar` kuralında `background` ve `border-right`'ı değiştir, sonra token eşlemesini ekle:

```css
.sidebar {
  /* …var olan konum ve boşluk özellikleri… */
  background: var(--side-bg); border-right: 1px solid var(--side-edge);
  /* Inside the sidebar, the page's tokens take the sidebar's values, so its
     links, headings, search box and account menu need no rules of their own. */
  --ink: var(--side-ink);
  --ink-soft: var(--side-ink-soft);
  --ink-muted: var(--side-ink-muted);
  --ink-faint: var(--side-ink-faint);
  --line: var(--side-line);
  --line-strong: var(--side-line);
  --sheet: var(--side-raised);
  --shelf: var(--side-hover);
  --shelf-strong: var(--side-hover);
  --accent-strong: var(--side-link);
  --danger-soft: var(--side-danger-bg);
  --danger: var(--side-danger-ink);
  color: var(--ink);
}
```

- [ ] **Step 2: Seçili menü öğesi:** `.nav__item.is-active { background: var(--side-active); border-color: transparent; color: var(--side-ink); box-shadow: none; }`. `.nav__item.is-active .nav__tab` halkası `var(--side-active)` üstünde görünmeli: `box-shadow: 0 0 0 2px var(--side-active), 0 0 0 3px var(--chip-dot, var(--accent));`.

- [ ] **Step 3: Yer tutucu:** arama kutusu (`.sidebar__search .input`) zemin olarak `--sheet` (yani `--side-raised`) alır. Yer tutucusu `--ink-faint` ile 3:1 kontrastı sağlamayabilir; gerekirse `.sidebar .input::placeholder { color: var(--side-ink-muted); }` ekle. Odak halkası `--focus` ile kalır.

- [ ] **Step 4: Rozet ve diğer öğeler:** bildirim rozeti (`.badge`) ve `.brand__mark` vurgu rengini kullanır; okunurluklarını kontrol et. Mobil menü düğmesi (`.mobilebar`) kenar çubuğunun dışında olduğu için sayfanın token'larını kullanır.

- [ ] **Step 5:** Tam test koşusu. Commit: `feat(ui): a dark sidebar in both themes`.

---

### Task 4: Başlıklar ve elle yazılmış renkler

**Files:** `static/css/base.css`, `board.css`, `components.css`, `gantt.css`

- [ ] `base.css`: `h1, h2, h3, .h1, .h2, .h3` → `font-weight: 800; letter-spacing: -0.02em;`. `h3, .h3`'teki ayrı `font-weight: 650` → `700`.
- [ ] `components.css:84` `.page-head h1` ve `board.css:4` `.board-head h1`: `font-weight: 700` → `800`.
- [ ] `layout.css` `.brand`: `font-weight: 750` → `800`.
- [ ] `board.css:120` `.drawer::backdrop` ve `components.css:178` `.dialog::backdrop`: `rgb(42 38 32 / …)` → `rgb(22 21 43 / …)`, saydamlık değerleri aynı.
- [ ] Kalan beyazlar (`gantt.css` 50 ve 52, `components.css` 125, 130 ve 167) vurgu ya da tehlike zemini üstündeki yazılardır, yeni temada da doğrudur; dokunma.
- [ ] Tam test koşusu. Commit: `feat(ui): heavier headings, cool backdrops`.

---

### Task 5: Görsel doğrulama

Kod değişmez; yalnız bulunan sorunlar düzeltilir, ayrı bir commit'le.

- [ ] `collage dev` ile, gerekirse yeniden başlatarak, Chrome'da spec §4'teki sayfalar açık ve koyu temada görülür. Koyu tema için DevTools'ta "Emulate CSS prefers-color-scheme: dark" kullanılır. Ekran görüntüleri rapora eklenir.
- [ ] Kontrast tarayıcıda hesaplanır. Örnek: `getComputedStyle(el).color` ve arka plan değerleri alınıp WCAG oranı hesaplanır. En az şu öğeler ölçülür:
  - gövde metni / sayfa zemini;
  - `.nav__item` / kenar çubuğu zemini;
  - `.nav__heading` / kenar çubuğu zemini;
  - `.me__mail` / kenar çubuğu zemini;
  - `.card__meta` / kart;
  - yer tutucu / arama kutusu.

  Eşikler: 4.5:1, ikincil metin ve yer tutucu için 3:1.
- [ ] 390 px genişlikte board, açık kenar çubuğu ve kart paneli görülür.
- [ ] Sorun çıkarsa token değerleri düzeltilir, değişen değer ve nedeni rapora yazılır. Commit: `fix(ui): theme contrast and fit` (ancak bir şey değiştiyse).
