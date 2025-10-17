#!/usr/bin/env python3

import sys
from camoufox.sync_api import Camoufox

def test_simple():
    print("Testing Camoufox...")
    
    try:
        # Simple test without complex options
        with Camoufox(
            headless=False,
            humanize=True,
        ) as browser:
            page = browser.new_page()
            
            print("Navigating to Amazon.de...")
            page.goto('https://www.amazon.de/s?k=t-shirt')
            
            # Wait for page to load
            page.wait_for_timeout(5000)
            
            # Take screenshot
            page.screenshot(path='camoufox-test.png')
            print("Screenshot saved to camoufox-test.png")
            
            # Check title
            title = page.title()
            print(f"Page title: {title}")
            
            # Look for products
            products = page.query_selector_all('[data-asin]')
            print(f"Found {len(products)} elements with data-asin")
            
            # Check for captcha
            captcha = page.query_selector('#captchacharacters')
            if captcha:
                print("WARNING: Captcha detected!")
            else:
                print("No captcha detected")
            
            # Get first few ASINs
            for i, product in enumerate(products[:5]):
                asin = product.get_attribute('data-asin')
                if asin:
                    print(f"ASIN {i+1}: {asin}")
            
            input("Press Enter to close browser...")
            
    except Exception as e:
        print(f"Error: {e}")
        import traceback
        traceback.print_exc()

if __name__ == '__main__':
    test_simple()