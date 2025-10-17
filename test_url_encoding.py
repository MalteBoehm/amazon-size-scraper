#!/usr/bin/env python3

import urllib.parse
from camoufox.sync_api import Camoufox

def test_urls():
    # Der Original-URL von dir
    original_url = 'https://www.amazon.de/s?k=t+shirt+%2B+%22gr%C3%B6%C3%9Fentabelle%22+%2B+%22l%C3%A4nge%22&i=fashion&__mk_de_DE=%C3%85M%C3%85%C5%BD%C3%95%C3%91&crid=3SY6UWBYTVR1G&sprefix=t+shirt+%2B+%22gr%C3%B6%C3%9Fentabelle%22+%2B+%22l%C3%A4nge%22+%2Cfashion%2C64&ref=nb_sb_noss'
    
    # URL dekodieren um zu sehen was drin ist
    decoded = urllib.parse.unquote(original_url)
    print("Original URL:", original_url[:100] + "...")
    print("\nDecoded URL:", decoded[:100] + "...")
    
    # Parse die URL components
    parsed = urllib.parse.urlparse(original_url)
    params = urllib.parse.parse_qs(parsed.query)
    
    print("\nURL Parameters:")
    for key, value in params.items():
        print(f"  {key}: {value}")
    
    # Verschiedene Encoding-Varianten testen
    test_urls = [
        # 1. Original URL (wie du sie mir gegeben hast)
        ("Original", original_url),
        
        # 2. Vollständig dekodiert
        ("Fully decoded", decoded),
        
        # 3. Neu konstruiert mit korrekter Kodierung
        ("Reconstructed", construct_search_url()),
        
        # 4. Einfache Version ohne spezielle Zeichen
        ("Simple", "https://www.amazon.de/s?k=t-shirt&i=fashion"),
    ]
    
    print("\n" + "="*80 + "\n")
    
    with Camoufox(headless=False, humanize=True) as browser:
        for name, url in test_urls:
            print(f"\nTesting {name}...")
            print(f"URL: {url[:80]}...")
            
            page = browser.new_page()
            
            try:
                page.goto(url, wait_until='networkidle', timeout=30000)
                page.wait_for_timeout(2000)
                
                title = page.title()
                print(f"Title: {title}")
                
                if "Tut uns Leid" in title:
                    print("❌ ERROR PAGE!")
                else:
                    products = page.query_selector_all('[data-component-type="s-search-result"]')
                    print(f"✓ Found {len(products)} products")
                    
                # Screenshot
                page.screenshot(path=f'url-test-{name.lower().replace(" ", "-")}.png')
                
            except Exception as e:
                print(f"Error: {e}")
            
            page.close()

def construct_search_url():
    """Konstruiere die URL mit korrekter Kodierung"""
    base = "https://www.amazon.de/s"
    params = {
        'k': 't shirt + "größentabelle" + "länge"',
        'i': 'fashion',
        '__mk_de_DE': 'ÅMÅŽÕÑ',
        'crid': '3SY6UWBYTVR1G',
        'sprefix': 't shirt + "größentabelle" + "länge" ,fashion,64',
        'ref': 'nb_sb_noss'
    }
    
    # URL mit urllib konstruieren
    query_string = urllib.parse.urlencode(params, quote_via=urllib.parse.quote)
    return f"{base}?{query_string}"

if __name__ == '__main__':
    test_urls()