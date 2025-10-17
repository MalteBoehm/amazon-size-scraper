-- Insert 50 test products with various ASINs
INSERT INTO products (asin, url, title) VALUES
('B08N5WRWNW', 'https://www.amazon.de/dp/B08N5WRWNW', 'Echo Dot (4. Generation)'),
('B07ZPC9QD4', 'https://www.amazon.de/dp/B07ZPC9QD4', 'T-Shirt 1'),
('B07ZPKQD4Q', 'https://www.amazon.de/dp/B07ZPKQD4Q', 'T-Shirt 2'),
('B07B428M7F', 'https://www.amazon.de/dp/B07B428M7F', 'T-Shirt 3'),
('B07B7WJ2FL', 'https://www.amazon.de/dp/B07B7WJ2FL', 'T-Shirt 4'),
('B07B8TLCM9', 'https://www.amazon.de/dp/B07B8TLCM9', 'T-Shirt 5'),
('B07B8V2MMH', 'https://www.amazon.de/dp/B07B8V2MMH', 'T-Shirt 6'),
('B07B7W1PG6', 'https://www.amazon.de/dp/B07B7W1PG6', 'T-Shirt 7'),
('B07B8V8ZYP', 'https://www.amazon.de/dp/B07B8V8ZYP', 'T-Shirt 8'),
('B07B7ZHKBP', 'https://www.amazon.de/dp/B07B7ZHKBP', 'T-Shirt 9'),
('B07B7ZJMVW', 'https://www.amazon.de/dp/B07B7ZJMVW', 'T-Shirt 10'),
('B07B8V9KQF', 'https://www.amazon.de/dp/B07B8V9KQF', 'T-Shirt 11'),
('B07B42FR8P', 'https://www.amazon.de/dp/B07B42FR8P', 'T-Shirt 12'),
('B07B8V2C1X', 'https://www.amazon.de/dp/B07B8V2C1X', 'T-Shirt 13'),
('B07B7ZH3VJ', 'https://www.amazon.de/dp/B07B7ZH3VJ', 'T-Shirt 14'),
('B07B7ZJKDG', 'https://www.amazon.de/dp/B07B7ZJKDG', 'T-Shirt 15'),
('B07B8V91MJ', 'https://www.amazon.de/dp/B07B8V91MJ', 'T-Shirt 16'),
('B07B8V6VQK', 'https://www.amazon.de/dp/B07B8V6VQK', 'T-Shirt 17'),
('B07B8V99CB', 'https://www.amazon.de/dp/B07B8V99CB', 'T-Shirt 18'),
('B07B8V73RX', 'https://www.amazon.de/dp/B07B8V73RX', 'T-Shirt 19'),
('B07B8V7DK6', 'https://www.amazon.de/dp/B07B8V7DK6', 'T-Shirt 20')
ON CONFLICT (asin) DO NOTHING;
