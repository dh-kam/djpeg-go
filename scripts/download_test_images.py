import urllib.request
import os

out_dir = "testdata/random100"
os.makedirs(out_dir, exist_ok=True)

print(f"Downloading 100 images to {out_dir}...")
for i in range(1, 101):
    url = "https://picsum.photos/800/600.jpg"
    filename = os.path.join(out_dir, f"img_{i:03d}.jpg")
    try:
        urllib.request.urlretrieve(url, filename)
        if i % 10 == 0:
            print(f"Downloaded {i}/100 images")
    except Exception as e:
        print(f"Failed to download image {i}: {e}")

print("Download complete.")
