#!/usr/bin/env python3

import re
from camoufox.sync_api import Camoufox

def test_product():
    asin = "B08N5WRWNW"  # Echo Dot
    url = f"https://www.amazon.de/dp/{asin}"
    
    print(f"Testing Camoufox with product: {url}")
    
    try:
        with Camoufox(
            headless=False,
            humanize=True,
        ) as browser:
            page = browser.new_page()
            
            print("Navigating to product page...")
            page.goto(url, wait_until='networkidle')
            
            # Human-like behavior
            print("Simulating human behavior...")
            page.wait_for_timeout(2000)
            
            # Random mouse movements
            import random
            for _ in range(3):
                x = random.randint(100, 800)
                y = random.randint(100, 600)
                page.mouse.move(x, y)
                page.wait_for_timeout(random.randint(200, 500))
            
            # Scroll down
            page.evaluate('window.scrollBy(0, 300)')
            page.wait_for_timeout(1000)
            
            # Take screenshot
            page.screenshot(path=f'camoufox-{asin}.png', full_page=True)
            print(f"Screenshot saved to camoufox-{asin}.png")
            
            # Check title
            title = page.title()
            print(f"Page title: {title}")
            
            # Check for captcha
            captcha_selectors = [
                '#captchacharacters',
                'form[action*="Captcha"]',
                'img[src*="captcha"]'
            ]
            
            captcha_found = False
            for selector in captcha_selectors:
                if page.query_selector(selector):
                    print(f"WARNING: Captcha detected with selector: {selector}")
                    captcha_found = True
                    break
            
            if not captcha_found:
                print("No captcha detected!")
                
                # Try to extract product info
                product_title = page.query_selector('#productTitle')
                if product_title:
                    print(f"Product: {product_title.text_content().strip()}")
                
                # Look for dimensions
                content = page.content()
                
                # Search for dimensions pattern
                dimension_patterns = [
                    r'(\d+(?:[,.]\d+)?)\s*x\s*(\d+(?:[,.]\d+)?)\s*x\s*(\d+(?:[,.]\d+)?)\s*(cm|mm|m)',
                    r'Abmessungen.*?:\s*(\d+(?:[,.]\d+)?)\s*x\s*(\d+(?:[,.]\d+)?)\s*x\s*(\d+(?:[,.]\d+)?)\s*(cm|mm|m)',
                    r'Produktabmessungen.*?:\s*(\d+(?:[,.]\d+)?)\s*x\s*(\d+(?:[,.]\d+)?)\s*x\s*(\d+(?:[,.]\d+)?)\s*(cm|mm|m)',
                ]
                
                found_dimensions = False
                for pattern in dimension_patterns:
                    matches = re.findall(pattern, content, re.IGNORECASE)
                    if matches:
                        print(f"Found dimensions: {matches[0]}")
                        found_dimensions = True
                        break
                
                if not found_dimensions:
                    print("No dimensions found in page content")
                    
                    # Try specific selectors
                    detail_selectors = [
                        '#detailBullets_feature_div',
                        '#productDetails_techSpec_section_1',
                        '#feature-bullets',
                        '.detail-bullet-list'
                    ]
                    
                    for selector in detail_selectors:
                        elem = page.query_selector(selector)
                        if elem:
                            text = elem.text_content()
                            print(f"\nFound {selector}:")
                            print(text[:200] + "..." if len(text) > 200 else text)
            
            # Keep browser open for inspection
            print("\nBrowser is open for inspection...")
            print("Check the page manually and press Ctrl+C to exit")
            
            # Wait indefinitely
            while True:
                page.wait_for_timeout(1000)
                
    except KeyboardInterrupt:
        print("\nClosing browser...")
    except Exception as e:
        print(f"Error: {e}")
        import traceback
        traceback.print_exc()

if __name__ == '__main__':
    test_product()