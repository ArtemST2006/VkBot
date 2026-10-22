-- +goose Up
-- Demo accounts are deliberately on example.com: they are not real VK partners.
INSERT INTO partners (slug, name, account_url) VALUES
    ('demo-partner-01', 'Демо: Кофейня', 'https://example.com/partners/coffee'),
    ('demo-partner-02', 'Демо: Кинотеатр', 'https://example.com/partners/cinema'),
    ('demo-partner-03', 'Демо: Книжный', 'https://example.com/partners/books'),
    ('demo-partner-04', 'Демо: Пиццерия', 'https://example.com/partners/pizza'),
    ('demo-partner-05', 'Демо: Спортклуб', 'https://example.com/partners/sport'),
    ('demo-partner-06', 'Демо: Цветочная', 'https://example.com/partners/flowers'),
    ('demo-partner-07', 'Демо: Музыкальный магазин', 'https://example.com/partners/music'),
    ('demo-partner-08', 'Демо: Пекарня', 'https://example.com/partners/bakery'),
    ('demo-partner-09', 'Демо: Арт-пространство', 'https://example.com/partners/art'),
    ('demo-partner-10', 'Демо: Маркет', 'https://example.com/partners/market');

-- Day one is the Moscow calendar date when this migration is first applied.
WITH schedule (slug, day_offset, prize_name, winner_count) AS (
    VALUES
        ('demo-partner-01', 0, 'Подарочный набор кофе', 2),
        ('demo-partner-02', 1, 'Два билета в кино', 2),
        ('demo-partner-03', 2, 'Сертификат в книжный', 1),
        ('demo-partner-04', 3, 'Сертификат на пиццу', 3),
        ('demo-partner-05', 4, 'Месяц тренировок', 1),
        ('demo-partner-06', 5, 'Букет цветов', 2),
        ('demo-partner-07', 6, 'Сертификат на музыку', 1),
        ('demo-partner-08', 7, 'Набор выпечки', 3),
        ('demo-partner-09', 8, 'Билет на выставку', 2),
        ('demo-partner-10', 9, 'Подарочная корзина', 1)
), campaign AS (
    SELECT (now() AT TIME ZONE 'Europe/Moscow')::date AS first_day
)
INSERT INTO partner_days (
    partner_id, event_date, promo_valid_until, promo_instructions, prize_name, winner_count
)
SELECT
    p.id,
    c.first_day + s.day_offset,
    ((c.first_day + s.day_offset + 1)::timestamp AT TIME ZONE 'Europe/Moscow') - interval '1 second',
    'Демонстрационный промокод. Ссылка и код не предназначены для реальной покупки.',
    s.prize_name,
    s.winner_count
FROM schedule AS s
JOIN partners AS p ON p.slug = s.slug
CROSS JOIN campaign AS c;

-- Five available codes for each demo day; no users or activity are pre-created.
INSERT INTO promo_codes (partner_day_id, code)
SELECT
    d.id,
    upper(replace(p.slug, '-', '')) || '-' || lpad(n.code_number::text, 2, '0')
FROM partner_days AS d
JOIN partners AS p ON p.id = d.partner_id
CROSS JOIN generate_series(1, 5) AS n(code_number)
WHERE p.slug IN (
    'demo-partner-01', 'demo-partner-02', 'demo-partner-03', 'demo-partner-04', 'demo-partner-05',
    'demo-partner-06', 'demo-partner-07', 'demo-partner-08', 'demo-partner-09', 'demo-partner-10'
);

-- +goose Down
-- Referenced demo data stays protected by foreign keys once users interact with it.
DELETE FROM promo_codes
WHERE assigned_user_id IS NULL
  AND partner_day_id IN (
      SELECT d.id
      FROM partner_days AS d
      JOIN partners AS p ON p.id = d.partner_id
      WHERE p.slug IN (
          'demo-partner-01', 'demo-partner-02', 'demo-partner-03', 'demo-partner-04', 'demo-partner-05',
          'demo-partner-06', 'demo-partner-07', 'demo-partner-08', 'demo-partner-09', 'demo-partner-10'
      )
  );

DELETE FROM partner_days
WHERE partner_id IN (
    SELECT id FROM partners WHERE slug IN (
        'demo-partner-01', 'demo-partner-02', 'demo-partner-03', 'demo-partner-04', 'demo-partner-05',
        'demo-partner-06', 'demo-partner-07', 'demo-partner-08', 'demo-partner-09', 'demo-partner-10'
    )
);

DELETE FROM partners WHERE slug IN (
    'demo-partner-01', 'demo-partner-02', 'demo-partner-03', 'demo-partner-04', 'demo-partner-05',
    'demo-partner-06', 'demo-partner-07', 'demo-partner-08', 'demo-partner-09', 'demo-partner-10'
);
