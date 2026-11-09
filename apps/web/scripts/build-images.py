"""Builds the site's images: assets/source/* -> public/images/**.avif.

Originals stay in assets/source (see CREDITS.md). What ships is AVIF, sized for its
largest use, plus smaller copies (name-480.avif, -960, -1600) that the image loader
(src/shared/lib/image-loader.ts) picks per device. The sizes are made here rather than by
Vercel's optimizer, which passes AVIF sources through untouched (and has a monthly quota),
so phones were downloading 2400 px photos.

    python scripts/build-images.py
"""
from pathlib import Path

from PIL import Image

ROOT = Path(__file__).resolve().parent.parent
SRC = ROOT / "assets" / "source"
OUT = ROOT / "public" / "images"
QUALITY = 55
# Keep in step with VARIANT_WIDTHS in src/shared/lib/image-loader.ts.
VARIANTS = (480, 960, 1600)

# output path -> (source, max width). Room paths match apps/api/seed/data/room_types.json.
JOBS = {
    "site/hero-bg.avif": ("gen/hero-bg.jpg", 2400),
    "site/hero-bg-mobile.avif": ("gen/hero-bg-mobile.jpg", 1200),
    "site/hero-coffee-rows.avif": ("gen/hero-coffee-rows.jpg", 2400),
    "site/hero-fg.avif": ("gen/hero-fg.webp", 2000),  # keeps its alpha channel
    "site/coffee-cherries.avif": ("gen/coffee-cherries.jpg", 1000),
    "site/harvest.avif": ("gen/harvest.jpg", 1000),
    "site/filter-coffee.avif": ("gen/filter-coffee.jpg", 2000),
    "site/estate-lawn-wedding.avif": ("gen/estate-lawn-wedding.jpg", 2400),
    "site/planters-hall.avif": ("gen/planters-hall.jpg", 1600),
    "rooms/plantation-room-1.avif": ("unsplash/plantation-room.webp", 1600),
    "rooms/estate-view-room-1.avif": ("unsplash/estate-view.webp", 1600),
    "rooms/heritage-suite-1.avif": ("unsplash/heritage-verandah.webp", 1600),
    "rooms/coffee-villa-1.avif": ("gen/coffee-villa.jpg", 1600),
    "rooms/coffee-villa-2.avif": ("gen/coffee-villa-living.jpg", 1600),
    "rooms/coffee-villa-3.avif": ("gen/outdoor-shower.jpg", 1600),
    "rooms/pool-villa-1.avif": ("unsplash/pool-villa.webp", 1600),
}


def main() -> None:
    for out, (src, width) in JOBS.items():
        im = Image.open(SRC / src)
        im = im.convert("RGBA" if im.mode in ("RGBA", "LA", "P") else "RGB")
        if im.width > width:
            im = im.resize((width, round(im.height * width / im.width)), Image.LANCZOS)
        dest = OUT / out
        dest.parent.mkdir(parents=True, exist_ok=True)
        im.save(dest, "AVIF", quality=QUALITY)
        print(f"{out:36} {im.width}x{im.height}  {dest.stat().st_size // 1024} KB")
        for w in VARIANTS:
            # Every variant exists for every image (capped at its own width), so the
            # loader never has to know an image's size.
            small = im if im.width <= w else im.resize((w, round(im.height * w / im.width)), Image.LANCZOS)
            vdest = dest.with_name(f"{dest.stem}-{w}.avif")
            small.save(vdest, "AVIF", quality=QUALITY)
            print(f"  -{w:<5} {small.width}x{small.height}  {vdest.stat().st_size // 1024} KB")


if __name__ == "__main__":
    main()
