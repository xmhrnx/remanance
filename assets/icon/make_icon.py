"""Draws the Remanence app icon: a dark storage platter seen from above,
its grooves faint, and one amber trace still glowing on a groove - the
data that remains after it was deleted.

    python assets/icon/make_icon.py

writes assets/icon.png (1024x1024) and assets/icon/mark.png (the platter
alone on transparency, for the app's own header).
"""
import math
import os

from PIL import Image, ImageChops, ImageDraw, ImageFilter

S = 4096  # drawn at 4x, then downsampled for clean edges
OUT = 1024
HERE = os.path.dirname(os.path.abspath(__file__))


def lerp(a, b, t):
    return tuple(int(a[i] + (b[i] - a[i]) * t) for i in range(len(a)))


def squircle_mask(size, inset, n=5.0):
    """A superellipse, the shape of modern app icons."""
    m = Image.new("L", (size, size), 0)
    d = ImageDraw.Draw(m)
    r = (size - 2 * inset) / 2
    c = size / 2
    pts = []
    for i in range(720):
        t = 2 * math.pi * i / 720
        ct, st = math.cos(t), math.sin(t)
        x = c + r * math.copysign(abs(ct) ** (2 / n), ct)
        y = c + r * math.copysign(abs(st) ** (2 / n), st)
        pts.append((x, y))
    d.polygon(pts, fill=255)
    return m


def radial(size, center, radius, inner, outer):
    img = Image.new("RGBA", (size, size))
    px = img.load()
    cx, cy = center
    step = 8
    small = Image.new("RGBA", (size // step, size // step))
    sp = small.load()
    for y in range(size // step):
        for x in range(size // step):
            dx, dy = x * step - cx, y * step - cy
            t = min(1.0, math.hypot(dx, dy) / radius)
            sp[x, y] = lerp(inner, outer, t * t)
    return small.resize((size, size), Image.BICUBIC)


def arc(draw, c, r, start, end, width, fill):
    box = (c - r, c - r, c + r, c + r)
    draw.arc(box, start, end, fill=fill, width=width)


def conic(size, center, stops):
    """An angular gradient: brushed-metal sheen around the spindle."""
    step = 8
    n = size // step
    small = Image.new("RGBA", (n, n))
    sp = small.load()
    cx, cy = center[0] / step, center[1] / step
    for y in range(n):
        for x in range(n):
            t = (math.atan2(y - cy, x - cx) / (2 * math.pi)) % 1.0
            for k in range(len(stops) - 1):
                t0, c0 = stops[k]
                t1, c1 = stops[k + 1]
                if t0 <= t <= t1:
                    sp[x, y] = lerp(c0, c1, (t - t0) / (t1 - t0))
                    break
    return small.resize((size, size), Image.BICUBIC)


def glow_of(layer, color, radius, strength):
    """A light halo from a layer's shape, in a solid color (blurring the
    layer itself would blur in its transparent black too)."""
    a = layer.split()[3].filter(ImageFilter.GaussianBlur(radius))
    a = a.point(lambda v: min(255, int(v * strength)))
    g = Image.new("RGBA", layer.size, color)
    g.putalpha(a)
    return g


def trace_arc(size, c, r, start, end, width, head, tail):
    """An arc that fades in from its tail to a bright head."""
    layer = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    d = ImageDraw.Draw(layer)
    steps = 90
    for k in range(steps):
        t = k / (steps - 1)
        a0 = start + (end - start) * k / steps
        a1 = start + (end - start) * (k + 1) / steps + 0.5
        col = lerp(tail, head, t ** 1.8)
        arc(d, c, r, a0, a1, width, col)
    return layer


def platter(size, glow=True):
    """The platter and its trace, on transparency."""
    img = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    c = size / 2
    R = size * 0.335

    disc = conic(size, (c, c), [
        (0.00, (44, 48, 58, 255)), (0.12, (78, 84, 98, 255)), (0.25, (36, 39, 48, 255)),
        (0.45, (24, 26, 33, 255)), (0.62, (66, 71, 84, 255)), (0.78, (30, 33, 41, 255)),
        (1.00, (44, 48, 58, 255)),
    ])
    shade = radial(size, (size * 0.38, size * 0.32), size * 0.55, (255, 255, 255, 30), (0, 0, 0, 90))
    disc = Image.alpha_composite(disc, shade)
    mask = Image.new("L", (size, size), 0)
    ImageDraw.Draw(mask).ellipse((c - R, c - R, c + R, c + R), fill=255)
    img.paste(disc, (0, 0), mask)

    d = ImageDraw.Draw(img)
    for i, rr in enumerate(range(int(R * 0.34), int(R * 0.97), int(R * 0.06))):
        arc(d, c, rr, 0, 360, max(2, size // 900), (210, 220, 240, 26 if i % 2 else 14))
    arc(d, c, R - size * 0.002, 0, 360, size // 400, (255, 255, 255, 30))
    arc(d, c, R - size * 0.002, 195, 300, size // 300, (255, 255, 255, 70))

    hub = R * 0.20
    d.ellipse((c - hub, c - hub, c + hub, c + hub), fill=(12, 13, 17, 255))
    arc(d, c, hub, 0, 360, size // 500, (255, 255, 255, 40))
    h2 = hub * 0.42
    d.ellipse((c - h2, c - h2, c + h2, c + h2), fill=(44, 47, 58, 255))

    amber = (255, 190, 104)
    w = int(size * 0.022)
    main = trace_arc(size, c, R * 0.72, 200, 322, w, (255, 214, 140, 255), (232, 120, 58, 0))
    # Two fainter fragments on other grooves: what's left of other files.
    frag1 = trace_arc(size, c, R * 0.52, 40, 92, int(w * 0.7), (255, 190, 110, 150), (232, 120, 58, 0))
    frag2 = trace_arc(size, c, R * 0.88, 118, 150, int(w * 0.6), (255, 190, 110, 110), (232, 120, 58, 0))
    ang = math.radians(322)
    hx, hy = c + R * 0.72 * math.cos(ang), c + R * 0.72 * math.sin(ang)
    hd = ImageDraw.Draw(main)
    pr = w * 0.9
    hd.ellipse((hx - pr, hy - pr, hx + pr, hy + pr), fill=(255, 236, 196, 255))
    pr2 = w * 0.45
    hd.ellipse((hx - pr2, hy - pr2, hx + pr2, hy + pr2), fill=(255, 255, 245, 255))

    for layer, strength in ((frag2, 0.8), (frag1, 0.9), (main, 1.0)):
        if glow:
            img = Image.alpha_composite(img, glow_of(layer, amber + (255,), size * 0.028, 1.1 * strength))
            img = Image.alpha_composite(img, glow_of(layer, (255, 214, 150, 255), size * 0.007, 1.4 * strength))
        img = Image.alpha_composite(img, layer)
    return img


def icon():
    inset = int(S * 0.055)
    shape = squircle_mask(S, inset)
    bg = radial(S, (S * 0.5, S * 0.12), S * 1.0, (46, 34, 28, 255), (8, 9, 12, 255))
    base = Image.new("RGBA", (S, S), (0, 0, 0, 0))
    base.paste(bg, (0, 0), shape)

    # Fine inner edge light.
    edge = ImageChops.subtract(shape, squircle_mask(S, inset + S // 500))
    base.paste(Image.new("RGBA", (S, S), (255, 220, 170, 34)), (0, 0), edge)

    p = platter(S)
    # Platter shadow.
    sh = Image.new("RGBA", (S, S), (0, 0, 0, 0))
    a = p.split()[3].point(lambda v: v * 0.55)
    sh.putalpha(a)
    sh = sh.transform(sh.size, Image.AFFINE, (1, 0, 0, 0, 1, -S * 0.018)).filter(ImageFilter.GaussianBlur(S * 0.02))
    base = Image.alpha_composite(base, sh)
    base = Image.alpha_composite(base, p)
    clipped = Image.new("RGBA", (S, S), (0, 0, 0, 0))
    clipped.paste(base, (0, 0), shape)
    return clipped.resize((OUT, OUT), Image.LANCZOS)


if __name__ == "__main__":
    icon().save(os.path.join(HERE, "..", "icon.png"))
    platter(S).resize((256, 256), Image.LANCZOS).save(os.path.join(HERE, "mark.png"))
    print("wrote assets/icon.png and assets/icon/mark.png")
