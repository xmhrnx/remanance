"""Draws the DataDoc app icon: a dark page with its corner folded, and an
amber heartbeat glowing across it - files checked over and brought back.

    python assets/icon/make_icon.py

writes assets/icon.png (1024x1024), assets/icon/mark.png (the page alone
on transparency, for the app's own header) and the Android layers.
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


def glow_of(layer, color, radius, strength):
    """A light halo from a layer's shape, in a solid color (blurring the
    layer itself would blur in its transparent black too)."""
    a = layer.split()[3].filter(ImageFilter.GaussianBlur(radius))
    a = a.point(lambda v: min(255, int(v * strength)))
    g = Image.new("RGBA", layer.size, color)
    g.putalpha(a)
    return g


def tint(size, color, mask):
    """A layer of one color, shaped by mask, to lay over an image (pasting
    a translucent color would punch through the image instead)."""
    layer = Image.new("RGBA", (size, size), color[:3] + (0,))
    layer.putalpha(mask.point(lambda v: v * color[3] // 255))
    return layer


def page_shape(size, box, radius, fold):
    """A sheet with rounded corners and its top right corner folded over."""
    x0, y0, x1, y1 = box
    m = Image.new("L", (size, size), 0)
    d = ImageDraw.Draw(m)
    d.rounded_rectangle(box, radius, fill=255)
    d.polygon([(x1 - fold, y0 - 2), (x1 + 2, y0 - 2), (x1 + 2, y0 + fold)], fill=0)
    return m


def pulse_points(size, box):
    """A heartbeat across the page and a little past its edges: flat, a
    small rise, the sharp beat, a dip, and calm again."""
    x0, y0, x1, y1 = box
    w = x1 - x0
    by = y0 + (y1 - y0) * 0.56
    a = size * 0.075  # how far past the page it runs
    pts = [
        (x0 - a, by), (x0 + w * 0.16, by), (x0 + w * 0.24, by - w * 0.08), (x0 + w * 0.31, by),
        (x0 + w * 0.39, by), (x0 + w * 0.47, by - w * 0.46), (x0 + w * 0.57, by + w * 0.26),
        (x0 + w * 0.64, by), (x0 + w * 0.72, by), (x0 + w * 0.80, by - w * 0.10), (x0 + w * 0.88, by),
        (x1 + a, by),
    ]
    return pts


def doc(size, glow=True):
    """The page and its pulse, on transparency: the mark, centered in a
    canvas the size of the icon."""
    img = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    pw, ph = size * 0.44, size * 0.56
    x0, y0 = (size - pw) / 2, (size - ph) / 2
    box = (x0, y0, x0 + pw, y0 + ph)
    fold = size * 0.12
    radius = size * 0.045
    shape = page_shape(size, box, radius, fold)

    sheet = radial(size, (x0 + pw * 0.2, y0), size * 0.75, (82, 88, 104, 255), (22, 24, 31, 255))
    img.paste(sheet, (0, 0), shape)

    # A light rim on the page's edge, brightest along the top.
    rim = ImageChops.subtract(shape, shape.filter(ImageFilter.MinFilter(max(3, (size // 700) | 1))))
    img = Image.alpha_composite(img, tint(size, (255, 255, 255, 46), rim))

    d = ImageDraw.Draw(img)
    # The fold, turned over, with a shadow beneath it.
    x1 = x0 + pw
    fsh = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    ImageDraw.Draw(fsh).polygon([(x1 - fold, y0), (x1 - fold, y0 + fold), (x1, y0 + fold)], fill=(0, 0, 0, 120))
    fsh = fsh.filter(ImageFilter.GaussianBlur(size * 0.01))
    fclip = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    fclip.paste(fsh, (0, 0), shape)
    img = Image.alpha_composite(img, fclip)
    d = ImageDraw.Draw(img)
    d.polygon([(x1 - fold, y0), (x1 - fold, y0 + fold - radius * 0.4), (x1 - fold + radius * 0.4, y0 + fold), (x1, y0 + fold)],
              fill=(104, 111, 130, 255))

    # Lines of text, faint, above and below the pulse.
    lw = int(size * 0.016)
    text = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    d = ImageDraw.Draw(text)
    for i, (ty, frac) in enumerate(((0.17, 0.42), (0.27, 0.62), (0.80, 0.70), (0.89, 0.48))):
        yy = y0 + ph * ty
        d.rounded_rectangle((x0 + pw * 0.14, yy - lw / 2, x0 + pw * (0.14 + frac), yy + lw / 2), lw / 2,
                            fill=(220, 228, 245, 34 if i % 2 else 48))
    img = Image.alpha_composite(img, text)

    # The pulse, fading in from the left to a bright head on the right.
    w = int(size * 0.026)
    pts = pulse_points(size, box)
    m = Image.new("L", (size, size), 0)
    ImageDraw.Draw(m).line(pts, fill=255, width=w, joint="curve")
    left, right = pts[0][0], pts[-1][0]
    ramp = Image.new("L", (size, 1))
    rp = ramp.load()
    col = Image.new("RGBA", (size, 1))
    cp = col.load()
    tail, head = (232, 120, 58), (255, 214, 140)
    for x in range(size):
        t = min(1.0, max(0.0, (x - left) / (right - left)))
        rp[x, 0] = int(255 * min(1.0, 0.15 + t * 1.6))
        cp[x, 0] = lerp(tail, head, t ** 1.4) + (255,)
    ramp = ramp.resize((size, size))
    line = col.resize((size, size))
    line.putalpha(ImageChops.multiply(m, ramp))
    hx, hy = pts[-1]
    hd = ImageDraw.Draw(line)
    pr = w * 0.95
    hd.ellipse((hx - pr, hy - pr, hx + pr, hy + pr), fill=(255, 236, 196, 255))
    pr2 = w * 0.48
    hd.ellipse((hx - pr2, hy - pr2, hx + pr2, hy + pr2), fill=(255, 255, 245, 255))

    amber = (255, 190, 104)
    if glow:
        img = Image.alpha_composite(img, glow_of(line, amber + (255,), size * 0.03, 1.1))
        img = Image.alpha_composite(img, glow_of(line, (255, 214, 150, 255), size * 0.008, 1.4))
    return Image.alpha_composite(img, line)


def icon():
    inset = int(S * 0.055)
    shape = squircle_mask(S, inset)
    bg = radial(S, (S * 0.5, S * 0.12), S * 1.0, (46, 34, 28, 255), (8, 9, 12, 255))
    base = Image.new("RGBA", (S, S), (0, 0, 0, 0))
    base.paste(bg, (0, 0), shape)

    # Fine inner edge light.
    edge = ImageChops.subtract(shape, squircle_mask(S, inset + S // 500))
    base = Image.alpha_composite(base, tint(S, (255, 220, 170, 34), edge))

    p = doc(S)
    base = Image.alpha_composite(base, shadow(p, S * 0.018, S * 0.02))
    base = Image.alpha_composite(base, p)
    clipped = Image.new("RGBA", (S, S), (0, 0, 0, 0))
    clipped.paste(base, (0, 0), shape)
    return clipped.resize((OUT, OUT), Image.LANCZOS)


def shadow(layer, drop, blur):
    """A soft shadow under a layer's shape."""
    sh = Image.new("RGBA", layer.size, (0, 0, 0, 0))
    sh.putalpha(layer.split()[3].point(lambda v: v * 0.55))
    sh = sh.transform(sh.size, Image.AFFINE, (1, 0, 0, 0, 1, -drop))
    return sh.filter(ImageFilter.GaussianBlur(blur))


def mark():
    """The page alone, cropped close, for the app's header."""
    p = doc(S)
    return square_crop(p).resize((256, 256), Image.LANCZOS)


def square_crop(img):
    x0, y0, x1, y1 = img.getbbox()
    side = max(x1 - x0, y1 - y0)
    cx, cy = (x0 + x1) / 2, (y0 + y1) / 2
    return img.crop((int(cx - side / 2), int(cy - side / 2), int(cx + side / 2), int(cy + side / 2)))


def android_monochrome():
    """Android 13 and later tint this shape for themed icons: the page as
    a solid sheet with the fold and the pulse cut out, the same size as
    the foreground's page."""
    k = 0.70
    pw, ph = S * 0.44 * k, S * 0.56 * k
    x0, y0 = (S - pw) / 2, (S - ph) / 2
    box = (x0, y0, x0 + pw, y0 + ph)
    m = page_shape(S, box, S * 0.045 * k, S * 0.12 * k)
    d = ImageDraw.Draw(m)
    x1 = x0 + pw
    f = S * 0.12 * k
    gap = int(S * 0.012)
    d.line([(x1 - f, y0), (x1 - f, y0 + f), (x1, y0 + f)], fill=0, width=gap)
    pts = [(max(x0 + gap * 2, x), y) for x, y in pulse_points(S, box)]
    pts = [(min(x1 - gap * 2, x), y) for x, y in pts]
    d.line(pts, fill=0, width=int(S * 0.026 * k), joint="curve")
    out = Image.new("RGBA", (S, S), (255, 255, 255, 0))
    out.putalpha(m)
    return out.resize((OUT, OUT), Image.LANCZOS)


def android_layers():
    """Android draws an adaptive icon from two 108dp layers and shows only
    the middle 72dp, cut to the launcher's shape, a circle on many phones:
    the mark has to fit a circle 61% of the layer across to keep clear of
    every mask. The background is full bleed; the mask shapes it."""
    bg = radial(S, (S * 0.5, S * 0.12), S * 1.0, (46, 34, 28, 255), (8, 9, 12, 255))
    p = doc(S)
    # The page and its pulse span about 72% of their canvas at their
    # widest; shrinking by 0.70 keeps the corners inside the safe circle.
    k = 0.70
    small = p.resize((int(S * k), int(S * k)), Image.LANCZOS)
    fg = Image.new("RGBA", (S, S), (0, 0, 0, 0))
    o = (S - small.width) // 2
    fg.alpha_composite(shadow(small, -S * 0.012, S * 0.015), (o, o))
    fg.alpha_composite(small, (o, o))
    return fg.resize((OUT, OUT), Image.LANCZOS), bg.convert("RGB").resize((OUT, OUT), Image.LANCZOS)


if __name__ == "__main__":
    icon().save(os.path.join(HERE, "..", "icon.png"))
    mark().save(os.path.join(HERE, "mark.png"))
    fg, bg = android_layers()
    fg.save(os.path.join(HERE, "android-foreground.png"))
    bg.save(os.path.join(HERE, "android-background.png"))
    android_monochrome().save(os.path.join(HERE, "android-monochrome.png"))
    print("wrote assets/icon.png, assets/icon/mark.png and the Android layers")
