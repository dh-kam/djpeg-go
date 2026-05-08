import os
import random
from PIL import Image

out_dir = "tests/testdata/random100"
os.makedirs(out_dir, exist_ok=True)

print(f"Generating 100 images to {out_dir}...")
for i in range(1, 101):
    # Random size between 100x100 and 800x800
    width = random.randint(100, 800)
    height = random.randint(100, 800)
    
    # Randomly choose mode: RGB or L (grayscale)
    mode = random.choice(['RGB', 'L'])
    
    # Generate random noise image
    if mode == 'RGB':
        img = Image.new('RGB', (width, height))
        pixels = img.load()
        for x in range(width):
            for y in range(height):
                pixels[x, y] = (random.randint(0, 255), random.randint(0, 255), random.randint(0, 255))
    else:
        img = Image.new('L', (width, height))
        pixels = img.load()
        for x in range(width):
            for y in range(height):
                pixels[x, y] = random.randint(0, 255)
    
    # Save as JPEG with random quality between 50 and 100
    quality = random.randint(50, 100)
    # Randomly choose if it should be progressive
    progressive = random.choice([True, False])
    
    filename = os.path.join(out_dir, f"img_{i:03d}.jpg")
    img.save(filename, 'JPEG', quality=quality, progressive=progressive)
    
    if i % 10 == 0:
        print(f"Generated {i}/100 images")

print("Generation complete.")
