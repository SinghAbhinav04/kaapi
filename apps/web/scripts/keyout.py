# Keys out a flat magenta (#FF00FF) background into alpha, with despill on the edges.
import sys
import numpy as np
from PIL import Image

src, out, preview = sys.argv[1], sys.argv[2], sys.argv[3]
im = Image.open(src).convert("RGB")
a = np.asarray(im).astype(np.float32)
r, g, b = a[..., 0], a[..., 1], a[..., 2]

m = np.minimum(r, b) - g                      # how magenta a pixel is
alpha = np.clip((150.0 - m) / (150.0 - 55.0), 0.0, 1.0)

edge = alpha < 1.0
b2 = np.where(edge, np.minimum(b, (r + g) / 2.0), b)
r2 = np.where(edge, np.minimum(r, g * 1.15 + 45.0), r)
rgba = np.dstack([r2, g, b2, alpha * 255.0]).clip(0, 255).astype(np.uint8)

res = Image.fromarray(rgba, "RGBA")
w = 2000
res = res.resize((w, round(res.height * w / res.width)), Image.LANCZOS)
res.save(out, "WEBP", quality=80, method=6)

# preview on paper and on a dark ground
pv = Image.new("RGBA", (res.width, res.height * 2), (245, 241, 234, 255))
pv.paste(Image.new("RGBA", res.size, (40, 60, 50, 255)), (0, res.height))
pv.alpha_composite(res, (0, 0)); pv.alpha_composite(res, (0, res.height))
pv.convert("RGB").resize((1000, res.height * 1000 // res.width * 2)).save(preview)
print("ok", res.size, "transparent share:", round(float((alpha == 0).mean()), 3))
