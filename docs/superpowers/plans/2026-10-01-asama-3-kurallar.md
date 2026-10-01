# Aşama 3 — Kural motoru ve ayarlar: Uygulama Planı

**Goal:** Board başına yapılandırılan kurallar: geçişler, taşıma yetkileri, giriş ve çıkış koşulları, kolon WIP'i ve kişi WIP'i. Kural ihlal eden bir taşıma, oluşturma ya da atama, ihlallerin tamamı kullanıcının dilinde listelenerek 422 ile reddedilir.

**Spec:** §5 (tamamı), §14 Aşama 3. Aşama 1 planının Global Constraints'i geçerlidir. Bu plan da gece boyunca kendi başıma çalışırken yazıldı; Aşama 2 planıyla aynı biçimde.

## Kararlar

- **`internal/rules` saf bir pakettir.** Veritabanını, HTTP'yi ya da collage'ı import etmez. `Evaluate(actor, move, snapshot)` bir taşımanın çiğnediği kuralların hepsini döndürür; sıra §5.2'deki gibi geçiş, yetki, çıkış, giriş, kolon WIP, kişi WIP. `EvaluateCreate` kolona girişi ve kolon WIP'ini, `EvaluateAssign` atama sonrası kişi WIP'ini değerlendirir.
- **Aynı kolonda sıra değiştirmek** hiçbir kurala tabi değildir.
- **Admin**, yetki kayıtlarında takım lead'i ve takım üyesi sayılır. "Zorla taşıma" yoktur (§12).
- **Kişi WIP'i** yalnız sayılan bir kolona dışarıdan girişte ya da atanan kişi değiştiğinde denetlenir. Sayılan iki kolon arasında taşıma kişinin sayısını değiştirmez.
- **Kurallar geriye dönük değildir:** sayım hep "bu kart hariç" yapılır ve yalnız yeni girişler engellenir.
- **Snapshot** store içinde, taşıma transaction'ında, board satırının kilidi tutulurken okunur (§5.3). Böylece WIP yarışı çözülür.
- **İhlaller** `*store.RuleError` olarak döner, içinde `[]rules.Violation` taşır. Web katmanı bunları `rules.*` anahtarlarıyla çevirir.
- **Migration `003_rules.sql`:** `board_roles`, `board_role_members`, `transitions`, `move_permissions`, `column_conditions`. `min_attachments` koşulunun sayabilmesi için Aşama 4'ün `attachments` tablosu da burada açılır.
- **Ayarlar sayfasına eklenenler:** geçiş modu ve geçiş matrisi, kişi WIP limiti, yetkiler, koşullar, board rolleri ve üyeleri.

## Kabul (§14 Aşama 3)

- §5.4'teki testlerin hepsi geçer: kural türü başına geçen ve kalan vakalar, birden fazla ihlal, aynı kolonda sıralama, oluşturma anında giriş, limitin altına düşürülmüş WIP, gerçek PostgreSQL'e karşı aynı anda iki taşıma (tam olarak biri kabul edilir), döngü engelleme.
- Kural ihlal eden bir taşıma, ihlallerin hepsini kullanıcının dilinde listeleyen bir 422 ile reddedilir ve kart eski yerine döner.
- Kural ayarlarını lead ve admin değiştirebilir, member değiştiremez.
