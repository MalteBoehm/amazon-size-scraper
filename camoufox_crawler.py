#!/usr/bin/env python3

import json
import sys
import urllib.parse
from camoufox.sync_api import Camoufox

def collect_products(search_url, max_pages=2):
    # Decode URL if needed
    if '%' in search_url:
        search_url = urllib.parse.unquote(search_url)
    
    print(f"Starting crawler with URL: {search_url}")
    
    all_products = []
    
    with Camoufox(headless=False, humanize=True) as browser:
        page = browser.new_page()
        current_url = search_url
        
        for page_num in range(1, max_pages + 1):
            print(f"\n--- Page {page_num} ---")
            print(f"URL: {current_url}")
            
            # Navigate
            page.goto(current_url, wait_until='networkidle')
            page.wait_for_timeout(3000)
            
            # Check title
            title = page.title()
            print(f"Title: {title}")
            
            if "Tut uns Leid" in title:
                print("ERROR: Got error page!")
                break
            
            # Extract products
            products = page.query_selector_all('[data-component-type="s-search-result"]')
            print(f"Found {len(products)} products")
            
            for product in products:
                try:
                    asin = product.get_attribute('data-asin')
                    if not asin:
                        continue
                    
                    # Title
                    title_elem = product.query_selector('h2 a span')
                    title = title_elem.text_content() if title_elem else ''
                    
                    # Price
                    price_elem = product.query_selector('.a-price-whole')
                    price = price_elem.text_content() if price_elem else ''
                    
                    # Check if title mentions size table
                    has_size_info = any(word in title.lower() for word in ['größentabelle', 'größe', 'länge', 'breite'])
                    
                    product_data = {
                        'asin': asin,
                        'title': title.strip(),
                        'price': price.strip(),
                        'url': f'https://www.amazon.de/dp/{asin}',
                        'has_size_info': has_size_info
                    }
                    
                    all_products.append(product_data)
                    
                    if has_size_info:
                        print(f"✓ {asin}: {title[:50]}... (MIGHT HAVE SIZE TABLE)")
                    else:
                        print(f"  {asin}: {title[:50]}...")
                    
                except Exception as e:
                    print(f"Error extracting product: {e}")
            
            # Find next page
            next_button = page.query_selector('a:has-text("Weiter")')
            if next_button and page_num < max_pages:
                href = next_button.get_attribute('href')
                if href:
                    if href.startswith('/'):
                        current_url = 'https://www.amazon.de' + href
                    else:
                        current_url = href
                    
                    print(f"\nFound next page: {current_url[:80]}...")
                    page.wait_for_timeout(3000)  # Rate limit
                else:
                    print("No next page found")
                    break
            else:
                print("No more pages")
                break
    
    # Save results
    with open('camoufox-products.json', 'w', encoding='utf-8') as f:
        json.dump(all_products, f, ensure_ascii=False, indent=2)
    
    print(f"\nTotal products found: {len(all_products)}")
    print(f"Products with size info: {sum(1 for p in all_products if p['has_size_info'])}")
    print("Results saved to camoufox-products.json")

if __name__ == '__main__':
    if len(sys.argv) < 2:
        # Default search
        url = "https://www.amazon.de/s?k=t-shirt+größentabelle&i=fashion"
    else:
        url = sys.argv[1]
    
    max_pages = int(sys.argv[2]) if len(sys.argv) > 2 else 2
    
    collect_products(url, max_pages)