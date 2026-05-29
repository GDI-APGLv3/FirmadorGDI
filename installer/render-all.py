"""
render-all.py
Pipeline completo:
  1. Playwright → screenshot dialog.png (493x312) y banner.png (493x58)
  2. Pillow → convierte a dialog.bmp y banner.bmp
  3. Pillow → genera firmadorgdi.ico (16, 32, 48, 256 px)
"""

import os
import sys
import time
from pathlib import Path
from PIL import Image, ImageDraw, ImageFont

DIR        = Path(r"C:\Users\santi\OneDrive\GDILatam\APP-GDILatam\FirmadorGDI\installer")
LOGO_PATH  = Path(r"C:\Users\santi\OneDrive\GDILatam\Operations\Comunicacion\01-Manual-Marca\GDI\logos\gdi-logo-3.png")

# ── Colores GDI ──────────────────────────────────────────────────────────────
GDI_DARK   = (13, 19, 56)
GDI_INDIGO = (22, 21, 140)
SKY        = (14, 165, 233)
WHITE      = (255, 255, 255)


# ─────────────────────────────────────────────────────────────────────────────
# PASO 1: Screenshots con Playwright
# ─────────────────────────────────────────────────────────────────────────────

def to_file_url(path: Path) -> str:
    return "file:///" + str(path).replace("\\", "/")


def take_screenshot(page, html_path: Path, out_png: Path, width: int, height: int):
    page.goto(to_file_url(html_path), wait_until="networkidle")
    page.set_viewport_size({"width": width, "height": height})
    time.sleep(1.2)  # esperar fuentes Google
    page.screenshot(
        path=str(out_png),
        clip={"x": 0, "y": 0, "width": width, "height": height},
        omit_background=False,
    )
    print(f"  PNG: {out_png}  ({width}x{height})")


def render_screenshots():
    from playwright.sync_api import sync_playwright

    print("\n[1/3] Renderizando screenshots con Playwright ...")
    with sync_playwright() as pw:
        browser = pw.chromium.launch(headless=True)
        page = browser.new_page()

        take_screenshot(
            page,
            DIR / "dialog-src.html",
            DIR / "dialog.png",
            493, 312,
        )
        take_screenshot(
            page,
            DIR / "banner-src.html",
            DIR / "banner.png",
            493, 58,
        )

        browser.close()


# ─────────────────────────────────────────────────────────────────────────────
# PASO 2: PNG → BMP
# ─────────────────────────────────────────────────────────────────────────────

def png_to_bmp(src: Path, dst: Path, expected: tuple):
    img = Image.open(src).convert("RGB")
    w, h = img.size
    if (w, h) != expected:
        print(f"  WARN: {src.name} es {w}x{h}, esperado {expected}. Redimensionando...")
        img = img.resize(expected, Image.LANCZOS)
    img.save(dst, format="BMP")
    size_kb = dst.stat().st_size / 1024
    print(f"  BMP: {dst}  ({expected[0]}x{expected[1]}, {size_kb:.1f} KB)")


def convert_bmps():
    print("\n[2/3] Convirtiendo PNG a BMP ...")
    png_to_bmp(DIR / "dialog.png", DIR / "dialog.bmp", (493, 312))
    png_to_bmp(DIR / "banner.png", DIR / "banner.bmp", (493, 58))


# ─────────────────────────────────────────────────────────────────────────────
# PASO 3: Generar ICO
# ─────────────────────────────────────────────────────────────────────────────

def make_icon_frame(size: int) -> Image.Image:
    """
    Genera una imagen RGBA cuadrada de `size`x`size`.
    Diseño: fondo círculo indigo + logo GDI blanco + punto sky.
    """
    canvas = 256  # trabajar siempre en 256, escalar al final
    img = Image.new("RGBA", (canvas, canvas), (0, 0, 0, 0))
    draw = ImageDraw.Draw(img)

    # Fondo: círculo exterior oscuro + círculo indigo interior
    draw.ellipse([0, 0, canvas - 1, canvas - 1], fill=(*GDI_DARK, 255))
    margin = 10
    draw.ellipse(
        [margin, margin, canvas - 1 - margin, canvas - 1 - margin],
        fill=(*GDI_INDIGO, 255),
    )

    # Logo GDI (RGBA, blanco sobre transparente)
    logo_ok = False
    if LOGO_PATH.exists():
        try:
            logo = Image.open(LOGO_PATH).convert("RGBA")
            logo_size = int(canvas * 0.58)
            logo = logo.resize((logo_size, logo_size), Image.LANCZOS)
            # Recolorizar a blanco puro respetando el canal alpha
            r, g, b, a = logo.split()
            logo_white = Image.merge("RGBA", (
                Image.new("L", logo.size, 255),
                Image.new("L", logo.size, 255),
                Image.new("L", logo.size, 255),
                a,
            ))
            offset = (canvas - logo_size) // 2
            img.paste(logo_white, (offset, offset), logo_white)
            logo_ok = True
        except Exception as e:
            print(f"  WARN logo: {e}")

    if not logo_ok:
        # Fallback geométrico: letra "G"
        try:
            font = ImageFont.truetype(r"C:\Windows\Fonts\arialbd.ttf", int(canvas * 0.52))
        except Exception:
            font = ImageFont.load_default()
        text = "G"
        bbox = draw.textbbox((0, 0), text, font=font)
        tw = bbox[2] - bbox[0]
        th = bbox[3] - bbox[1]
        tx = (canvas - tw) // 2 - bbox[0]
        ty = (canvas - th) // 2 - bbox[1] - int(canvas * 0.02)
        draw.text((tx, ty), text, fill=(*WHITE, 255), font=font)

    # Punto sky — esquina inferior derecha (acento de marca)
    dot_r = int(canvas * 0.13)
    dot_c = int(canvas * 0.75)
    draw.ellipse(
        [dot_c - dot_r, dot_c - dot_r, dot_c + dot_r, dot_c + dot_r],
        fill=(*SKY, 255),
    )
    # Anillo oscuro interior
    inner = int(dot_r * 0.42)
    draw.ellipse(
        [dot_c - inner, dot_c - inner, dot_c + inner, dot_c + inner],
        fill=(*GDI_DARK, 210),
    )

    # Escalar al tamaño pedido
    if size != canvas:
        img = img.resize((size, size), Image.LANCZOS)

    return img


def generate_ico():
    print("\n[3/3] Generando firmadorgdi.ico ...")
    # Generar la imagen base en 256x256
    base = make_icon_frame(256)
    sizes = [(16, 16), (32, 32), (48, 48), (256, 256)]

    ico_path = DIR / "firmadorgdi.ico"
    # Pillow escala automaticamente la imagen base a los tamanios indicados
    base.save(
        ico_path,
        format="ICO",
        sizes=sizes,
    )
    size_kb = ico_path.stat().st_size / 1024
    size_labels = ", ".join(f"{s[0]}px" for s in sizes)
    print(f"  ICO: {ico_path}  ({size_labels}, {size_kb:.1f} KB)")


# ─────────────────────────────────────────────────────────────────────────────
# MAIN
# ─────────────────────────────────────────────────────────────────────────────

def main():
    render_screenshots()
    convert_bmps()
    generate_ico()

    print("\nResumen final:")
    targets = ["dialog.bmp", "banner.bmp", "firmadorgdi.ico"]
    all_ok = True
    for fname in targets:
        fp = DIR / fname
        if fp.exists():
            print(f"  OK   {fp}  ({fp.stat().st_size / 1024:.1f} KB)")
        else:
            print(f"  FALTA  {fp}")
            all_ok = False

    if not all_ok:
        sys.exit(1)
    print("\nTodos los assets listos para WiX.")


if __name__ == "__main__":
    main()
