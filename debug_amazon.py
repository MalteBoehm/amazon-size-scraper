#!/usr/bin/env python3

from camoufox.sync_api import Camoufox
import time

print("🔍 Amazon Debug Test")
print("="*80)

# Different URL variations to test
test_urls = [
    ("Simple search", "https://www.amazon.de/s?k=t-shirt"),
    ("With category", "https://www.amazon.de/s?k=t-shirt&i=fashion"),
    ("Direct category", "https://www.amazon.de/b?node=77028031"),
    ("Homepage", "https://www.amazon.de"),
]

with Camoufox(headless=False, humanize=True) as browser:
    page = browser.new_page()
    
    for name, url in test_urls:
        print(f"\n🧪 Testing: {name}")
        print(f"URL: {url}")
        
        try:
            page.goto(url, wait_until='domcontentloaded', timeout=20000)
            time.sleep(2)
            
            title = page.title()
            print(f"Title: {title}")
            
            # Take screenshot
            screenshot_name = f"debug-{name.replace(' ', '-').lower()}.png"
            page.screenshot(path=screenshot_name)
            print(f"Screenshot: {screenshot_name}")
            
            # Check what's on the page
            if "Tut uns Leid" in title:
                print("❌ ERROR PAGE")
                
                # Check for specific error elements
                error_texts = page.query_selector_all('p')
                for p in error_texts[:3]:
                    text = p.text_content().strip()
                    if text:
                        print(f"  Error text: {text[:100]}...")
                        
            else:
                print("✅ Page loaded successfully")
                
                # Check for products
                products = page.query_selector_all('[data-asin]')
                print(f"  Products found: {len(products)}")
                
                # Check for search results
                results = page.query_selector_all('[data-component-type="s-search-result"]')
                print(f"  Search results: {len(results)}")
            
        except Exception as e:
            print(f"❌ Error: {e}")
    
    print("\n" + "="*80)
    print("💡 Manual check:")
    print("Please check the page manually in the browser window")
    print("Press Ctrl+C when done")
    
    try:
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        print("\nClosing...")