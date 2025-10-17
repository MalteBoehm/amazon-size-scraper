#!/usr/bin/env python3

import json
import sys
import urllib.parse
import time
from datetime import datetime
from camoufox.sync_api import Camoufox

def decode_url(url):
    """Decode URL properly - this is the key!"""
    # First decode
    if '%' in url:
        decoded = urllib.parse.unquote(url)
        # Fix multiple + signs that get decoded incorrectly
        # Replace +++ with single +
        decoded = decoded.replace('+++', ' + ')
        decoded = decoded.replace('++', ' + ')
        return decoded
    return url

def extract_products_from_page(page):
    """Extract all products from current page"""
    products = []
    
    # Wait for products to load
    page.wait_for_selector('[data-component-type="s-search-result"]', timeout=10000)
    
    # Get all product containers
    product_elements = page.query_selector_all('[data-component-type="s-search-result"]')
    
    for product in product_elements:
        try:
            # Extract ASIN
            asin = product.get_attribute('data-asin')
            if not asin:
                continue
            
            # Extract title
            title_elem = product.query_selector('h2 a span')
            title = title_elem.text_content().strip() if title_elem else ''
            
            # Extract price
            price_elem = product.query_selector('.a-price-whole')
            price = price_elem.text_content().strip() if price_elem else ''
            
            # Extract URL
            link_elem = product.query_selector('h2 a')
            href = link_elem.get_attribute('href') if link_elem else ''
            product_url = f"https://www.amazon.de{href}" if href.startswith('/') else href
            
            # Check if title mentions size-related keywords
            size_keywords = ['größentabelle', 'größe', 'länge', 'breite', 'size', 'chart', 'maße']
            has_size_info = any(keyword in title.lower() for keyword in size_keywords)
            
            product_data = {
                'asin': asin,
                'title': title,
                'price': price,
                'url': product_url,
                'has_size_info': has_size_info,
                'scraped_at': datetime.now().isoformat()
            }
            
            products.append(product_data)
            
            # Print with marker for size info
            marker = "✓" if has_size_info else " "
            print(f"{marker} {asin}: {title[:60]}...")
            
        except Exception as e:
            print(f"Error extracting product: {e}")
            continue
    
    return products

def find_next_page_url(page):
    """Find the URL for the next page"""
    try:
        # Look for "Weiter" (Next) button
        next_buttons = [
            'a.s-pagination-next:not(.s-pagination-disabled)',
            'a:has-text("Weiter")',
            '.s-pagination-strip a.s-pagination-next'
        ]
        
        for selector in next_buttons:
            next_elem = page.query_selector(selector)
            if next_elem:
                href = next_elem.get_attribute('href')
                if href:
                    if href.startswith('/'):
                        return f"https://www.amazon.de{href}"
                    return href
        
        return None
    except:
        return None

def crawl_amazon_search(start_url, max_pages=5):
    """Main crawler function"""
    # IMPORTANT: Decode the URL first!
    decoded_url = decode_url(start_url)
    
    print(f"Original URL: {start_url[:80]}...")
    print(f"Decoded URL: {decoded_url[:80]}...")
    print(f"Max pages: {max_pages}")
    print("="*80)
    
    all_products = []
    
    with Camoufox(headless=False, humanize=True) as browser:
        page = browser.new_page()
        current_url = decoded_url
        
        for page_num in range(1, max_pages + 1):
            print(f"\n📄 Page {page_num}")
            print(f"URL: {current_url[:100]}...")
            
            # Navigate to page
            try:
                page.goto(current_url, wait_until='domcontentloaded', timeout=30000)
                time.sleep(3)  # Let page settle
                
                # Check title
                title = page.title()
                print(f"Page title: {title}")
                
                # Check for error page
                if "Tut uns Leid" in title or "Sorry" in title:
                    print("❌ ERROR: Got error page! Stopping.")
                    break
                
                # Check for captcha
                if page.query_selector('#captchacharacters'):
                    print("⚠️  CAPTCHA detected! Manual intervention needed.")
                    input("Solve the captcha and press Enter to continue...")
                
                # Extract products
                print("\nExtracting products...")
                products = extract_products_from_page(page)
                
                if not products:
                    print("No products found on this page.")
                    break
                
                print(f"\nFound {len(products)} products on page {page_num}")
                print(f"Products with size info: {sum(1 for p in products if p['has_size_info'])}")
                
                all_products.extend(products)
                
                # Save after each page
                save_products(all_products)
                
                # Check for next page
                if page_num < max_pages:
                    next_url = find_next_page_url(page)
                    if next_url:
                        print(f"\nNext page URL found: {next_url[:80]}...")
                        current_url = next_url
                        time.sleep(5)  # Rate limit between pages
                    else:
                        print("\nNo next page found.")
                        break
                
            except Exception as e:
                print(f"❌ Error on page {page_num}: {e}")
                break
    
    # Final summary
    print("\n" + "="*80)
    print(f"✅ Crawling completed!")
    print(f"Total products collected: {len(all_products)}")
    print(f"Products with size info: {sum(1 for p in all_products if p['has_size_info'])}")
    print(f"Results saved to: amazon-products.json")

def save_products(products):
    """Save products to JSON file"""
    with open('amazon-products.json', 'w', encoding='utf-8') as f:
        json.dump(products, f, ensure_ascii=False, indent=2)

if __name__ == '__main__':
    # Your exact URL
    default_url = 'https://www.amazon.de/s?k=t+shirt+%2B+%22gr%C3%B6%C3%9Fentabelle%22+%2B+%22l%C3%A4nge%22&i=fashion&__mk_de_DE=%C3%85M%C3%85%C5%BD%C3%95%C3%91&crid=3SY6UWBYTVR1G&sprefix=t+shirt+%2B+%22gr%C3%B6%C3%9Fentabelle%22+%2B+%22l%C3%A4nge%22+%2Cfashion%2C64&ref=nb_sb_noss'
    
    url = sys.argv[1] if len(sys.argv) > 1 else default_url
    max_pages = int(sys.argv[2]) if len(sys.argv) > 2 else 3
    
    crawl_amazon_search(url, max_pages)